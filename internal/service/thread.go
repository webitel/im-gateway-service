package service

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"

	"google.golang.org/grpc/metadata"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	gtwthread "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
)

const (
	WebitelIssuer = "webitel"
)

type ThreadManager interface {
	Search(ctx context.Context, searchQuery *gtwthread.ThreadSearchRequest) (*gtwthread.SearchThreadResponse, error)
	SearchLeft(ctx context.Context, request *gtwthread.SearchLeftRequest) ([]*gtwthread.Thread, bool, error)
	GetCounters(ctx context.Context, req *gtwthread.GetCountersRequest) (*gtwthread.GetCountersResponse, error)
	Get(ctx context.Context, req *gtwthread.GetThreadRequest) (*gtwthread.Thread, error)
	AddMember(ctx context.Context, req *gtwthread.AddMemberRequest) (*gtwthread.AddMemberResponse, error)
	Transfer(ctx context.Context, req *gtwthread.TransferRequest) (*gtwthread.TransferResponse, error)
	RemoveMember(ctx context.Context, req *gtwthread.RemoveMemberRequest) error
	HandBackToBot(ctx context.Context, req *gtwthread.HandBackToBotRequest) error
	TakeOverFromBot(ctx context.Context, req *gtwthread.TakeOverFromBotRequest) error
	SetCrmContact(ctx context.Context, req *gtwthread.SetCrmContactRequest) (*gtwthread.SetCrmContactResponse, error)
	SetVariables(ctx context.Context, req *gtwthread.SetVariablesRequest) (*gtwthread.ThreadVariables, error)
	SearchVariables(ctx context.Context, req *gtwthread.SearchVariablesRequest) (*gtwthread.SearchVariablesResponse, error)
	LocateVariables(ctx context.Context, req *gtwthread.LocateVariablesRequest) (*gtwthread.ThreadVariables, error)
	FlushVariables(ctx context.Context, req *gtwthread.FlushVariablesRequest) (*gtwthread.ThreadVariables, error)
	Create(cxt context.Context, req *gtwthread.ThreadManagementCreateRequest) (*gtwthread.ThreadManagementCreateResponse, error)
}

type thread struct {
	logger *slog.Logger

	threadClient  *imthread.ThreadClient
	contactClient *imcontact.Client
	channels      ThreadChannelResolver
	appConfig     AppConfigProvider
	crmAccess     CRMContactAccessChecker
	crmBinder     crmContactBinder
}

// crmContactBinder stores the CRM contact chosen for a thread in im-thread.
type crmContactBinder interface {
	SetCrmContact(ctx context.Context, req *threadv1.SetCrmContactRequest) (*threadv1.SetCrmContactResponse, error)
}

// CRMContactAccessChecker decides what binding a CRM contact may do for the
// caller: it fails when the caller may not read the contact, and link reports
// whether the caller may also change it.
type CRMContactAccessChecker interface {
	CheckCRMContactAccess(ctx context.Context, crmContactID string) (link bool, err error)
}

func (t *thread) prepareCreateDirectConfig(ctx context.Context, req *gtwthread.ThreadManagementCreateRequest, session auth.Identifier) (*threadv1.ThreadManagementCreateRequest, error) {
	directConfig, ok := req.GetType().(*gtwthread.ThreadManagementCreateRequest_Direct)
	if !ok {
		return nil, errors.InvalidArgument("received non direct config call", errors.WithID("service.thread.prepare_create_direct_config.assert"), errors.WithValue("type", fmt.Sprintf("%T", req.GetType())))
	}

	directMember, err := t.fetchAllowedMember(ctx, session, directConfig.Direct.GetMember().GetSub(), directConfig.Direct.GetMember().GetIss())
	if err != nil {
		return nil, err
	}

	return &threadv1.ThreadManagementCreateRequest{
		DomainId: session.GetDomainID(),
		Initiator: &threadv1.Peer{
			Kind: &threadv1.Peer_ContactId{
				ContactId: session.GetContactID(),
			},
			Identity: &threadv1.Identity{
				Issuer: session.GetIssuer(),
				Name:   session.GetName(),
			},
		},
		Type: &threadv1.ThreadManagementCreateRequest_Direct{
			Direct: &threadv1.DirectConfig{
				Member: &threadv1.Peer{
					Kind: &threadv1.Peer_ContactId{ContactId: directMember.GetId()},
					Identity: &threadv1.Identity{
						Issuer: directMember.GetIssId(),
						Name:   directMember.GetName(),
					},
				},
			},
		},
	}, nil
}

func (t *thread) Create(ctx context.Context, req *gtwthread.ThreadManagementCreateRequest) (*gtwthread.ThreadManagementCreateResponse, error) {
	session, exists := auth.GetIdentityFromContext(ctx)
	if !exists {
		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(session)
	if err != nil {
		return nil, err
	}

	var internalRequest *threadv1.ThreadManagementCreateRequest

	switch req.GetType().(type) {
	case *gtwthread.ThreadManagementCreateRequest_Direct:
		directCfg, err := t.prepareCreateDirectConfig(ctx, req, session)
		if err != nil {
			return nil, err
		}

		internalRequest = directCfg
	default:
		return nil, errors.InvalidArgument("received unknown type of thread create request", errors.WithID("service.thread.create.unknown_type"))
	}

	internalResponse, err := t.threadClient.Create(ctx, internalRequest)
	if err != nil {
		return nil, err
	}

	// TODO: fetch in other routine when thread creation is in progress
	uniqueContactIDs := t.collectUniqueContactsFromThread([]*threadv1.Thread{internalResponse.GetThread()})

	contacts, err := t.fetchContacts(ctx, uniqueContactIDs, domainID)
	if err != nil {
		t.logger.Error("failed to fetch contact information for enrichment", slog.Any("error", err))

		return nil, err
	}

	response := convertToThread(internalResponse.GetThread(), contacts)

	threadSubject := ""

	for _, c := range contacts {
		if c.GetId() != session.GetContactID() {
			threadSubject = cmp.Or(c.GetName(), c.GetUsername())

			break
		}
	}

	response.Subject = threadSubject

	return &gtwthread.ThreadManagementCreateResponse{Thread: response}, nil
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

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	target, err := t.fetchAllowedMember(ctx, identity, req.GetContact().GetSub(), req.GetContact().GetIss())
	if err != nil {
		return nil, err
	}
	// Always record who acted as the message sender. Trusted service
	// orchestrators (schema — flow_manager/call_center — or engine) also set
	// system_call so im-thread skips membership/permission checks while still
	// attributing the system message to the schema/engine contactv1. Regular user
	// calls leave system_call false so those checks are enforced.
	initiatorContactID := identity.GetContactID()
	systemCall := auth.IsSystemCall(ctx)

	response, err := t.threadClient.AddMember(ctx, &threadv1.AddMemberRequest{
		ThreadId:           req.GetThreadId(),
		NewMemberContactId: target.GetId(),
		Role:               threadv1.ThreadRole(req.GetRole()),
		DomainId:           domainID,
		InitiatorContactId: &initiatorContactID,
		SystemCall:         &systemCall,
		Queue:              queueFromContext(ctx, t.logger),
	})
	if err != nil {
		return nil, err
	}

	return &gtwthread.AddMemberResponse{Member: &gtwthread.ThreadMember{
		Id: response.GetMember().GetId(),
	}}, nil
}

func (t *thread) Transfer(ctx context.Context, req *gtwthread.TransferRequest) (*gtwthread.TransferResponse, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	if req.GetContact() == nil {
		return nil, errors.New("new member contact is required")
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

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	target, err := t.fetchAllowedMember(ctx, identity, req.GetContact().GetSub(), req.GetContact().GetIss())
	if err != nil {
		return nil, err
	}

	initiatorContactID := identity.GetContactID()

	if auth.IsSystemCall(ctx) {
		if req.GetInitiator() == nil {
			return nil, errors.New("initiator is required for service-initiated transfer")
		}

		from, err := t.fetchContact(ctx, req.GetInitiator().GetSub(), req.GetInitiator().GetIss(), domainID)
		if err != nil {
			return nil, err
		}

		initiatorContactID = from.GetId()
	}

	transferRequest := &threadv1.TransferRequest{
		ThreadId:           req.GetThreadId(),
		NewMemberContactId: target.GetId(),
		Role:               threadv1.ThreadRole(req.GetRole()),
		InitiatorContactId: initiatorContactID,
		TargetIsBot:        target.GetIsBot(),
		Queue:              queueFromContext(ctx, t.logger),
	}

	response, err := t.threadClient.Transfer(ctx, transferRequest)
	if err != nil {
		return nil, err
	}

	return &gtwthread.TransferResponse{Member: &gtwthread.ThreadMember{
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

	removeMemberRequest := &threadv1.RemoveMemberRequest{
		TargetMemberId: req.GetMemberId(),
		Reason:         req.Reason,
	}

	// See AddMember: always record the actor as sender; system_call gates the
	// permission checks separately so a trusted orchestrator (schema/engine) can
	// remove members from threads it is not part of while still being attributed.
	initiatorContactID := identity.GetContactID()
	removeMemberRequest.InitiatorContactId = &initiatorContactID
	systemCall := auth.IsSystemCall(ctx)
	removeMemberRequest.SystemCall = &systemCall

	return t.threadClient.RemoveMember(ctx, removeMemberRequest)
}

func (t *thread) HandBackToBot(ctx context.Context, req *gtwthread.HandBackToBotRequest) error {
	if req.GetThreadId() == "" {
		return errors.InvalidArgument("thread id is required", errors.WithID("service.thread.hand_back_to_bot"))
	}

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return err
	}

	return t.threadClient.HandBackToBot(ctx, &threadv1.HandBackToBotRequest{
		ThreadId:           req.GetThreadId(),
		InitiatorContactId: identity.GetContactID(),
		DomainId:           domainID,
	})
}

func (t *thread) TakeOverFromBot(ctx context.Context, req *gtwthread.TakeOverFromBotRequest) error {
	if req.GetThreadId() == "" {
		return errors.InvalidArgument("thread id is required", errors.WithID("service.thread.take_over_from_bot"))
	}

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return err
	}

	return t.threadClient.TakeOverFromBot(ctx, &threadv1.TakeOverFromBotRequest{
		ThreadId:           req.GetThreadId(),
		InitiatorContactId: identity.GetContactID(),
		DomainId:           domainID,
	})
}

// readCallerID returns the contact id im-thread uses for read-access checks;
// empty for trusted inter-service (schema/engine) calls, which im-thread treats as unrestricted.
func readCallerID(ctx context.Context, identity auth.Identifier) string {
	if auth.IsSystemCall(ctx) {
		return ""
	}

	return identity.GetContactID()
}

func (t *thread) SetCrmContact(ctx context.Context, req *gtwthread.SetCrmContactRequest) (*gtwthread.SetCrmContactResponse, error) {
	if req.GetThreadId() == "" {
		return nil, errors.InvalidArgument("thread id is required", errors.WithID("service.thread.set_crm_contact"))
	}

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	// Binding needs read access to the CRM contact; linking the thread's IM
	// contact to it changes the contact, so it needs write access.
	link, err := t.crmAccess.CheckCRMContactAccess(ctx, req.GetCrmContactId())
	if err != nil {
		return nil, err
	}

	resp, err := t.crmBinder.SetCrmContact(ctx, &threadv1.SetCrmContactRequest{
		ThreadId:           req.GetThreadId(),
		CrmContactId:       req.GetCrmContactId(),
		InitiatorContactId: identity.GetContactID(),
		DomainId:           domainID,
		Source:             threadv1.CrmContactSource(req.GetSource()),
		LinkImContact:      link,
	})
	if err != nil {
		return nil, err
	}

	return &gtwthread.SetCrmContactResponse{
		CrmContactId: resp.GetCrmContactId(),
	}, nil
}

func (t *thread) fetchContact(ctx context.Context, sub, iss string, domainID int32) (*contactv1.Contact, error) {
	target, err := t.lookupContact(ctx, sub, iss, domainID)
	if err != nil {
		return nil, err
	}

	if target == nil {
		return nil, errNoContactFound()
	}

	return target, nil
}

// lookupContact returns nil, nil when no contact matches.
func (t *thread) lookupContact(ctx context.Context, sub, iss string, domainID int32) (*contactv1.Contact, error) {
	target, err := t.contactClient.SearchContact(ctx, &contactv1.SearchContactRequest{
		Subjects: []string{sub},
		IssId:    []string{iss},
		DomainId: domainID,
	})
	if err != nil {
		return nil, err
	}

	if len(target.GetContacts()) == 0 {
		return nil, nil //nolint:nilnil // nil contact means "not found"; callers check for it
	}

	return target.GetContacts()[0], nil
}

func errNoContactFound() error {
	return errors.New("no contact found for new member")
}

// fetchAllowedMember resolves the policy before the contact so a restricted application gets the
// same PermissionDenied for an unknown contact as for a forbidden one and cannot probe which
// identities exist in the domain.
func (t *thread) fetchAllowedMember(ctx context.Context, session auth.Identifier, sub, iss string) (*contactv1.Contact, error) {
	domainID, err := domainIDInt32(session)
	if err != nil {
		return nil, err
	}

	policy, err := resolveContactPolicy(ctx, t.appConfig, session)
	if err != nil {
		return nil, err
	}

	member, err := t.lookupContact(ctx, sub, iss, domainID)
	if err != nil {
		return nil, err
	}

	if member == nil && !policy.Restricted() {
		return nil, errNoContactFound()
	}

	if err := checkContactAllowed(policy, member.GetId()); err != nil {
		return nil, err
	}

	return member, nil
}

func (t *thread) fetchContacts(ctx context.Context, ids []string, domainID int32) (map[string]*contactv1.Contact, error) {
	contactInfo, err := t.contactClient.SearchContact(ctx, &contactv1.SearchContactRequest{
		Size:     sizeInt32(len(ids)),
		Ids:      ids,
		DomainId: domainID,
	})
	if err != nil {
		return nil, err
	}

	contacts := make(map[string]*contactv1.Contact)

	for _, c := range contactInfo.GetContacts() {
		contacts[c.GetId()] = c
	}

	return contacts, nil
}

func NewThread(logger *slog.Logger, threadClient *imthread.ThreadClient, contactClient *imcontact.Client, channels ThreadChannelResolver, appConfig AppConfigProvider, crmAccess CRMContactAccessChecker) *thread {
	o := new(thread)
	o.logger = logger
	o.threadClient = threadClient
	o.contactClient = contactClient
	o.channels = channels
	o.appConfig = appConfig
	o.crmAccess = crmAccess
	o.crmBinder = threadClient

	return o
}

// withChannel converts an internal thread and attaches the channel it came in through.
func (t *thread) withChannel(ctx context.Context, domainID int64, thr *threadv1.Thread, contacts map[string]*contactv1.Contact) *gtwthread.Thread {
	converted := convertToThread(thr, contacts)
	if t.channels != nil {
		converted.Channel = t.channels.Resolve(ctx, domainID, thr, contacts)
	}

	return converted
}

func gtwThreadKindToInternal(kinds []gtwthread.ThreadKind) []threadv1.ThreadKind {
	if len(kinds) == 0 {
		return nil
	}

	internal := make([]threadv1.ThreadKind, 0, len(kinds))
	for _, kind := range kinds {
		internal = append(internal, threadv1.ThreadKind(kind))
	}

	return internal
}

// participantsToInternal maps gateway PeerIdentity filters to the thread-service ContactIdentity
// form, forwarding only (sub, iss); resolution to a contact happens inside the thread service.
func participantsToInternal(participants []*gtwthread.PeerIdentity) []*threadv1.ContactIdentity {
	if len(participants) == 0 {
		return nil
	}

	internal := make([]*threadv1.ContactIdentity, 0, len(participants))
	for _, p := range participants {
		internal = append(internal, &threadv1.ContactIdentity{
			Sub: p.GetSub(),
			Iss: p.GetIss(),
		})
	}

	return internal
}

func (t *thread) GetCounters(ctx context.Context, req *gtwthread.GetCountersRequest) (*gtwthread.GetCountersResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	resp, err := t.threadClient.GetCounters(ctx, &threadv1.GetCountersRequest{
		SelfId:   identity.GetContactID(),
		DomainId: domainID,
		Kinds:    gtwThreadKindToInternal(req.GetTypes()),
	})
	if err != nil {
		return nil, err
	}

	return &gtwthread.GetCountersResponse{
		All:     convertThreadCounters(resp.GetAll()),
		Active:  convertThreadCounters(resp.GetActive()),
		Snoozed: convertThreadCounters(resp.GetSnoozed()),
	}, nil
}

func convertThreadCounters(c *threadv1.ThreadCounters) *gtwthread.ThreadCounters {
	return &gtwthread.ThreadCounters{Total: c.GetTotal(), UnreadChats: c.GetUnreadChats()}
}

func (t *thread) Search(ctx context.Context, searchQuery *gtwthread.ThreadSearchRequest) (*gtwthread.SearchThreadResponse, error) {
	log := t.logger.With(slog.String("op", "thread.Search"))

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.ErrorContext(ctx, "identity not found")

		return nil, auth.IdentityNotFoundErr
	}

	searchReq, err := newSearchRequest(searchQuery, identity, auth.HasPermission(ctx, auth.PermissionIMReadAll))
	if err != nil {
		return nil, err
	}

	domainID := searchReq.GetDomainIds()[0]

	internalThreads, err := t.threadClient.Search(ctx, searchReq)
	if err != nil {
		log.Error("failed to fetch internal threads", slog.Any("error", err))

		return nil, err
	}

	uniqueContactIDs := t.collectUniqueContactsFromThread(internalThreads.GetItems())

	contacts, err := t.fetchContacts(ctx, uniqueContactIDs, domainID)
	if err != nil {
		log.Error(
			"failed to fetch internal contact information for enrichment",
			slog.Any("error", err),
			slog.Any("ids", uniqueContactIDs),
		)

		return nil, err
	}

	res := make([]*gtwthread.Thread, 0, len(internalThreads.GetItems()))
	for _, thr := range internalThreads.GetItems() {
		res = append(res, t.withChannel(ctx, identity.GetDomainID(), thr, contacts))
	}

	return &gtwthread.SearchThreadResponse{
		Items:         res,
		Next:          internalThreads.GetNext(),
		UpdatesCursor: internalThreads.GetUpdatesCursor(),
	}, nil
}

// newSearchRequest builds the im-thread Search request for the caller. Callers granted
// im_read_all search every thread of their domain, not only the ones they are a member of.
// It fails when the caller's domain id does not fit the int32 im-thread contract.
func newSearchRequest(searchQuery *gtwthread.ThreadSearchRequest, identity auth.Identifier, systemCall bool) (*threadv1.ThreadSearchRequest, error) {
	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	return &threadv1.ThreadSearchRequest{
		Fields:       toInternalThreadFields(searchQuery.GetFields()),
		Ids:          searchQuery.GetIds(),
		DomainIds:    []int32{domainID},
		Q:            searchQuery.GetQ(),
		SelfId:       identity.GetContactID(),
		MemberIds:    searchQuery.GetContactIds(),
		Participants: participantsToInternal(searchQuery.GetParticipants()),
		Size:         searchQuery.GetSize(),
		Sort:         toInternalThreadSort(searchQuery.GetSort()),
		Page:         searchQuery.GetPage(),
		Kinds:        gtwThreadKindToInternal(searchQuery.GetTypes()),
		Unread:       searchQuery.GetUnread(),
		Tags:         searchQuery.GetTags(),
		ExcludeTags:  searchQuery.GetExcludeTags(),
		SystemCall:   &systemCall,
	}, nil
}

// newSearchLeftRequest builds the im-thread SearchLeft request for the caller. The gateway
// exposes the thread gate as "channel" (sort key "channel_name"); im-thread calls it
// "source" ("source_name"). "queue" / "queue_name" have the same name on both sides.
// It fails when the caller's domain id does not fit the int32 im-thread contract.
func newSearchLeftRequest(req *gtwthread.SearchLeftRequest, identity auth.Identifier) (*threadv1.SearchLeftRequest, error) {
	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	return &threadv1.SearchLeftRequest{
		Fields:   toInternalThreadFields(req.GetFields()),
		MemberId: identity.GetContactID(),
		DomainId: domainID,
		Kinds:    gtwThreadKindToInternal(req.GetTypes()),
		Size:     req.GetSize(),
		Sort:     toInternalThreadSort(req.GetSort()),
		Page:     req.GetPage(),
	}, nil
}

func (t *thread) SearchLeft(ctx context.Context, request *gtwthread.SearchLeftRequest) ([]*gtwthread.Thread, bool, error) {
	log := t.logger.With(slog.String("op", "thread.SearchLeft"))

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.ErrorContext(ctx, "identity not found")

		return nil, false, auth.IdentityNotFoundErr
	}

	if identity.GetIssuer() != WebitelIssuer {
		log.ErrorContext(ctx, "identity is not a Webitel issuer")

		return nil, false, errors.Wrap(auth.ForbiddenIssuerErr, errors.WithID("service.thread.issuer"))
	}

	internalReq, err := newSearchLeftRequest(request, identity)
	if err != nil {
		log.ErrorContext(ctx, "invalid search left request", slog.Any("error", err))

		return nil, false, err
	}

	threads, err := t.threadClient.SearchLeft(ctx, internalReq)
	if err != nil {
		log.Error("failed to fetch internal threads", slog.Any("error", err))

		return nil, false, err
	}

	uniqueContactIDs := t.collectUniqueContactsFromThread(threads.GetItems())

	contacts, err := t.fetchContacts(ctx, uniqueContactIDs, internalReq.GetDomainId())
	if err != nil {
		log.Error(
			"failed to fetch internal contact information for enrichment",
			slog.Any("error", err),
			slog.Any("ids", uniqueContactIDs),
		)

		return nil, false, err
	}

	res := make([]*gtwthread.Thread, 0, len(threads.GetItems()))
	for _, thr := range threads.GetItems() {
		res = append(res, t.withChannel(ctx, identity.GetDomainID(), thr, contacts))
	}

	return res, threads.GetNext(), nil
}

func (t *thread) Get(ctx context.Context, req *gtwthread.GetThreadRequest) (*gtwthread.Thread, error) {
	log := t.logger.With(slog.String("op", "thread.Get"))

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.ErrorContext(ctx, "identity not found")

		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	internalThread, err := t.threadClient.Get(ctx, &threadv1.GetThreadRequest{
		Id:       req.GetId(),
		DomainId: domainID,
		Fields:   toInternalThreadFields(req.GetFields()),
		CallerId: readCallerID(ctx, identity),
	})
	if err != nil {
		log.Error("failed to fetch internal thread", slog.Any("error", err))

		return nil, err
	}

	uniqueContactIDs := t.collectUniqueContactsFromThread([]*threadv1.Thread{internalThread})

	contacts, err := t.fetchContacts(ctx, uniqueContactIDs, domainID)
	if err != nil {
		log.Error("failed to fetch contact information for enrichment", slog.Any("error", err))

		return nil, err
	}

	thr := t.withChannel(ctx, identity.GetDomainID(), internalThread, contacts)
	thr.UpdatesCursor = internalThread.GetUpdatesCursor()

	return thr, nil
}

func (t *thread) SetVariables(ctx context.Context, req *gtwthread.SetVariablesRequest) (*gtwthread.ThreadVariables, error) {
	log := t.logger.With("operation", "service.thread.set_variables")

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.Warn("identity not found in context")

		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	response, err := t.threadClient.SetVariables(ctx, &threadv1.SetVariablesRequest{
		MemberId:  identity.GetContactID(),
		ThreadId:  req.GetThreadId(),
		Variables: convertToThreadProto(req.GetVariables()),
	})
	if err != nil {
		log.Error("internal service set variables", "err", err, "thread_id", req.GetThreadId(), "contact_id", identity.GetContactID())

		return nil, err
	}

	threadVars, err := t.convertToThreadVariables(ctx, response, domainID)
	if err != nil {
		log.Error("convert to thread variables", "err", err, "thread_id", req.GetThreadId(), "contact_id", identity.GetContactID())

		return nil, err
	}

	return threadVars, nil
}

func (t *thread) SearchVariables(ctx context.Context, req *gtwthread.SearchVariablesRequest) (*gtwthread.SearchVariablesResponse, error) {
	log := t.logger.With("operation", "service.thread.search_variables")

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.Warn("identity not found in context")

		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	response, err := t.threadClient.SearchVariables(ctx, &threadv1.SearchVariablesRequest{
		Size:      req.GetSize(),
		Page:      req.GetPage(),
		Fields:    req.GetFields(),
		ThreadIds: req.GetThreadIds(),
		CallerId:  readCallerID(ctx, identity),
		DomainId:  domainID,
	})
	if err != nil {
		log.Error("search variables", "err", err)

		return nil, err
	}

	var vars []*gtwthread.ThreadVariables

	for _, item := range response.GetItems() {
		v, err := t.convertToThreadVariables(ctx, item, domainID)
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
	log := t.logger.With("operation", "service.thread.locate_variables")

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.Warn("identity not found in context")

		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
	}

	response, err := t.threadClient.LocateVariables(ctx, &threadv1.LocateVariablesRequest{
		ThreadId: req.GetThreadId(),
		CallerId: readCallerID(ctx, identity),
		DomainId: domainID,
	})
	if err != nil {
		log.Error("locate variables", "err", err)

		return nil, err
	}

	v, err := t.convertToThreadVariables(ctx, response, domainID)
	if err != nil {
		log.Error("convert to thread variables", "err", err)

		return nil, err
	}

	return v, nil
}

func (t *thread) FlushVariables(ctx context.Context, req *gtwthread.FlushVariablesRequest) (*gtwthread.ThreadVariables, error) {
	log := t.logger.With("operation", "service.thread.flush_variables")

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.Warn("identity not found in context")

		return nil, auth.IdentityNotFoundErr
	}

	domainID, err := domainIDInt32(identity)
	if err != nil {
		return nil, err
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

	v, err := t.convertToThreadVariables(ctx, response, domainID)
	if err != nil {
		log.Error("convert to thread variables", "err", err)

		return nil, err
	}

	return v, nil
}

func (t *thread) collectUniqueContactsFromThread(threads []*threadv1.Thread) []string {
	uniqueMap := make(map[string]struct{})

	for _, thr := range threads {
		for _, m := range thr.GetMembers() {
			if m.GetContactId() == "" {
				continue
			}

			uniqueMap[m.GetContactId()] = struct{}{}
		}

		if thr.GetLastMsg() != nil {
			senderID := thr.GetLastMsg().GetSenderId()
			if senderID != "" {
				uniqueMap[senderID] = struct{}{}
			}

			for _, f := range thr.GetLastMsg().GetFailures() {
				uniqueMap[f.GetMemberId()] = struct{}{}
			}
		}

		for _, v := range thr.GetVariables().GetVariables() {
			setBy := v.GetSetBy()
			if setBy == "" {
				continue
			}

			uniqueMap[setBy] = struct{}{}
		}
	}

	return slices.Collect(maps.Keys(uniqueMap))
}

func convertToThread(thr *threadv1.Thread, contactData map[string]*contactv1.Contact) *gtwthread.Thread {
	var (
		members               = make([]*gtwthread.ThreadMember, 0, len(thr.GetMembers()))
		findLastMessageSender = thr.GetLastMsg() != nil
		lastMessageSender     *gtwthread.ThreadMember
		membersByContact      = make(map[string]*gtwthread.ThreadMember, len(thr.GetMembers()))
	)
	for _, toConvert := range thr.GetMembers() {
		correspondingContact, ok := contactData[toConvert.GetContactId()]
		if !ok {
			correspondingContact = &contactv1.Contact{
				Subject: toConvert.GetContactId(),
			}
		}

		member := convertToMember(toConvert, correspondingContact)
		members = append(members, member)
		membersByContact[toConvert.GetContactId()] = member

		if findLastMessageSender && toConvert.GetContactId() == thr.GetLastMsg().GetSenderId() {
			findLastMessageSender = false
			lastMessageSender = member
		}
	}

	var threadVars *gtwthread.ThreadVariables

	if thr.GetVariables() != nil {
		vars := make(map[string]*gtwthread.VariableEntry, len(thr.GetVariables().GetVariables()))
		for k, v := range thr.GetVariables().GetVariables() {
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

	lastMsg := convertToMessage(thr.GetLastMsg(), lastMessageSender)
	if lastMsg != nil {
		lastMsg.Failures = convertDeliveryFailures(thr.GetLastMsg().GetFailures(), membersByContact, contactData)
	}

	return &gtwthread.Thread{
		Id:          thr.GetId(),
		Subject:     thr.GetSubject(),
		Type:        gtwthread.ThreadKind(thr.GetKind()),
		CreatedAt:   thr.GetCreatedAt(),
		UpdatedAt:   thr.GetUpdatedAt(),
		Description: thr.GetDescription(),
		LastMsg:     lastMsg,
		Members:     members,
		Variables:   threadVars,
		UnreadCount: thr.GetUnreadCount(),

		// Per-member read horizons; client derives inbox/outbox watermarks from this.
		ReadStates:   convertThreadMemberReadStates(thr.GetReadStates()),
		CrmContactId: thr.GetCrmContactId(),
		Tags:         convertThreadTags(thr.GetTags()),
		Queue:        convertThreadQueue(thr.GetQueue()),
	}
}

// queueFromContext reads the optional call_center queue a member/preview call carries in the
// x-webitel-queue-id / x-webitel-queue-name-bin headers. No id header -> nil. An id that is not
// a positive integer (im-thread's "no queue") is logged and dropped: the queue is display-only,
// so it never fails the call.
func queueFromContext(ctx context.Context, logger *slog.Logger) *threadv1.ThreadQueue {
	md, _ := metadata.FromIncomingContext(ctx)

	ids := md.Get(auth.QueueIDIdentificationHeader)
	if len(ids) == 0 || ids[0] == "" {
		return nil
	}

	id, err := strconv.ParseInt(ids[0], 10, 64)
	if err != nil || id <= 0 {
		logger.WarnContext(ctx, "QUEUE_HEADER_INVALID", slog.String("queue_id", ids[0]))

		return nil
	}

	return &threadv1.ThreadQueue{Id: id, Name: auth.BinaryHeader(md, auth.QueueNameIdentificationHeader)}
}

// The gateway exposes the thread's gate as "channel"; im-thread calls it "source".
const (
	channelField         = "channel"
	channelNameSortKey   = "channel_name"
	internalSourceField  = "source"
	internalSourceSortBy = "source_name"
)

// toInternalThreadFields renames the gateway "channel" field to im-thread's "source",
// which carries the gate the channel is resolved from.
func toInternalThreadFields(fields []string) []string {
	if !slices.Contains(fields, channelField) {
		return fields
	}

	out := slices.Clone(fields)
	for i, f := range out {
		if f == channelField {
			out[i] = internalSourceField
		}
	}

	return out
}

// toInternalThreadSort maps the gateway "±channel_name" sort key to im-thread's "±source_name".
func toInternalThreadSort(sort string) string {
	if len(sort) > 1 && sort[1:] == channelNameSortKey {
		return sort[:1] + internalSourceSortBy
	}

	return sort
}

// convertThreadQueue maps the queue through which the caller received the thread; nil stays nil.
func convertThreadQueue(q *threadv1.ThreadQueue) *gtwthread.ThreadQueue {
	if q == nil {
		return nil
	}

	return &gtwthread.ThreadQueue{Id: q.GetId(), Name: q.GetName()}
}

func convertThreadTags(tags []*threadv1.ChatTag) []*gtwthread.ChatTag {
	if len(tags) == 0 {
		return nil
	}

	out := make([]*gtwthread.ChatTag, 0, len(tags))
	for _, tag := range tags {
		out = append(out, &gtwthread.ChatTag{
			Id:        tag.GetId(),
			ThreadId:  tag.GetThreadId(),
			Tag:       tag.GetTag(),
			CreatedAt: tag.GetCreatedAt(),
		})
	}

	return out
}

func convertThreadMemberReadStates(states []*threadv1.MemberReadState) []*gtwthread.MemberReadState {
	if len(states) == 0 {
		return nil
	}

	out := make([]*gtwthread.MemberReadState, 0, len(states))
	for _, st := range states {
		out = append(out, &gtwthread.MemberReadState{
			MemberId:         st.GetMemberId(),
			DeliveredUpToSeq: st.GetDeliveredUpToSeq(),
			ReadUpToSeq:      st.GetReadUpToSeq(),
		})
	}

	return out
}

func convertToMember(m *threadv1.ThreadMember, contact *contactv1.Contact) *gtwthread.ThreadMember {
	converted := &gtwthread.ThreadMember{
		Id:      m.GetId(),
		Contact: convertToContact(contact),
		Role:    gtwthread.ThreadRole(m.GetRole()),
	}
	if m.GetPermissions() != nil {
		converted.Permissions = &gtwthread.ThreadPermissions{
			CanSendMessages:             m.GetPermissions().GetCanSendMessages(),
			CanAddMembers:               m.GetPermissions().GetCanAddMembers(),
			CanRemoveMembers:            m.GetPermissions().GetCanRemoveMembers(),
			CanChangeMembersPermissions: m.GetPermissions().GetCanChangeMembersPermissions(),
			CanChangeThreadInfo:         m.GetPermissions().GetCanChangeThreadInfo(),
			CanDeleteMessages:           m.GetPermissions().GetCanDeleteMessages(),
		}
	}

	return converted
}

func convertToMessage(req *threadv1.HistoryMessage, sender *gtwthread.ThreadMember) *gtwthread.HistoryMessage {
	if req == nil {
		return nil
	}

	return &gtwthread.HistoryMessage{
		Id:          req.GetId(),
		ThreadId:    req.GetThreadId(),
		CreatedAt:   req.GetCreatedAt(),
		EditedAt:    req.GetUpdatedAt(),
		Type:        req.GetType(),
		Body:        req.GetBody(),
		Metadata:    req.GetMetadata(),
		Documents:   convertDocuments(req.GetDocuments()),
		Images:      convertImages(req.GetImages()),
		Contact:     imthread.MapContact(req.GetContact()),
		Location:    imthread.MapLocation(req.GetLocation()),
		Interactive: imthread.MapInteractive(req.GetInteractive()),
		System:      imthread.MapSystem(req.GetSystem()),
		Sender:      sender,
	}
}

// convertDeliveryFailures resolves each failed recipient to a thread member; one who has
// since left the thread keeps just the contactv1.
func convertDeliveryFailures(
	failures []*threadv1.DeliveryFailure,
	members map[string]*gtwthread.ThreadMember,
	contactData map[string]*contactv1.Contact,
) []*gtwthread.DeliveryFailure {
	if len(failures) == 0 {
		return nil
	}

	out := make([]*gtwthread.DeliveryFailure, 0, len(failures))
	for _, f := range failures {
		member, ok := members[f.GetMemberId()]
		if !ok {
			member = &gtwthread.ThreadMember{Contact: convertToContact(contactData[f.GetMemberId()])}
		}

		out = append(out, &gtwthread.DeliveryFailure{
			Member: member,
			Error:  &gtwthread.DeliveryError{Code: f.GetError().GetCode(), Message: f.GetError().GetMessage()},
		})
	}

	return out
}

func convertDocuments(reqDocs []*threadv1.Document) []*gtwthread.Document {
	if len(reqDocs) == 0 {
		return nil
	}

	docs := make([]*gtwthread.Document, len(reqDocs))
	for i, d := range reqDocs {
		docs[i] = &gtwthread.Document{
			Id:        d.GetFileId(),
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
			Key:   req.GetKey(),
			Value: req.GetValue(),
		}
		protoRequests[i] = protoReq
	}

	return protoRequests
}

func convertToContact(c *contactv1.Contact) *gtwthread.Contact {
	return &gtwthread.Contact{
		Id:       c.GetId(),
		Iss:      c.GetIssId(),
		Sub:      c.GetSubject(),
		Type:     c.GetType(),
		Name:     cmp.Or(c.GetName(), c.GetUsername(), NoNameRecipient),
		IsBot:    c.GetIsBot(),
		Username: c.GetUsername(),
		Phone:    c.GetPhone(),
	}
}

func (t *thread) convertToThreadVariables(ctx context.Context, response *threadv1.ThreadVariables, domainID int32) (*gtwthread.ThreadVariables, error) {
	log := t.logger.With("operation", "service.thread.convert_to_thread_variables")

	uniqueSettersSet := make(map[string]struct{})
	for _, v := range response.GetVariables() {
		uniqueSettersSet[v.GetSetBy()] = struct{}{}
	}

	uniqueSettersIDs := slices.Collect(maps.Keys(uniqueSettersSet))

	contacts, err := t.contactClient.SearchContact(ctx, &contactv1.SearchContactRequest{
		Size:     sizeInt32(len(uniqueSettersIDs)),
		Fields:   []string{"id", "issuer_id", "type", "subject_id", "username", "name", "is_bot"},
		DomainId: domainID,
		Ids:      uniqueSettersIDs,
	})
	if err != nil {
		log.ErrorContext(ctx, "search variables setters contacts", "err", err)

		return nil, err
	}

	externalSettersMap := make(map[string]*gtwthread.Contact, len(contacts.GetContacts()))
	for _, c := range contacts.GetContacts() {
		externalSettersMap[c.GetId()] = &gtwthread.Contact{
			Iss:   c.GetIssId(),
			Sub:   c.GetSubject(),
			Type:  c.GetType(),
			Name:  cmp.Or(c.GetName(), c.GetUsername(), NoNameRecipient),
			IsBot: c.GetIsBot(),
		}
	}

	vars := make(map[string]*gtwthread.VariableEntry, len(response.GetVariables()))
	for k, v := range response.GetVariables() {
		setBy, ok := externalSettersMap[v.GetSetBy()]
		if !ok {
			log.Warn("not found external setter", "internal_id", v.GetSetBy())
		}

		vars[k] = &gtwthread.VariableEntry{
			Value: v.GetValue(),
			SetBy: setBy,
			SetAt: v.GetSetAt(),
		}
	}

	return &gtwthread.ThreadVariables{
		ThreadId:  response.GetThreadId(),
		Variables: vars,
	}, nil
}
