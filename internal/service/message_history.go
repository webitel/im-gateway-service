package service

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"

	"github.com/webitel/im-gateway-service/internal/service/dto"
)

type (
	MessageHistorySearcher interface {
		Search(ctx context.Context, searchQuery *dto.SearchMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, error) 
	}

	messageHistory struct {
		logger *slog.Logger
		historyClient *imthread.MessageHistoryClient
		contactClient *imcontact.Client
	}
)

// NewMessageHistory returns a new instance of MessageHistorySearcher.
//
// Args:
//  - logger: logger for the service
//  - historyClient: client for the Message History service
//	- contactClient: client for the Contact service
//
// Returns:
//  - A new instance of MessageHistorySearcher
func NewMessageHistory(logger *slog.Logger, historyClient *imthread.MessageHistoryClient, contactClient *imcontact.Client) *messageHistory {
	return &messageHistory{
		logger:        logger,
		historyClient: historyClient,
		contactClient: contactClient,
	}
}

// Search performs a search for messages in the message history given a search query.
//
// Args:
//  - ctx: context of the request
//  - searchQuery: search query for the message history
//
// Returns:
//  - response: search result
//  - error: any error encountered during the search operation
func (s *messageHistory) Search(ctx context.Context, searchQuery *dto.SearchMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, error)  {
	log := s.logger.With(
		slog.String("op", "messageHistory.Search"),
		slog.Any("threads", searchQuery.ThreadIDs),
	)

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.ErrorContext(ctx, "identity not found")
		return nil, auth.IdentityNotFoundErr
	}
	searchQuery.DomainID = int32(identity.GetDomainID())

	response, fromInternal, err := s.historyClient.Search(ctx, searchQuery)
    if err != nil {
        log.Error("failed to fetch message history", slog.Any("err", err))
        return nil, err
    }

	participantIDs := s.collectUniqueIDs(response.Messages, fromInternal)

	identityMap, err := s.fetchParticipantMap(ctx, searchQuery.DomainID, participantIDs)
    if err != nil {
        log.Error("failed to fetch participants info", slog.Any("err", err))
        return nil, err
    }

	s.enrichResponse(response, fromInternal, identityMap)

	return response, nil
}

// collectUniqueIDs takes a slice of messages and a slice of internalIDs and returns a slice of unique IDs.
// It collects all the IDs from the messages and internalIDs and returns a slice of unique IDs.
// The returned slice will contain all the IDs from the messages and internalIDs, without any duplicates.
func (s *messageHistory) collectUniqueIDs(messages []*dto.HistoryMessage, internalIDs []string) []string {
	uniqueMap := make(map[string]struct{})
	for _, id := range internalIDs {
		uniqueMap[id] = struct{}{}
	}

	for _, m := range messages {
		if m.SenderID != "" {uniqueMap[m.SenderID] = struct{}{}}
	}

	return slices.Collect(maps.Keys(uniqueMap))
}

// fetchParticipantMap fetches the participant map for the given domain ID and IDs.
// It returns a map of IDs to MessageSender objects from the imap.
// If there are no IDs provided, it returns an empty map and no error.
// If there is an error while fetching the participants, it returns an error.
func (s *messageHistory) fetchParticipantMap(ctx context.Context, domainID int32, ids []string) (map[string]*dto.MessageSender, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	external, err := s.contactClient.SearchContact(ctx, &contact.SearchContactRequest{
        Fields:   []string{"id", "issuer_id", "type", "subject_id", "username"},
        DomainId: domainID,
        Size:     int32(len(ids)),
        Ids:      ids,
    })
    if err != nil {
        return nil, err
    }

	res := make(map[string]*dto.MessageSender, len(external.GetContacts()))
    for _, p := range external.GetContacts() {
        res[p.Id] = dto.NewMessageSender(p.GetSubject(), p.GetIssId(), p.GetType(), p.GetUsername())
    }
    return res, nil
}

// enrichResponse enriches the search message history response by replacing the receiver and sender IDs
// with the corresponding message sender objects from the imap.
func (s *messageHistory) enrichResponse(resp *dto.SearchMessageHistoryResponse, internal []string, imap map[string]*dto.MessageSender) {
	resp.MessageSenders = make([]*dto.MessageSender, 0, len(internal))
    for _, id := range internal {
        if ms, ok := imap[id]; ok {
            resp.MessageSenders = append(resp.MessageSenders, ms)
        }
    }

    for _, m := range resp.Messages {
        m.Sender = imap[m.SenderID]
    }
}