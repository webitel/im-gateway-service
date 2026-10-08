package service

import (
	"context"
	"log/slog"
	"slices"

	"github.com/webitel/im-gateway-service/gen/go/contact/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	imcontact "github.com/webitel/im-gateway-service/infra/client/im-contact"
	imthread "github.com/webitel/im-gateway-service/infra/client/im-thread"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

type (
	UpdatesFetcher interface {
		GetUpdates(ctx context.Context, query *dto.GetUpdatesRequest) (*dto.GetUpdatesResponse, error)
	}

	updatesClient interface {
		GetUpdates(ctx context.Context, query *dto.GetUpdatesRequest) (*dto.GetUpdatesResponse, error)
	}

	contactSearcher interface {
		SearchContact(ctx context.Context, req *contact.SearchContactRequest) (*contact.ContactList, error)
	}

	updates struct {
		logger    *slog.Logger
		client    updatesClient
		contacts  contactSearcher
		appConfig AppConfigProvider
	}
)

func NewUpdates(logger *slog.Logger, client *imthread.UpdatesClient, contactClient *imcontact.Client, appConfig AppConfigProvider) *updates {
	return &updates{logger: logger, client: client, contacts: contactClient, appConfig: appConfig}
}

// GetUpdates returns every thread changed for the authenticated caller, with contacts
// resolved in history's shape and new dialogs in thread-list shape.
func (s *updates) GetUpdates(ctx context.Context, query *dto.GetUpdatesRequest) (*dto.GetUpdatesResponse, error) {
	identity, ok := auth.GetIdentityFromContext(ctx)
	if !ok {
		return nil, auth.IdentityNotFoundErr
	}

	query.DomainID = int32(identity.GetDomainID())
	query.CallerID = identity.GetContactID()
	query.SystemMessageAllowList = s.appConfig.ResolvePolicy(ctx, identity.GetDomainID(), identity.GetApplicationID()).ToDTO()

	resp, err := s.client.GetUpdates(ctx, query)
	if err != nil {
		return nil, err
	}

	if err := s.enrich(ctx, query.DomainID, resp.Threads); err != nil {
		s.logger.Error("failed to resolve update contacts", slog.Any("err", err))

		return nil, err
	}

	return resp, nil
}

// enrich resolves every contact the threads reference with one contact search.
func (s *updates) enrich(ctx context.Context, domainID int32, threads []*dto.ThreadUpdates) error {
	ids := updatesContactIDs(threads)
	if len(ids) == 0 {
		return nil
	}

	found, err := s.contacts.SearchContact(ctx, &contact.SearchContactRequest{
		DomainId: domainID,
		Size:     int32(len(ids)),
		Ids:      ids,
	})
	if err != nil {
		return err
	}

	byID := make(map[string]*contact.Contact, len(found.GetContacts()))
	for _, c := range found.GetContacts() {
		byID[c.GetId()] = c
	}

	for _, t := range threads {
		imap := senderMap(found.GetContacts(), t.From)

		messages := t.Messages
		if t.TopMessage != nil {
			messages = append(slices.Clip(messages), t.TopMessage)
		}

		enrichMessages(messages, imap)

		for _, mc := range t.MemberChanges {
			mc.Member = imap[mc.ContactID]
			mc.By = imap[mc.ByID]
		}

		for _, rs := range t.ReadStates {
			rs.Member = imap[rs.MemberID]
		}

		if t.RawDialog != nil {
			t.Dialog = convertToThread(t.RawDialog, byID)
		}
	}

	return nil
}

func updatesContactIDs(threads []*dto.ThreadUpdates) []string {
	seen := make(map[string]struct{})
	add := func(ids ...string) {
		for _, id := range ids {
			if id != "" {
				seen[id] = struct{}{}
			}
		}
	}

	for _, t := range threads {
		for _, m := range t.From {
			add(m.GetContactId())
		}

		for _, m := range append(slices.Clip(t.Messages), t.TopMessage) {
			if m != nil {
				add(m.SenderID)
				add(historyExtraContactIDs([]*dto.HistoryMessage{m})...)
			}
		}

		for _, mc := range t.MemberChanges {
			add(mc.ContactID, mc.ByID)
		}

		for _, rs := range t.ReadStates {
			add(rs.MemberID)
		}

		if t.RawDialog != nil {
			add(new(thread).collectUniqueContactsFromThread([]*threadv1.Thread{t.RawDialog})...)
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	return ids
}
