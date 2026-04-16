package service

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwthread "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/webitel-go-kit/pkg/errors"
)

type ThreadManager interface {
	Search(ctx context.Context, searchQuery *gtwthread.ThreadSearchRequest) ([]*gtwthread.Thread, bool, error)
	AddMember(ctx context.Context, req *gtwthread.AddMemberRequest) (*gtwthread.AddMemberResponse, error)
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
}

func (t *thread) AddMember(ctx context.Context, req *gtwthread.AddMemberRequest) (*gtwthread.AddMemberResponse, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.GetContact() == nil {
		return nil, errors.New("new member is required")
	}
	if req.GetThreadId() == "" {
		return nil, errors.New("thread id is required")
	}
	if req.GetRole() == gtwthread.ThreadRole_ROLE_UNSPECIFIED {
		return nil, errors.New("role is required")
	}
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}
	target, err := t.fetchContact(ctx, req.GetContact().GetSub(), req.GetContact().GetIss(), int32(identity.GetDomainID()))
	if err != nil {
		return nil, err
	}
	initiatorContactId := identity.GetContactID()
	addMemberRequest := &threadv1.AddMemberRequest{
		ThreadId:           req.GetThreadId(),
		NewMemberContactId: target.GetId(),
		Role:               threadv1.ThreadRole(req.Role),
		InitiatorContactId: &initiatorContactId,
	}

	response, err := t.threadClient.AddMember(ctx, addMemberRequest)
	if err != nil {
		return nil, err
	}

	return &gtwthread.AddMemberResponse{Member: &gtwthread.ThreadMember{
		Id: response.GetMember().GetId(),
	}}, nil
}

func (t *thread) RemoveMember(ctx context.Context, req *gtwthread.RemoveMemberRequest) error {
	if req == nil {
		return errors.New("request is nil")
	}
	if req.GetMemberId() == "" {
		return errors.New("target id is required")
	}
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return auth.IdentityNotFoundErr
	}
	initiatorContactId := identity.GetContactID()
	removeMemberRequest := &threadv1.RemoveMemberRequest{
		TargetMemberId:     req.GetMemberId(),
		InitiatorContactId: &initiatorContactId,
	}
	return t.threadClient.RemoveMember(ctx, removeMemberRequest)
}

func (t *thread) fetchContact(ctx context.Context, sub, iss string, domainID int32) (*contact.Contact, error) {
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

func (t *thread) fetchContacts(ctx context.Context, ids []string, domainID int32) (map[string]*contact.Contact, error) {
	contactInfo, err := t.contactClient.SearchContact(ctx, &contact.SearchContactRequest{
		Size:     int32(len(ids)),
		Ids:      ids,
		DomainId: domainID,
	})

	if err != nil {
		return nil, err
	}

	contacts := map[string]*contact.Contact{}

	for _, c := range contactInfo.GetContacts() {
		contacts[c.GetId()] = c
	}

	return contacts, nil
}

func NewThread(logger *slog.Logger, threadClient *imthread.ThreadClient, contactClient *imcontact.Client) *thread {
	o := new(thread)
	o.logger = logger
	o.threadClient = threadClient
	o.contactClient = contactClient
	return o
}

func (t *thread) Search(ctx context.Context, searchQuery *gtwthread.ThreadSearchRequest) ([]*gtwthread.Thread, bool, error) {
	log := t.logger.With(slog.String("op", "thread.Search"))

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.ErrorContext(ctx, "identity not found")
		return nil, false, auth.IdentityNotFoundErr
	}

	internalThreads, err := t.threadClient.Search(ctx, &threadv1.ThreadSearchRequest{
		Fields:    searchQuery.Fields,
		Ids:       searchQuery.Ids,
		DomainIds: []int32{int32(identity.GetDomainID())},
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

	uniqueContactIds := t.collectUniqueContactsFromThread(internalThreads.GetItems())
	contacts, err := t.fetchContacts(ctx, uniqueContactIds, int32(identity.GetDomainID()))
	if err != nil {
		log.Error(
			"failed to fetch internal contact information for enrichment",
			slog.Any("error", err),
			slog.Any("ids", uniqueContactIds),
		)

		return nil, false, err
	}

	res := []*gtwthread.Thread{}
	for _, thr := range internalThreads.GetItems() {
		converted := convertToThread(thr, contacts)
		res = append(res, converted)
	}

	return res, internalThreads.Next, nil
}

func (t *thread) SetVariables(ctx context.Context, req *gtwthread.SetVariablesRequest) (*gtwthread.ThreadVariables, error) {
	log := t.logger.With("operation", "service.thread.set_variables")

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.Warn("identity not found in context")
		return nil, auth.IdentityNotFoundErr
	}

	response, err := t.threadClient.SetVariables(ctx, &threadv1.SetVariablesRequest{
		MemberId:  identity.GetContactID(),
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
		MemberId: identity.GetContactID(),
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

func (t *thread) collectUniqueContactsFromThread(threads []*threadv1.Thread) []string {
	uniqueMap := map[string]struct{}{}

	for _, thr := range threads {
		for _, m := range thr.GetMembers() {
			if m.GetContactId() == "" {
				continue
			}
			uniqueMap[m.GetContactId()] = struct{}{}
		}

		if thr.LastMsg != nil {
			senderID := thr.LastMsg.GetSenderId()
			if senderID != "" {
				uniqueMap[senderID] = struct{}{}
			}
		}

		if thr.Variables != nil {
			for _, v := range thr.Variables.GetVariables() {
				setBy := v.GetSetBy()
				if setBy == "" {
					continue
				}
				uniqueMap[setBy] = struct{}{}
			}
		}
	}

	return slices.Collect(maps.Keys(uniqueMap))
}

func convertToThread(thr *threadv1.Thread, contactData map[string]*contact.Contact) *gtwthread.Thread {
	var (
		members               = make([]*gtwthread.ThreadMember, 0, len(thr.GetMembers()))
		findLastMessageSender = thr.LastMsg != nil
		lastMessageSender     *gtwthread.ThreadMember
	)
	for _, toConvert := range thr.GetMembers() {
		correspondingContact, ok := contactData[toConvert.GetContactId()]
		if !ok {
			correspondingContact = &contact.Contact{
				Subject: toConvert.GetContactId(),
			}
		}
		member := convertToMember(toConvert, correspondingContact)
		members = append(members, member)

		if findLastMessageSender && toConvert.GetContactId() == thr.LastMsg.GetSenderId() {
			findLastMessageSender = false
			lastMessageSender = member
		}
	}

	var threadVars *gtwthread.ThreadVariables
	if thr.Variables != nil {
		vars := make(map[string]*gtwthread.VariableEntry, len(thr.Variables.Variables))
		for k, v := range thr.Variables.Variables {
			contact := contactData[v.GetSetBy()]
			vars[k] = &gtwthread.VariableEntry{
				Value: v.GetValue(),
				SetBy: convertToContact(contact),
				SetAt: v.GetSetAt(),
			}
		}
		threadVars = &gtwthread.ThreadVariables{
			Variables: vars,
		}
	}

	return &gtwthread.Thread{
		Id:          thr.GetId(),
		Subject:     thr.GetSubject(),
		Kind:        gtwthread.ThreadKind(thr.GetKind()),
		CreatedAt:   thr.GetCreatedAt(),
		UpdatedAt:   thr.GetUpdatedAt(),
		Description: thr.GetDescription(),
		LastMsg:     convertToMessage(thr.GetLastMsg(), lastMessageSender),
		Members:     members,
		Variables:   threadVars,
	}
}

func convertToMember(m *threadv1.ThreadMember, contact *contact.Contact) *gtwthread.ThreadMember {
	converted := &gtwthread.ThreadMember{
		Id:      m.GetId(),
		Contact: convertToContact(contact),
		Role:    gtwthread.ThreadRole(m.GetRole()),
	}
	if m.Permissions != nil {
		converted.Permissions = &gtwthread.ThreadPermissions{
			CanSendMessages:             m.Permissions.CanSendMessages,
			CanAddMembers:               m.Permissions.CanAddMembers,
			CanRemoveMembers:            m.Permissions.CanRemoveMembers,
			CanChangeMembersPermissions: m.Permissions.CanChangeMembersPermissions,
			CanChangeThreadInfo:         m.Permissions.CanChangeThreadInfo,
		}
	}
	return converted

}

func convertToMessage(req *threadv1.HistoryMessage, sender *gtwthread.ThreadMember) *gtwthread.HistoryMessage {
	if req == nil {
		return nil
	}
	return &gtwthread.HistoryMessage{
		Id:        req.GetId(),
		ThreadId:  req.GetThreadId(),
		CreatedAt: req.GetCreatedAt(),
		UpdatedAt: req.GetUpdatedAt(),
		Type:      req.GetType(),
		Body:      req.GetBody(),
		Metadata:  req.GetMetadata(),
		Documents: convertDocuments(req.GetDocuments()),
		Images:    convertImages(req.GetImages()),
		Sender:    sender,
	}
}

func convertDocuments(reqDocs []*threadv1.Document) []*gtwthread.Document {
	if len(reqDocs) == 0 {
		return nil
	}

	docs := make([]*gtwthread.Document, len(reqDocs))
	for i, d := range reqDocs {
		docs[i] = &gtwthread.Document{
			Id:        d.GetId(),
			MessageId: d.GetMessageId(),
			FileId:    d.GetFileId(),
			Name:      d.GetName(),
			Mime:      d.GetMime(),
		}
	}
	return docs
}

func convertImages(reqImgs []*threadv1.Image) []*gtwthread.Image {
	if len(reqImgs) == 0 {
		return nil
	}

	imgs := make([]*gtwthread.Image, len(reqImgs))
	for i, img := range reqImgs {
		imgs[i] = &gtwthread.Image{
			Id:        img.GetId(),
			MessageId: img.GetMessageId(),
			FileId:    img.GetFileId(),
			Mime:      img.GetMime(),
			Width:     img.GetWidth(),
			Height:    img.GetHeight(),
		}
	}
	return imgs
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

func convertToContact(c *contact.Contact) *gtwthread.Contact {
	return &gtwthread.Contact{
		Iss:      c.GetIssId(),
		Sub:      c.GetSubject(),
		Type:     c.GetType(),
		Name:     coalesceString(c.GetName(), c.GetUsername(), NoNameRecipient),
		IsBot:    c.GetIsBot(),
		Username: c.GetUsername(),
	}
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
