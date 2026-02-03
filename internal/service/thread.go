package service

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service/dto"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

var (
	_ ThreadSearcher = (*thread)(nil)
)

type (
	ThreadSearcher interface {
		Search(ctx context.Context, searchQuery *dto.ThreadSearchRequestDTO) ([]*dto.ThreadDTO, bool, error)
	}
)

type thread struct {
	logger *slog.Logger

	threadClient *imthread.ThreadClient
	contactClient *imcontact.Client

	converter mapper.ThreadConverter
}

func NewThread(logger *slog.Logger, threadClient *imthread.ThreadClient, contactClient *imcontact.Client, converter mapper.ThreadConverter) *thread {
	o := new(thread)
	o.logger = logger
	o.threadClient = threadClient
	o.contactClient = contactClient
	o.converter = converter
	return o
}

func (t *thread) Search(ctx context.Context, searchQuery *dto.ThreadSearchRequestDTO) ([]*dto.ThreadDTO, bool, error) {
	log := t.logger.With(slog.String("op", "thread.Search"))

	if len(searchQuery.DomainIDs) == 0 {
		log.Warn("request without domain_ids", slog.Any("request",searchQuery))
    	return nil, false, errors.New("domain_ids is required")
	}
	
	internalThreads, err := t.threadClient.Search(ctx, searchQuery)
	if err != nil {
		log.Error("failed to fetch internal threads", slog.Any("error", err))
		return nil, false, err
	}

	uniqueIds := t.collectUniqueMembersIDs(internalThreads.GetThreads())
	contactsIdentities, err := t.fetchExternalParticipantsInfo(ctx, uniqueIds, int(searchQuery.DomainIDs[0]))
	if err != nil {
		log.Error("failed to fetch internal contact information for enrichment", 
			slog.Any("error", err),
			slog.Any("ids",uniqueIds),
		)

		return nil, false, err
	}

	enrichedThreads := t.converter.ThreadV1ListToThreadDTOList(internalThreads.GetThreads())
	t.enrichThreads(enrichedThreads, contactsIdentities)

	return enrichedThreads, internalThreads.Next, nil
}

func (t *thread) enrichThreads(threads []*dto.ThreadDTO, im map[string]*dto.ExternalParticipantDTO) {
	// due to ExternalParticipant struct and mapper we use subject as internal ID saver in this case... 
	
	for _, thr := range threads {
		if owner, ok := im[thr.Owner.InternalID]; ok {
			thr.Owner = owner 
		}

		for i := range thr.Admins {
    		if ad, ok := im[thr.Admins[i].InternalID]; ok {
    		    thr.Admins[i] = ad
    		}
		}

		for i := range thr.MemberIDs {
		    if m, ok := im[thr.MemberIDs[i].InternalID]; ok {
		        thr.MemberIDs[i] = m
		    }
		}

		for i := range thr.Members {
		    if m, ok := im[thr.Members[i].Member.InternalID]; ok {
		        thr.Members[i].Member = m
		    }
		}

	}
}

func (t *thread) collectUniqueMembersIDs(threads []*threadv1.Thread) []string {
	uniqueMap := make(map[string]struct{}, len(threads))

	for _, thr := range threads {
		uniqueMap[thr.Owner] = struct{}{}
		
		for _, a := range thr.Admins {
			uniqueMap[a] = struct{}{}
		}

		for _, mi := range thr.GetMemberIds() {
			uniqueMap[mi] = struct{}{}
		}

		for _, m := range thr.GetMembers() {
			uniqueMap[m.GetId()] = struct{}{}
		}
	}

	return slices.Collect(maps.Keys(uniqueMap))
}

func (t *thread) fetchExternalParticipantsInfo(ctx context.Context, uniqueIDs []string, domainID int) (map[string]*dto.ExternalParticipantDTO, error) {
	contactInfo, err := t.contactClient.SearchContact(ctx, &contact.SearchContactRequest{
		Size: int32(len(uniqueIDs)),
		Fields: []string{"id", "issuer_id", "type", "subject_id"},
		DomainId: int32(domainID),
		Ids: uniqueIDs,
	})

	if err != nil {
		return nil, err
	}

	externalParticipantIdentities := make(map[string]*dto.ExternalParticipantDTO)

	for _, c := range contactInfo.GetContacts() {
		externalParticipantIdentities[c.GetId()] = &dto.ExternalParticipantDTO{
			Issuer: c.GetIssId(),
			Subject: c.GetSubject(),
			Type:    c.GetType(),
		}
	}

	return externalParticipantIdentities, nil
}