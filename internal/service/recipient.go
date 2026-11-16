package service

import (
	"context"
	"fmt"
	"strings"

	impb "github.com/webitel/im-gateway-service/gen/go/contact/v1"
)

type recipientContacts interface {
	SearchContact(ctx context.Context, req *impb.SearchContactRequest) (*impb.ContactList, error)
}

// recipientSubjectNormalizers maps an issuer to the normalizer its provider
// applies to inbound senders, so a recipient typed in another format still
// resolves to the existing contact. Lookup only: an unknown recipient is never
// created on send.
var recipientSubjectNormalizers = map[string]func(string) string{
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
