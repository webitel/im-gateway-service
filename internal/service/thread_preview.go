package service

import (
	"context"
	"log/slog"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwpreview "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
)

type ThreadPreviewer interface {
	GivePreview(ctx context.Context, req *gtwpreview.GivePreviewRequest) (*gtwpreview.GivePreviewResponse, error)
	RemoveFromPreview(ctx context.Context, req *gtwpreview.RemoveFromPreviewRequest) (*gtwpreview.RemoveFromPreviewResponse, error)
	UpgradePreviewToFullMember(ctx context.Context, req *gtwpreview.UpgradePreviewToFullMemberRequest) (*gtwpreview.UpgradePreviewToFullMemberResponse, error)
}

type ThreadPreviewService struct {
	logger        *slog.Logger
	previewClient *imthread.ThreadPreviewClient
	contactClient *imcontact.Client
}

func NewThreadPreviewService(logger *slog.Logger, previewClient *imthread.ThreadPreviewClient, contactClient *imcontact.Client) *ThreadPreviewService {
	return &ThreadPreviewService{
		logger:        logger,
		previewClient: previewClient,
		contactClient: contactClient,
	}
}

func (s *ThreadPreviewService) GivePreview(ctx context.Context, req *gtwpreview.GivePreviewRequest) (*gtwpreview.GivePreviewResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	identity, err := previewCaller(ctx)
	if err != nil {
		return nil, err
	}
	domainID := int32(identity.GetDomainID())

	target, err := s.resolveContact(ctx, req.GetContact(), domainID)
	if err != nil {
		return nil, err
	}

	resp, err := s.previewClient.GivePreview(ctx, &threadv1.GivePreviewRequest{
		ThreadId:           req.GetThreadId(),
		ContactId:          target.GetId(),
		InitiatorContactId: identity.GetContactID(),
		DomainId:           domainID,
		DurationMs:         req.GetDurationMs(),
	})
	if err != nil {
		return nil, err
	}

	initiator := s.lookupInitiator(ctx, identity.GetContactID(), domainID)

	return &gtwpreview.GivePreviewResponse{
		Preview: convertToThreadPreview(resp.GetPreview(), target, initiator),
	}, nil
}

func (s *ThreadPreviewService) RemoveFromPreview(ctx context.Context, req *gtwpreview.RemoveFromPreviewRequest) (*gtwpreview.RemoveFromPreviewResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	identity, err := previewCaller(ctx)
	if err != nil {
		return nil, err
	}
	domainID := int32(identity.GetDomainID())

	target, err := s.resolveContact(ctx, req.GetContact(), domainID)
	if err != nil {
		return nil, err
	}

	initiatorID := identity.GetContactID()
	_, err = s.previewClient.RemoveFromPreview(ctx, &threadv1.RemoveFromPreviewRequest{
		ThreadId:           req.GetThreadId(),
		ContactId:          target.GetId(),
		InitiatorContactId: &initiatorID,
		DomainId:           domainID,
	})
	if err != nil {
		return nil, err
	}

	return &gtwpreview.RemoveFromPreviewResponse{}, nil
}

func (s *ThreadPreviewService) UpgradePreviewToFullMember(ctx context.Context, req *gtwpreview.UpgradePreviewToFullMemberRequest) (*gtwpreview.UpgradePreviewToFullMemberResponse, error) {
	if req == nil {
		return nil, errors.InvalidArgument("request cannot be nil")
	}
	identity, err := previewCaller(ctx)
	if err != nil {
		return nil, err
	}
	domainID := int32(identity.GetDomainID())

	target, err := s.resolveContact(ctx, req.GetContact(), domainID)
	if err != nil {
		return nil, err
	}

	resp, err := s.previewClient.UpgradePreviewToFullMember(ctx, &threadv1.UpgradePreviewToFullMemberRequest{
		ThreadId:           req.GetThreadId(),
		ContactId:          target.GetId(),
		InitiatorContactId: identity.GetContactID(),
		DomainId:           domainID,
		Role:               threadv1.ThreadRole(req.GetRole()),
		Queue:              queueFromContext(ctx, s.logger),
	})
	if err != nil {
		return nil, err
	}

	var member *gtwpreview.ThreadMember
	if m := resp.GetMember(); m != nil {
		member = convertToMember(m, target)
	}

	return &gtwpreview.UpgradePreviewToFullMemberResponse{
		Member: member,
	}, nil
}

// previewCaller returns the caller identity for a ThreadPreviewService request.
// The preview API is restricted to trusted inter-service (system) calls: the
// initiator is not permission-checked downstream, so end users must not reach it.
func previewCaller(ctx context.Context) (auth.Identifier, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}
	if !auth.IsSystemCall(ctx) {
		return nil, errors.Forbidden("thread preview is available to inter-service callers only")
	}

	return identity, nil
}

// resolveContact maps the gateway PeerIdentity (sub/iss) of the target contact to its contact record.
func (s *ThreadPreviewService) resolveContact(ctx context.Context, peer *gtwpreview.PeerIdentity, domainID int32) (*contactv1.Contact, error) {
	list, err := s.contactClient.SearchContact(ctx, &contactv1.SearchContactRequest{
		Subjects: []string{peer.GetSub()},
		IssId:    []string{peer.GetIss()},
		DomainId: domainID,
	})
	if err != nil {
		return nil, err
	}
	if len(list.GetContacts()) == 0 {
		return nil, errors.NotFound("preview target contact not found")
	}

	return list.GetContacts()[0], nil
}

// lookupInitiator resolves the caller's contact for response enrichment only.
// A failure is logged and yields nil, because the downstream mutation has already succeeded.
func (s *ThreadPreviewService) lookupInitiator(ctx context.Context, contactID string, domainID int32) *contactv1.Contact {
	list, err := s.contactClient.SearchContact(ctx, &contactv1.SearchContactRequest{
		Ids:      []string{contactID},
		DomainId: domainID,
	})
	if err != nil {
		s.logger.Warn("PREVIEW_INITIATOR_LOOKUP_FAILED",
			slog.String("contact_id", contactID),
			slog.Any("error", err),
		)
		return nil
	}
	if len(list.GetContacts()) == 0 {
		return nil
	}

	return list.GetContacts()[0]
}

func convertToThreadPreview(p *threadv1.ThreadPreview, target, initiator *contactv1.Contact) *gtwpreview.ThreadPreview {
	if p == nil {
		return nil
	}
	converted := &gtwpreview.ThreadPreview{
		Id:        p.GetId(),
		ThreadId:  p.GetThreadId(),
		CreatedAt: p.GetCreatedAt(),
		ExpiresAt: p.GetExpiresAt(),
	}
	if target != nil {
		converted.Contact = convertToContact(target)
	}
	if initiator != nil {
		converted.Initiator = convertToContact(initiator)
	}

	return converted
}
