package service

import (
	"cmp"
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	api "github.com/webitel/im-gateway-service/gen/go/gateway/v1"

	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"

	"github.com/webitel/im-gateway-service/internal/service/dto"
)

type (
	MessageHistorySearcher interface {
		Search(ctx context.Context, searchQuery *dto.SearchMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, error)
		SearchLeftThreads(ctx context.Context, query *dto.SearchLeftThreadsMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, error)
	}

	messageHistory struct {
		logger        *slog.Logger
		historyClient *imthread.MessageHistoryClient
		contactClient *imcontact.Client
	}
)

// NewMessageHistory returns a new instance of MessageHistorySearcher.
//
// Args:
//   - logger: logger for the service
//   - historyClient: client for the Message History service
//   - contactClient: client for the Contact service
//
// Returns:
//   - A new instance of MessageHistorySearcher
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
//   - ctx: context of the request
//   - searchQuery: search query for the message history
//
// Returns:
//   - response: search result
//   - error: any error encountered during the search operation
func (s *messageHistory) Search(ctx context.Context, searchQuery *dto.SearchMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, error) {
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
	searchQuery.CallerID = identity.GetContactID()

	response, fromInternal, err := s.historyClient.Search(ctx, searchQuery)
	if err != nil {
		log.Error("failed to fetch message history", slog.Any("err", err))
		return nil, err
	}

	reactedBy := make([]string, 0)
	for _, m := range response.Messages {
		if m.ReactedMetadata != nil {
			reactedBy = append(reactedBy, m.ReactedMetadata.ContactID)
		}
	}

	identityMap, err := s.fetchParticipantMap(ctx, searchQuery.DomainID, fromInternal, reactedBy...)
	if err != nil {
		log.Error("failed to fetch participants info", slog.Any("err", err))
		return nil, err
	}

	s.enrichResponse(response, fromInternal, identityMap)

	return response, nil
}

// SearchLeftThreads performs a search for messages covering the user's closed
// membership periods within a thread.
//
// Args:
//   - ctx: context of the request
//   - query: search query for the left-threads message history
//
// Returns:
//   - response: flat search result with sender enrichment
//   - error: any error encountered during the search operation
func (s *messageHistory) SearchLeftThreads(ctx context.Context, query *dto.SearchLeftThreadsMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, error) {
	log := s.logger.With(
		slog.String("op", "messageHistory.SearchLeftThreads"),
		slog.String("thread", query.ThreadID),
	)

	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		log.ErrorContext(ctx, "identity not found")
		return nil, auth.IdentityNotFoundErr
	}

	query.DomainID = int32(identity.GetDomainID())

	response, fromInternal, err := s.historyClient.SearchLeftThreads(ctx, query)
	if err != nil {
		log.Error("failed to fetch left threads message history", slog.Any("err", err))
		return nil, err
	}

	identityMap, err := s.fetchParticipantMap(ctx, query.DomainID, fromInternal)
	if err != nil {
		log.Error("failed to fetch participants info", slog.Any("err", err))
		return nil, err
	}

	s.enrichResponse(response, fromInternal, identityMap)

	return response, nil
}

// fetchParticipantMap fetches the participant map for the given domain ID and IDs.
// It returns a map of IDs to MessageSender objects from the imap.
// If there are no IDs provided, it returns an empty map and no error.
// If there is an error while fetching the participants, it returns an error.
func (s *messageHistory) fetchParticipantMap(ctx context.Context, domainID int32, internal []*threadv1.ThreadMember, internalContactIDs ...string) (map[string]*dto.MessageSender, error) {
	if len(internal) == 0 {
		return nil, nil
	}

	uniqunesMap := make(map[string]*threadv1.ThreadMember)
	for _, member := range internal {
		uniqunesMap[member.GetContactId()] = member
	}
	ids := slices.Collect(maps.Keys(uniqunesMap))
	concatad := slices.Concat(ids, internalContactIDs)

	external, err := s.contactClient.SearchContact(ctx, &contact.SearchContactRequest{
		Fields:   []string{"id", "issuer_id", "type", "subject_id", "username", "name", "is_bot"},
		DomainId: domainID,
		Size:     int32(len(concatad)),
		Ids:      concatad,
	})
	if err != nil {
		return nil, err
	}

	res := make(map[string]*dto.MessageSender, len(external.GetContacts()))
	for _, p := range external.GetContacts() {
		if mem, ok := uniqunesMap[p.GetId()]; ok || slices.Contains(internalContactIDs, p.GetId()) {
			res[p.Id] = &dto.MessageSender{
				Sub:      p.GetSubject(),
				Iss:      p.GetIssId(),
				Type:     p.GetType(),
				Name:     cmp.Or(p.GetName(), p.GetUsername()),
				IsBot:    p.GetIsBot(),
				MemberID: mem.GetId(),
				Role:     int(mem.GetRole()),
				Username: p.GetUsername(),
			}
		}
	}
	return res, nil
}

// enrichResponse enriches the search message history response by replacing the receiver and sender IDs
// with the corresponding message sender objects from the imap.
func (s *messageHistory) enrichResponse(resp *dto.SearchMessageHistoryResponse, _ []*threadv1.ThreadMember, imap map[string]*dto.MessageSender) {
	for _, m := range resp.Messages {
		m.Sender = imap[m.SenderID]

		if m.ReactedMetadata != nil && imap[m.ReactedMetadata.ContactID] != nil {
			c := imap[m.ReactedMetadata.ContactID]
			m.ReactedMetadata.ReactedBy = &api.ThreadMember{
				Contact: &api.Contact{
					Iss:      c.Iss,
					Type:     c.Type,
					Name:     c.Name,
					Username: c.Username,
					Sub:      c.Sub,
					IsBot:    c.IsBot,
				},
				Role:        0,
				Permissions: nil,
			}
		}
	}
}
