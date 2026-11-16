package service

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"google.golang.org/grpc/codes"

	kiterrors "github.com/webitel/webitel-go-kit/pkg/errors"

	impb "github.com/webitel/im-gateway-service/gen/go/contact/v1"
)

type fakeRecipientContacts struct {
	stored    []*impb.Contact
	searchErr error
	searches  []*impb.SearchContactRequest
}

func (f *fakeRecipientContacts) SearchContact(_ context.Context, req *impb.SearchContactRequest) (*impb.ContactList, error) {
	f.searches = append(f.searches, req)
	if f.searchErr != nil {
		return nil, f.searchErr
	}

	var out []*impb.Contact

	for _, c := range f.stored {
		if slices.Contains(req.GetSubjects(), c.GetSubject()) && slices.Contains(req.GetIssId(), c.GetIssId()) {
			out = append(out, c)
		}
	}

	return &impb.ContactList{Contacts: out}, nil
}

func ptr[T any](v T) *T { return &v }

func TestResolveContact(t *testing.T) {
	const gate = "00000000-0000-0000-0000-000000000001"

	viberContact := &impb.Contact{Id: "existing", IssId: "viber_bm", Subject: "380000000001", Name: "Test Recipient"}
	viberDuplicate := &impb.Contact{Id: "duplicate", IssId: "viber_bm", Subject: "380000000001"}
	fbContact := &impb.Contact{Id: "fb", IssId: "facebook", Subject: "+123"}

	tests := []struct {
		name      string
		stored    []*impb.Contact
		searchErr error
		sub, iss  string
		via       *string
		wantID    string
		wantCode  codes.Code
		wantSub   string
	}{
		{name: "viber_bm existing, plus prefix normalized", stored: []*impb.Contact{viberContact}, sub: "+380000000001", iss: "viber_bm", via: ptr(gate), wantID: "existing", wantSub: "380000000001"},
		{name: "viber_bm existing, spaces normalized", stored: []*impb.Contact{viberContact}, sub: " +380 00 000 0001 ", iss: "viber_bm", via: ptr(gate), wantID: "existing", wantSub: "380000000001"},
		{name: "viber_bm unknown with via is not created", sub: "+380000000001", iss: "viber_bm", via: ptr(gate), wantCode: codes.NotFound, wantSub: "380000000001"},
		{name: "viber_bm unknown without via is not created", sub: "380000000001", iss: "viber_bm", wantCode: codes.NotFound, wantSub: "380000000001"},
		{name: "viber_bm ambiguous recipient is rejected", stored: []*impb.Contact{viberContact, viberDuplicate}, sub: "380000000001", iss: "viber_bm", via: ptr(gate), wantCode: codes.NotFound, wantSub: "380000000001"},
		{name: "other issuer unknown stays not found", sub: "+380000000001", iss: "facebook", via: ptr(gate), wantCode: codes.NotFound, wantSub: "+380000000001"},
		{name: "other issuer sub is not normalized", stored: []*impb.Contact{fbContact}, sub: "+123", iss: "facebook", via: ptr(gate), wantID: "fb", wantSub: "+123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contacts := &fakeRecipientContacts{stored: tt.stored, searchErr: tt.searchErr}
			m := &MessageService{logger: slog.New(slog.DiscardHandler), contacter: contacts}

			peer, err := m.resolveContact(context.Background(), tt.sub, tt.iss, tt.via, 1)
			if tt.wantCode != codes.OK {
				if code, _ := kiterrors.Value(err, kiterrors.ErrKeyCode).(codes.Code); err == nil || code != tt.wantCode {
					t.Fatalf("err = %v, want code %v", err, tt.wantCode)
				}
			} else if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if tt.wantCode == codes.OK && peer.GetContactId() != tt.wantID {
				t.Errorf("contact id = %q, want %q", peer.GetContactId(), tt.wantID)
			}

			if len(contacts.searches) != 1 {
				t.Fatalf("searches = %d, want 1", len(contacts.searches))
			}

			search := contacts.searches[0]
			if !slices.Equal(search.GetSubjects(), []string{tt.wantSub}) || !slices.Equal(search.GetIssId(), []string{tt.iss}) || search.GetDomainId() != 1 {
				t.Errorf("unexpected search request: %v", search)
			}
		})
	}
}

func TestResolveContactSearchError(t *testing.T) {
	boom := errors.New("boom")
	m := &MessageService{logger: slog.New(slog.DiscardHandler), contacter: &fakeRecipientContacts{searchErr: boom}}

	if _, err := m.resolveContact(context.Background(), "380000000001", "viber_bm", ptr("gate"), 1); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
}
