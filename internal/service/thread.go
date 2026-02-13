package service

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service/dto"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

var (
	internalContactsNotFoundErr = errors.New("internal provider return no records")
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

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.ErrorContext(ctx, "identity not found")
		return nil, false, auth.IdentityNotFoundErr
	}

	internalOwners, _, err := t.collectInternalContactsIDs(ctx,searchQuery.Owners,nil,int32(identity.GetDomainID()))
	if err != nil {	
		log.Error("failed to fetch internal thread participants", slog.Any("error", err))
		return nil, false, err
	}
	
	internalThreads, err := t.threadClient.Search(ctx, &threadv1.ThreadSearchRequest{
		Fields:    searchQuery.Fields,
		Ids:       searchQuery.IDs,
		DomainIds: []int32{int32(identity.GetDomainID())},
		Kinds:     t.converter.DTOKindsToThreadV1Kinds(searchQuery.Kinds),
		Owners:    internalOwners,
		Q:         searchQuery.Q,
		MemberIds: []string{identity.GetContactID()},
		Size:      searchQuery.Size,
		Sort:      searchQuery.Sort,
		Page:      searchQuery.Page,
	})
	if err != nil {
		log.Error("failed to fetch internal threads", slog.Any("error", err))
		return nil, false, err
	}

	uniqueIds := t.collectUniqueMembersIDs(internalThreads.GetThreads())
	contactsIdentities, err := t.fetchExternalParticipantsInfo(ctx, uniqueIds, int(identity.GetDomainID()))
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

func (t *thread) groupSubByIssuers(peers []shared.Peer) ([]string, []string) {
    uniqueIssuers := make(map[string]struct{})
    uniqueIDs := make(map[string]struct{})

    for _, p := range peers {
        uniqueIssuers[p.Issuer] = struct{}{}
        uniqueIDs[p.ID] = struct{}{}
    }

    issuers := make([]string, 0, len(uniqueIssuers))
    for k := range uniqueIssuers {
        issuers = append(issuers, k)
    }

    ids := make([]string, 0, len(uniqueIDs))
    for k := range uniqueIDs {
        ids = append(ids, k)
    }

    return issuers, ids
}

func (t *thread) fetchInternalParticipants(ctx context.Context, issuers, subjects []string, domainID int32) ([]string, error) {
	issLen, subLen := len(issuers), len(subjects)
	if issLen == 0 || subLen == 0 {
		return nil, nil // ignore in this case
	}

	requestSize := max(issLen, subLen)

	response, err := t.contactClient.SearchContact(ctx, &contact.SearchContactRequest{
		Size: int32(requestSize),
		Fields: []string{"id"},
		IssId: issuers,
		Subjects: subjects,
		DomainId: domainID,
	})

	if err != nil {
		return nil, err
	}

	if len(response.GetContacts()) == 0 {
		return nil, internalContactsNotFoundErr
	}

	resultIDs := make([]string, 0, len(response.GetContacts()))
	for _, contact := range response.GetContacts() {
		resultIDs = append(resultIDs, contact.Id)
	}

	return resultIDs, nil
}


func (t *thread) collectInternalContactsIDs(ctx context.Context, owners, members []shared.Peer, domainID int32) ([]string, []string, error) {
    var (
        ownersIss, ownersSub   = t.groupSubByIssuers(owners)
        membersIss, membersSub = t.groupSubByIssuers(members)
    )

    type result struct {
        ids []string
        err error
    }

    var wg sync.WaitGroup
    ownersResult := make(chan result, 1)
    membersResult := make(chan result, 1)

    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    wg.Add(2)
    
    go func() {
        defer wg.Done()
        ids, err := t.fetchInternalParticipants(ctx, ownersIss, ownersSub, domainID)
        select {
        case ownersResult <- result{ids: ids, err: err}:
        case <-ctx.Done():
        }
    }()

    go func() {
        defer wg.Done()
        ids, err := t.fetchInternalParticipants(ctx, membersIss, membersSub, domainID)
        select {
        case membersResult <- result{ids: ids, err: err}:
        case <-ctx.Done():
        }
    }()

    go func() {
        wg.Wait()
        close(ownersResult)
        close(membersResult)
    }()

    var ownersRes, membersRes result
    var ownersReceived, membersReceived bool

    for !ownersReceived || !membersReceived {
        select {
        case res, ok := <-ownersResult:
            if ok {
                ownersRes = res
                ownersReceived = true
            }
        case res, ok := <-membersResult:
            if ok {
                membersRes = res
                membersReceived = true
            }
        case <-ctx.Done():
            return nil, nil, ctx.Err()
        }
    }
    
    if ownersRes.err != nil && !errors.Is(ownersRes.err, internalContactsNotFoundErr) {
        return nil, nil, ownersRes.err
    }
    if membersRes.err != nil && !errors.Is(membersRes.err, internalContactsNotFoundErr) {
        return nil, nil, membersRes.err
    }

    return ownersRes.ids, membersRes.ids, nil
}