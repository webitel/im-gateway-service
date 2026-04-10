package service

import (
	"cmp"
	"context"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwthread "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
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
	_ ThreadManager = (*thread)(nil)
)

type ThreadManager interface {
	Search(ctx context.Context, searchQuery *dto.ThreadSearchRequestDTO) ([]*dto.ThreadDTO, bool, error)
	AddMember(ctx context.Context, req *gtwthread.AddMemberRequest) error
	RemoveMember(ctx context.Context, req *gtwthread.RemoveMemberRequest) error
	SetVariables(ctx context.Context, req *gtwthread.SetVariablesRequest) (*gtwthread.ThreadVariables, error)
	SearchVariables(ctx context.Context, req *gtwthread.SearchVariablesRequest) (*gtwthread.SearchVariablesResponse, error)
	LocateVariables(ctx context.Context, req *gtwthread.LocateVariablesRequest) (*gtwthread.ThreadVariables, error)
	FlushVariables(ctx context.Context, req *gtwthread.FlushVariablesRequest) (*gtwthread.ThreadVariables, error)
}

type thread struct {
	logger *slog.Logger

	threadClient  *imthread.ThreadClient
	contactClient *imcontact.Client

	converter mapper.ThreadConverter
}

func (t *thread) AddMember(ctx context.Context, req *gtwthread.AddMemberRequest) error {
	if req == nil {
		return errors.New("request is nil")
	}
	if req.GetMember() == nil {
		return errors.New("new member is required")
	}
	if req.GetThreadId() == "" {
		return errors.New("thread id is required")
	}
	if req.GetRole() == gtwthread.ThreadRole_ROLE_UNSPECIFIED {
		return errors.New("role is required")
	}
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return auth.IdentityNotFoundErr
	}
	target, err := t.findContact(ctx, req.GetMember().GetSub(), req.GetMember().GetIss(), int32(identity.GetDomainID()))
	if err != nil {
		return err
	}

	addMemberRequest := &threadv1.AddMemberRequest{
		ThreadId:    req.GetThreadId(),
		NewMemberId: target.GetId(),
		Role:        threadv1.ThreadRole(req.Role),
		InitiatorId: identity.GetContactID(),
	}

	return t.threadClient.AddMember(ctx, addMemberRequest)
}

func (t *thread) RemoveMember(ctx context.Context, req *gtwthread.RemoveMemberRequest) error {
	if req == nil {
		return errors.New("request is nil")
	}
	if req.GetMember() == nil {
		return errors.New("target id is required")
	}
	if req.GetThreadId() == "" {
		return errors.New("thread id is required")
	}

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return auth.IdentityNotFoundErr
	}
	target, err := t.findContact(ctx, req.GetMember().GetSub(), req.GetMember().GetIss(), int32(identity.GetDomainID()))
	if err != nil {
		return err
	}

	removeMemberRequest := &threadv1.RemoveMemberRequest{
		ThreadId:          req.GetThreadId(),
		TargetMemberId:    target.GetId(),
		InitiatorMemberId: identity.GetContactID(),
	}
	return t.threadClient.RemoveMember(ctx, removeMemberRequest)
}

func (t *thread) findContact(ctx context.Context, sub, iss string, domainID int32) (*contact.Contact, error) {
	target, err := t.contactClient.SearchContact(ctx, &contact.SearchContactRequest{
		Subjects: []string{sub},
		IssId:    []string{iss},
		DomainId: domainID,
	})
	if err != nil {
		return nil, err
	}
	if len(target.GetContacts()) == 0 {
		return nil, errors.New("no contact found for new member")
	}
	return target.GetContacts()[0], nil
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

	internalOwners, _, err := t.collectInternalContactsIDs(ctx, searchQuery.Owners, nil, int32(identity.GetDomainID()))
	if err != nil {
		log.Error("failed to fetch internal thread participants", slog.Any("error", err))
		return nil, false, err
	}

	internalThreads, err := t.threadClient.Search(ctx, &threadv1.ThreadSearchRequest{
		Fields:    searchQuery.Fields,
		Ids:       searchQuery.IDs,
		DomainIds: []int32{int32(identity.GetDomainID())},
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

	uniqueIds := t.collectUniqueMembersIDs(internalThreads.GetItems())

	contactsIdentities, err := t.fetchExternalParticipantsInfo(ctx, uniqueIds, int(identity.GetDomainID()))
	if err != nil {
		log.Error(
			"failed to fetch internal contact information for enrichment",
			slog.Any("error", err),
			slog.Any("ids", uniqueIds),
		)

		return nil, false, err
	}

	enrichedThreads := t.converter.ThreadV1ListToThreadDTOList(internalThreads.GetItems())
	t.enrichThreads(enrichedThreads, contactsIdentities, identity.GetContactID())

	return enrichedThreads, internalThreads.Next, nil
}

func (t *thread) SetVariables(ctx context.Context, req *gtwthread.SetVariablesRequest) (*gtwthread.ThreadVariables, error) {
	log := t.logger.With("operation", "service.thread.set_variables")

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.Warn("identity not found in context")
		return nil, auth.IdentityNotFoundErr
	}

	response, err := t.threadClient.SetVariables(ctx, &threadv1.SetVariablesRequest{
		ThreadId:  req.ThreadId,
		Variables: convertToThreadProto(req.GetVariables()),
	})

	if err != nil {
		log.Error("internal service set variables", "err", err, "thread_id", req.GetThreadId(), "contact_id", identity.GetContactID())
		return nil, err
	}

	threadVars, err := t.convertToThreadVariables(ctx, response, int32(identity.GetDomainID()))
	if err != nil {
		log.Error("convert to thread variables", "err", err, "thread_id", req.GetThreadId(), "contact_id", identity.GetContactID())
		return nil, err
	}

	return threadVars, nil
}

func (t *thread) SearchVariables(ctx context.Context, req *gtwthread.SearchVariablesRequest) (*gtwthread.SearchVariablesResponse, error) {
	var log = t.logger.With("operation", "service.thread.search_variables")

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.Warn("identity not found in context")
		return nil, auth.IdentityNotFoundErr
	}

	response, err := t.threadClient.SearchVariables(ctx, &threadv1.SearchVariablesRequest{
		Size:      req.GetSize(),
		Page:      req.GetPage(),
		Fields:    req.GetFields(),
		ThreadIds: req.GetThreadIds(),
	})

	if err != nil {
		log.Error("search variables", "err", err)
		return nil, err
	}

	var vars []*gtwthread.ThreadVariables
	for _, item := range response.GetItems() {
		v, err := t.convertToThreadVariables(ctx, item, int32(identity.GetDomainID()))
		if err != nil {
			log.Error("convert to thread variables", "err", err)
			return nil, err
		}
		vars = append(vars, v)
	}

	return &gtwthread.SearchVariablesResponse{
		Items: vars,
		Next:  response.GetNext(),
	}, nil
}

func (t *thread) LocateVariables(ctx context.Context, req *gtwthread.LocateVariablesRequest) (*gtwthread.ThreadVariables, error) {
	var log = t.logger.With("operation", "service.thread.locate_variables")

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.Warn("identity not found in context")
		return nil, auth.IdentityNotFoundErr
	}

	response, err := t.threadClient.LocateVariables(ctx, &threadv1.LocateVariablesRequest{
		ThreadId: req.GetThreadId(),
	})

	if err != nil {
		log.Error("locate variables", "err", err)
		return nil, err
	}

	v, err := t.convertToThreadVariables(ctx, response, int32(identity.GetDomainID()))
	if err != nil {
		log.Error("convert to thread variables", "err", err)
		return nil, err
	}

	return v, nil
}

func (t *thread) FlushVariables(ctx context.Context, req *gtwthread.FlushVariablesRequest) (*gtwthread.ThreadVariables, error) {
	var log = t.logger.With("operation", "service.thread.flush_variables")

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.Warn("identity not found in context")
		return nil, auth.IdentityNotFoundErr
	}

	response, err := t.threadClient.FlushVariables(ctx, &threadv1.FlushVariablesRequest{
		ThreadId: req.GetThreadId(),
		Keys:     req.GetKeys(),
	})

	if err != nil {
		log.Error("flush variables", "err", err)
		return nil, err
	}

	v, err := t.convertToThreadVariables(ctx, response, int32(identity.GetDomainID()))
	if err != nil {
		log.Error("convert to thread variables", "err", err)
		return nil, err
	}

	return v, nil
}

func (t *thread) enrichThreads(threads []*dto.ThreadDTO, im map[string]*dto.ExternalParticipantDTO, sessionMemberID string) {
	for _, thr := range threads {
		t.enrichThreadMembers(thr, im, sessionMemberID)
		t.enrichLastMessageSenders(thr, im)
		t.enrichThreadVariables(thr, im)
	}
}

func (t *thread) enrichThreadMembers(thr *dto.ThreadDTO, im map[string]*dto.ExternalParticipantDTO, sessionMemberID string) {
	for i := range thr.Members {
		internalID := thr.Members[i].Member.InternalID
		if m, ok := im[internalID]; ok {
			thr.Members[i].Member = m
			t.updateDirectThreadSubject(thr, thr.Members[i], internalID, sessionMemberID)
		}
	}
}

func (t *thread) enrichLastMessageSenders(thr *dto.ThreadDTO, im map[string]*dto.ExternalParticipantDTO) {
	if thr.LastMsg != nil {
		if member, ok := im[thr.LastMsg.SenderID]; ok {
			thr.LastMsg.Sender = &dto.MessageSender{
				Sub:   member.Sub,
				Iss:   member.Iss,
				Name:  member.Name,
				IsBot: member.IsBot,
				Type:  member.Type,
			}
		}
	}
}

func (t *thread) enrichThreadVariables(thr *dto.ThreadDTO, im map[string]*dto.ExternalParticipantDTO) {
	if thr.Variables == nil {
		return
	}

	for k, v := range thr.Variables.Variables {
		if member, ok := im[v.SetByInternalID]; ok {
			thr.Variables.Variables[k] = &dto.VariableEntryDTO{
				Value: v.Value,
				SetBy: &gtwthread.Contact{
					Iss:   member.Iss,
					Sub:   member.Sub,
					Type:  member.Type,
					Name:  member.Name,
					IsBot: member.IsBot,
				},
				SetAt: v.SetAt,
			}
		}
	}
}

func (t *thread) updateDirectThreadSubject(thr *dto.ThreadDTO, member *dto.ThreadMemberDTO, internalID, sessionMemberID string) {
	if thr.Kind != dto.ThreadKindDirect {
		return
	}

	if internalID == sessionMemberID {
		return
	}

	if member.Member == nil {
		return
	}

	thr.Subject = member.Member.Name
}

func (t *thread) collectUniqueMembersIDs(threads []*threadv1.Thread) []string {
	uniqueMap := make(map[string]struct{}, len(threads))

	for _, thr := range threads {
		for _, m := range thr.GetMembers() {
			uniqueMap[m.GetMemberId()] = struct{}{}
		}

		if thr.LastMsg != nil {
			uniqueMap[thr.LastMsg.SenderId] = struct{}{}
		}

		if thr.Variables != nil {
			for _, v := range thr.Variables.GetVariables() {
				uniqueMap[v.GetSetBy()] = struct{}{}
			}
		}
	}

	return slices.Collect(maps.Keys(uniqueMap))
}

func (t *thread) fetchExternalParticipantsInfo(ctx context.Context, uniqueIDs []string, domainID int) (map[string]*dto.ExternalParticipantDTO, error) {
	contactInfo, err := t.contactClient.SearchContact(ctx, &contact.SearchContactRequest{
		Size:     int32(len(uniqueIDs)),
		Fields:   []string{"id", "issuer_id", "type", "subject_id", "username", "name", "is_bot"},
		DomainId: int32(domainID),
		Ids:      uniqueIDs,
	})

	if err != nil {
		return nil, err
	}

	externalParticipantIdentities := make(map[string]*dto.ExternalParticipantDTO)

	for _, c := range contactInfo.GetContacts() {
		externalParticipantIdentities[c.GetId()] = &dto.ExternalParticipantDTO{
			Iss:   c.GetIssId(),
			Sub:   c.GetSubject(),
			Type:  c.GetType(),
			Name:  cmp.Or(c.GetName(), c.GetUsername()),
			IsBot: c.IsBot,
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
		Size:     int32(requestSize),
		Fields:   []string{"id"},
		IssId:    issuers,
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

func convertToThreadProto(request []*gtwthread.VariableEntryRequest) []*threadv1.VariableEntryRequest {
	protoRequests := make([]*threadv1.VariableEntryRequest, len(request))
	for i, req := range request {
		protoReq := &threadv1.VariableEntryRequest{
			Key:   req.Key,
			Value: req.Value,
		}
		protoRequests[i] = protoReq
	}
	return protoRequests
}

func (t *thread) convertToThreadVariables(ctx context.Context, response *threadv1.ThreadVariables, domainID int32) (*gtwthread.ThreadVariables, error) {
	var log = t.logger.With("operation", "service.thread.convert_to_thread_variables")

	var uniqueSettersSet = make(map[string]struct{})
	for _, v := range response.Variables {
		uniqueSettersSet[v.SetBy] = struct{}{}
	}
	var uniqueSettersIDs []string = slices.Collect(maps.Keys(uniqueSettersSet))

	contacts, err := t.contactClient.SearchContact(ctx, &contactv1.SearchContactRequest{
		Size:     int32(len(uniqueSettersIDs)),
		Fields:   []string{"id", "issuer_id", "type", "subject_id", "username", "name", "is_bot"},
		DomainId: domainID,
		Ids:      uniqueSettersIDs,
	})

	if err != nil {
		log.ErrorContext(ctx, "search variables setters contacts", "err", err)
		return nil, err
	}

	var externalSettersMap = make(map[string]*gtwthread.Contact, len(contacts.GetContacts()))
	for _, c := range contacts.GetContacts() {
		externalSettersMap[c.Id] = &gtwthread.Contact{
			Iss:   c.GetIssId(),
			Sub:   c.GetSubject(),
			Type:  c.GetType(),
			Name:  coalesceString(c.GetName(), c.GetUsername(), NoNameRecipient),
			IsBot: c.IsBot,
		}
	}

	vars := make(map[string]*gtwthread.VariableEntry, len(response.Variables))
	for k, v := range response.Variables {
		setBy, ok := externalSettersMap[v.SetBy]
		if !ok {
			log.Warn("not found external setter", "internal_id", v.SetBy)
		}

		vars[k] = &gtwthread.VariableEntry{
			Value: v.Value,
			SetBy: setBy,
			SetAt: v.SetAt,
		}
	}

	return &gtwthread.ThreadVariables{
		ThreadId:  response.ThreadId,
		Variables: vars,
	}, nil
}
