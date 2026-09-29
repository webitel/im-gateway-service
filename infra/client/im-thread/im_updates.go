package imthread

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"

	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"

	api "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// UpdatesClient calls im-thread-service Updates.
type UpdatesClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[threadv1.UpdatesClient]
}

func NewUpdatesClient(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*UpdatesClient, error) {
	log := logger.With(slog.String("component", "im-updates-client"))

	factory := func(conn *grpc.ClientConn) threadv1.UpdatesClient {
		return threadv1.NewUpdatesClient(conn)
	}

	c, err := webitel.New(logger, discovery, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[im-updates-client] initialization failed: %w", err)
	}

	return &UpdatesClient{logger: log, rpc: c}, nil
}

// GetUpdates fetches every thread changed for the caller since the cursor.
func (c *UpdatesClient) GetUpdates(ctx context.Context, query *dto.GetUpdatesRequest) (*dto.GetUpdatesResponse, error) {
	req := &threadv1.GetUpdatesRequest{
		CallerId:               query.CallerID,
		DomainId:               query.DomainID,
		Cursor:                 query.Cursor,
		SystemMessageAllowList: toSystemMessageAllowList(query.SystemMessageAllowList),
	}

	var (
		response *threadv1.GetUpdatesResponse
		err      error
	)

	err = c.rpc.Execute(ctx, func(uc threadv1.UpdatesClient) error {
		response, err = uc.GetUpdates(ctx, req)

		return err
	})
	if err != nil {
		c.logger.Error("failed to fetch updates", slog.Int("domain_id", int(query.DomainID)), slog.Any("error", err))

		return nil, err
	}

	return ToUpdatesResponseDTO(response), nil
}

func (c *UpdatesClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}

	return nil
}

func ToUpdatesResponseDTO(resp *threadv1.GetUpdatesResponse) *dto.GetUpdatesResponse {
	out := &dto.GetUpdatesResponse{Cursor: resp.GetCursor(), Resync: resp.GetResync()}

	for _, t := range resp.GetThreads() {
		out.Threads = append(out.Threads, toThreadUpdatesDTO(t))
	}

	return out
}

func toThreadUpdatesDTO(t *threadv1.ThreadUpdates) *dto.ThreadUpdates {
	messages := make([]*threadv1.HistoryMessage, 0, len(t.GetMessages()))
	for _, m := range t.GetMessages() {
		messages = append(messages, historyFromUpdated(m))
	}

	changes := make([]*dto.ThreadMemberChange, 0, len(t.GetMemberChanges()))
	for _, mc := range t.GetMemberChanges() {
		changes = append(changes, &dto.ThreadMemberChange{
			ContactID: mc.GetContactId(),
			Action:    int32(mapMemberChangeAction(mc.GetAction())),
			ByID:      mc.GetById(),
		})
	}

	out := &dto.ThreadUpdates{
		ThreadID:          t.GetThreadId(),
		Left:              t.GetLeft(),
		UnreadCount:       t.GetUnreadCount(),
		RawDialog:         t.GetDialog(),
		Messages:          mapMessages(messages),
		DeletedMessageIDs: t.GetDeletedMessageIds(),
		MemberChanges:     changes,
		ReadStates:        mapMemberReadStates(t.GetReadStates()),
		From:              t.GetMembers(),
	}

	if t.GetTopMessage() != nil {
		out.TopMessage = mapMessages([]*threadv1.HistoryMessage{historyFromUpdated(t.GetTopMessage())})[0]
	}

	return out
}

// historyFromUpdated lets updates reuse history's mapping; updated_at carries edited_at.
func historyFromUpdated(u *threadv1.UpdatedMessage) *threadv1.HistoryMessage {
	return &threadv1.HistoryMessage{
		Id:            u.GetId(),
		Seq:           u.GetSeq(),
		SenderId:      u.GetSenderId(),
		Type:          u.GetType(),
		Body:          u.GetBody(),
		Metadata:      u.GetMetadata(),
		CreatedAt:     u.GetCreatedAt(),
		UpdatedAt:     u.GetEditedAt(),
		ReplyTo:       u.GetReplyTo(),
		ForwardOrigin: u.GetForwardOrigin(),
		Documents:     u.GetDocuments(),
		Images:        u.GetImages(),
		Location:      u.GetLocation(),
		Contact:       u.GetContact(),
		Interactive:   u.GetInteractive(),
		System:        u.GetSystem(),
		Reactions:     u.GetReactions(),
	}
}

func mapMemberReadStates(states []*threadv1.MemberReadState) []*dto.MemberReadState {
	if len(states) == 0 {
		return nil
	}

	res := make([]*dto.MemberReadState, 0, len(states))
	for _, s := range states {
		res = append(res, &dto.MemberReadState{
			MemberID:         s.GetMemberId(),
			DeliveredUpToSeq: s.GetDeliveredUpToSeq(),
			ReadUpToSeq:      s.GetReadUpToSeq(),
		})
	}

	return res
}

func mapMemberChangeAction(a threadv1.ThreadMemberChangeAction) api.ThreadMemberChangeAction {
	switch a {
	case threadv1.ThreadMemberChangeAction_THREAD_MEMBER_CHANGE_ACTION_JOINED:
		return api.ThreadMemberChangeAction_THREAD_MEMBER_CHANGE_ACTION_JOINED
	case threadv1.ThreadMemberChangeAction_THREAD_MEMBER_CHANGE_ACTION_LEFT:
		return api.ThreadMemberChangeAction_THREAD_MEMBER_CHANGE_ACTION_LEFT
	case threadv1.ThreadMemberChangeAction_THREAD_MEMBER_CHANGE_ACTION_UNSPECIFIED:
	}

	return api.ThreadMemberChangeAction_THREAD_MEMBER_CHANGE_ACTION_UNSPECIFIED
}
