package service

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	gtwthread "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	crmcontacts "github.com/webitel/im-gateway-service/infra/client/crm-contacts"
)

// crmCheckTimeout bounds the CRM access checks of one request, so a CRM that
// does not answer fails the operator's request quickly instead of hanging it.
const crmCheckTimeout = 3 * time.Second

type (
	// ContactCandidateLister reads the candidates im-thread identified for a thread.
	ContactCandidateLister interface {
		ListCandidates(ctx context.Context, req *threadv1.ListContactCandidatesRequest) (*threadv1.ListContactCandidatesResponse, error)
	}

	// CRMContactVisibility tells which CRM contacts the caller may read or change.
	CRMContactVisibility interface {
		VisibleContacts(ctx context.Context, ids []string) (map[string]bool, error)
		WritableContacts(ctx context.Context, ids []string) (map[string]bool, error)
	}

	// ThreadReader confirms the caller may read a thread.
	ThreadReader interface {
		Get(ctx context.Context, req *threadv1.GetThreadRequest) (*threadv1.Thread, error)
	}

	// ContactCandidates serves the thread's CRM contact candidates.
	ContactCandidates interface {
		ListContactCandidates(ctx context.Context, req *gtwthread.ListContactCandidatesRequest) (*gtwthread.ListContactCandidatesResponse, error)
	}
)

type contactCandidates struct {
	logger     *slog.Logger
	threads    ThreadReader
	candidates ContactCandidateLister
	crm        CRMContactVisibility
}

func NewContactCandidates(logger *slog.Logger, threads ThreadReader, candidates ContactCandidateLister, crm CRMContactVisibility) *contactCandidates {
	return &contactCandidates{
		logger:     logger.With(slog.String("component", "contact_candidates")),
		threads:    threads,
		candidates: candidates,
		crm:        crm,
	}
}

func (s *contactCandidates) ListContactCandidates(ctx context.Context, req *gtwthread.ListContactCandidatesRequest) (*gtwthread.ListContactCandidatesResponse, error) {
	const errID = "service.contact_candidates.list"

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	if req.GetThreadId() == "" {
		return nil, errors.InvalidArgument("thread id is required", errors.WithID(errID))
	}

	// Before anything reaches im-thread or the CRM: only Webitel users
	// (operators) and schema calls see candidates. End users (portal, widget)
	// also send x-webitel-access, so the token alone does not tell them apart;
	// their token must never be forwarded to the CRM either.
	if !mayUseCRM(ctx, identity) {
		return nil, errors.Wrap(auth.ForbiddenIssuerErr, errors.WithID(errID))
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	// Membership check: im-thread answers NotFound for a thread the caller cannot read.
	if _, err := s.threads.Get(ctx, &threadv1.GetThreadRequest{
		Id:       req.GetThreadId(),
		DomainId: domainID,
		Fields:   []string{"id"},
		CallerId: readCallerID(ctx, identity),
	}); err != nil {
		return nil, err
	}

	resp, err := s.candidates.ListCandidates(ctx, &threadv1.ListContactCandidatesRequest{
		ThreadId: req.GetThreadId(),
		DomainId: domainID,
	})
	if err != nil {
		return nil, err
	}

	visible, err := s.visible(ctx, resp.GetCandidates(), resp.GetCrmContactId())
	if err != nil {
		return nil, err
	}

	out := &gtwthread.ListContactCandidatesResponse{
		CrmContactId:     resp.GetCrmContactId(),
		CrmContactSource: gtwthread.CrmContactSource(resp.GetCrmContactSource()),
		Candidates:       make([]*gtwthread.ContactCandidate, 0, len(resp.GetCandidates())),
	}

	// The bound contact is shown only when the caller may read it. Its source
	// is hidden with it, so the caller does not learn that some contact is bound.
	if bound := out.GetCrmContactId(); bound != "" && visible != nil && !visible[bound] {
		out.CrmContactId = ""
		out.CrmContactSource = gtwthread.CrmContactSource_CRM_CONTACT_SOURCE_UNSPECIFIED
	}

	for _, c := range resp.GetCandidates() {
		if visible != nil && !visible[c.GetCrmContactId()] {
			continue
		}

		out.Candidates = append(out.Candidates, toGatewayCandidate(c))
	}

	return out, nil
}

// CheckCRMContactAccess decides what binding crmContactID to a thread may do
// for the caller: reading the contact is required (PermissionDenied otherwise);
// link reports whether the caller may also change it, so the thread's IM
// contact is linked to it. The write check is best effort: when it fails the
// contact is bound without the link, as long as the caller may read it. A schema call acts for no user: it is trusted to
// bind, but with no rights to check it never links (changes) the contact.
// Clearing the binding needs no access.
func (s *contactCandidates) CheckCRMContactAccess(ctx context.Context, crmContactID string) (link bool, err error) {
	const errID = "service.contact_candidates.check_access"

	if crmContactID == "" || isSchemaCall(ctx) {
		return false, nil
	}

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return false, auth.IdentityNotFoundErr
	}

	if !mayUseCRM(ctx, identity) {
		return false, errors.Wrap(auth.ForbiddenIssuerErr, errors.WithID(errID))
	}

	ctx, cancel := context.WithTimeout(ctx, crmCheckTimeout)
	defer cancel()

	ids := []string{crmContactID}

	writable, err := s.crm.WritableContacts(ctx, ids)

	switch {
	case err == nil && writable[crmContactID]:
		return true, nil
	case err != nil && errors.Is(err, crmcontacts.ErrNoAccessToken):
		return false, s.accessError(ctx, err)
	case err != nil && !errors.Is(err, crmcontacts.ErrNoCRMAccess):
		s.logger.WarnContext(ctx, "crm write check failed; the contact is bound without the link", slog.Any("err", err))
	}

	visible, err := s.crm.VisibleContacts(ctx, ids)
	if errors.Is(err, crmcontacts.ErrNoCRMAccess) {
		return false, errors.Forbidden("the crm contact is not accessible", errors.WithID(errID))
	}

	if err != nil {
		return false, s.accessError(ctx, err)
	}

	if !visible[crmContactID] {
		return false, errors.Forbidden("the crm contact is not accessible", errors.WithID(errID))
	}

	return false, nil
}

// visible filters by the caller's CRM read access: the candidates and the
// bound contact; nil means no filtering, which only schema calls get.
func (s *contactCandidates) visible(ctx context.Context, candidates []*threadv1.ContactCandidate, bound string) (map[string]bool, error) {
	if isSchemaCall(ctx) {
		return nil, nil //nolint:nilnil // nil map is the documented "no filter" for schema calls
	}

	ctx, cancel := context.WithTimeout(ctx, crmCheckTimeout)
	defer cancel()

	ids := make([]string, 0, len(candidates)+1)
	for _, c := range candidates {
		ids = append(ids, c.GetCrmContactId())
	}

	if bound != "" && !slices.Contains(ids, bound) {
		ids = append(ids, bound)
	}

	visible, err := s.crm.VisibleContacts(ctx, ids)
	if errors.Is(err, crmcontacts.ErrNoCRMAccess) {
		// No access to contacts at all: none of them is shown.
		return make(map[string]bool), nil
	}

	if err != nil {
		return nil, s.accessError(ctx, err)
	}

	return visible, nil
}

func (s *contactCandidates) accessError(ctx context.Context, err error) error {
	const errID = "service.contact_candidates.crm_access"

	if errors.Is(err, crmcontacts.ErrNoAccessToken) {
		return errors.Forbidden("a webitel access token is required", errors.WithID(errID))
	}

	s.logger.ErrorContext(ctx, "crm access check failed", slog.Any("err", err))

	return errors.Unavailable("crm access check failed", errors.WithID(errID), errors.WithCause(err))
}

// mayUseCRM: a schema call (it acts for no user), or a Webitel user, whose
// token the CRM accepts and who has CRM rights to check. End users (portal,
// widget, messengers) have neither.
func mayUseCRM(ctx context.Context, identity auth.Identifier) bool {
	return isSchemaCall(ctx) || identity.GetIssuer() == WebitelIssuer
}

// isSchemaCall reports a Flow schema call over mTLS: it acts for no user, so
// there is no token to filter with. Engine calls carry the user's token and
// are filtered like any operator.
func isSchemaCall(ctx context.Context) bool {
	t, ok := auth.AuthTypeFromContext(ctx)

	return ok && t == auth.XWebitelTypeSchema
}

func toGatewayCandidate(c *threadv1.ContactCandidate) *gtwthread.ContactCandidate {
	out := &gtwthread.ContactCandidate{
		CrmContactId: c.GetCrmContactId(),
		Trust:        gtwthread.IdentificationTrust(c.GetTrust()),
		CreatedAt:    c.GetCreatedAt(),
		UpdatedAt:    c.GetUpdatedAt(),
		Reasons:      make([]*gtwthread.ContactMatchReason, 0, len(c.GetReasons())),
	}

	for _, r := range c.GetReasons() {
		out.Reasons = append(out.Reasons, &gtwthread.ContactMatchReason{
			Type:   gtwthread.IdentificationCriterionType(r.GetType()),
			Trust:  gtwthread.IdentificationTrust(r.GetTrust()),
			Origin: gtwthread.IdentificationOrigin(r.GetOrigin()),
			Key:    r.GetKey(),
		})
	}

	return out
}
