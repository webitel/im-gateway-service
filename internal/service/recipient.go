package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	impb "github.com/webitel/im-gateway-service/gen/go/contact/v1"
)

type recipientContacts interface {
	SearchContact(ctx context.Context, req *impb.SearchContactRequest) (*impb.ContactList, error)
	CreateContact(ctx context.Context, req *impb.CreateContactRequest) (*impb.Contact, error)
}

type recipientVias interface {
	Create(ctx context.Context, req *impb.CreateViaRequest) (*impb.Via, error)
}

// implicitContactIssuers lists issuers whose subject is a phone number the
// platform can message first, so an unknown recipient is created on send
// instead of rejected. Each normalizer must match the one the provider applies
// to inbound senders, or the reply lands on a duplicate contact.
var implicitContactIssuers = map[string]func(string) string{
	"viber_bm": normalizeMSISDN,
}

// normalizeMSISDN mirrors im-providers-service: Infobip reports senders as a
// bare international number without '+'.
func normalizeMSISDN(s string) string {
	return strings.TrimPrefix(strings.Join(strings.Fields(s), ""), "+")
}

func (m *MessageService) searchRecipient(ctx context.Context, sub, iss string, domainID int32) ([]*impb.Contact, error) {
	res, err := m.contacter.SearchContact(ctx, &impb.SearchContactRequest{
		Subjects: []string{sub},
		IssId:    []string{iss},
		DomainId: domainID,
		Size:     2,
		Page:     1,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve recipient: %w", err)
	}

	return res.GetContacts(), nil
}

// createRecipient creates the contact with the same fields the provider uses
// on inbound and links it to the gate. A concurrent inbound may win the race,
// so AlreadyExists falls back to a search.
func (m *MessageService) createRecipient(ctx context.Context, sub, iss, via string, domainID int32) ([]*impb.Contact, error) {
	contact, err := m.contacter.CreateContact(ctx, &impb.CreateContactRequest{
		IssId:    iss,
		Type:     iss,
		Name:     sub,
		Username: sub,
		Subject:  sub,
		DomainId: domainID,
	})
	if status.Code(err) == codes.AlreadyExists {
		return m.searchRecipient(ctx, sub, iss, domainID)
	}

	if err != nil {
		return nil, fmt.Errorf("create recipient: %w", err)
	}

	m.logger.InfoContext(ctx, "recipient contact created implicitly",
		slog.String("iss", iss),
		slog.String("contact_id", contact.GetId()),
		slog.String("via", via),
	)

	if _, err := m.vias.Create(ctx, &impb.CreateViaRequest{ContactId: contact.GetId(), Via: via}); err != nil &&
		status.Code(err) != codes.AlreadyExists {
		// Delivery routes by the request's via, so a missing link only affects
		// contact metadata and must not fail the send.
		m.logger.WarnContext(ctx, "link recipient via failed",
			slog.String("contact_id", contact.GetId()),
			slog.String("via", via),
			slog.Any("err", err),
		)
	}

	return []*impb.Contact{contact}, nil
}
