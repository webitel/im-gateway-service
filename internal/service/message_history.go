package service

import (
	"context"
	"log/slog"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
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
//  - *dto.SearchMessageHistoryResponse: search result
//  - error: any error encountered during the search operation
func (s *messageHistory) Search(ctx context.Context, searchQuery *dto.SearchMessageHistoryRequest) (*dto.SearchMessageHistoryResponse, error)  {
	log := s.logger.With(
		slog.String("op", "messageHistory.Search"),
		slog.Int("domain_id", int(searchQuery.DomainID)),
	)
	
	response, fromInternal, err := s.historyClient.Search(ctx, searchQuery)
	if err != nil {
		log.Error("failed to fetch message history from provider",
			slog.Any("err", err),
		)
		return nil, err
	}

	externalParticipants, err := s.contactClient.SearchContact(ctx, &contact.SearchContactRequest{
		Fields: []string{"issuer_id", "type", "subject_id"},
		DomainId: searchQuery.DomainID,
		Size: int32(len(fromInternal)),
		Ids: fromInternal,
	})	

	if err != nil {
		log.Error("failed to fetch participants external information",
			slog.Any("error", err),
		)
		return nil, err
	}

	response.MessageSenders = mapContactListToMessageSenderList(externalParticipants.GetContacts())

	return response, nil
}

// mapContactListToMessageSenderList maps a list of contacts to a list of message senders.
//
// Args:
//  - contacts: A list of contacts to be mapped.
//
// Returns:
//  - A list of message senders with the given subjects, issuers, and types.
func mapContactListToMessageSenderList(contacts []*contact.Contact) []*dto.MessageSender {
	var (
		messageSenderList = make([]*dto.MessageSender, 0, len(contacts))
	)

	for _, contact := range contacts {
		messageSenderList = append(messageSenderList, mapContactToMessageSender(contact))
	}

	return messageSenderList
}

// mapContactToMessageSender maps a contact to a message sender.
//
// Args:
//  - contact: The contact to be mapped.
//
// Returns:
//  - A message sender with the given subject, issuer, and type.
func mapContactToMessageSender(contact *contact.Contact) *dto.MessageSender {
	return &dto.MessageSender{
		Subject: contact.GetSubject(),
		Issuer:  contact.GetIssId(),
		Type:    contact.GetType(),
	}
}