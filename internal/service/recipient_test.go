package service

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	impb "github.com/webitel/im-gateway-service/gen/go/contact/v1"
)

type fakeRecipientContacts struct {
	stored    []*impb.Contact
	createErr error
	searches  []*impb.SearchContactRequest
	creates   []*impb.CreateContactRequest
}

func (f *fakeRecipientContacts) SearchContact(_ context.Context, req *impb.SearchContactRequest) (*impb.ContactList, error) {
	f.searches = append(f.searches, req)

	var out []*impb.Contact

	for _, c := range f.stored {
		if slices.Contains(req.GetSubjects(), c.GetSubject()) && slices.Contains(req.GetIssId(), c.GetIssId()) {
			out = append(out, c)
		}
	}

	return &impb.ContactList{Contacts: out}, nil
}

func (f *fakeRecipientContacts) CreateContact(_ context.Context, req *impb.CreateContactRequest) (*impb.Contact, error) {
	f.creates = append(f.creates, req)
	if f.createErr != nil {
		return nil, f.createErr
	}

	return &impb.Contact{Id: "new", IssId: req.GetIssId(), Subject: req.GetSubject(), Name: req.GetName()}, nil
}

type fakeRecipientVias struct {
	err     error
	created []*impb.CreateViaRequest
}

func (f *fakeRecipientVias) Create(_ context.Context, req *impb.CreateViaRequest) (*impb.Via, error) {
	f.created = append(f.created, req)

	return &impb.Via{}, f.err
}

func ptr[T any](v T) *T { return &v }

func TestResolveContact(t *testing.T) {
	const gate = "01a158cf-a2e4-7640-bc76-d6077c51d246"

	viberContact := &impb.Contact{Id: "existing", IssId: "viber_bm", Subject: "380937415826", Name: "Volodia"}
	fbContact := &impb.Contact{Id: "fb", IssId: "facebook", Subject: "+123"}

	tests := []struct {
		name        string
		stored      []*impb.Contact
		createErr   error
		viaErr      error
		sub, iss    string
		via         *string
		wantID      string
		wantErr     bool
		wantCreates int
		wantVias    int
	}{
		{name: "viber_bm existing, plus prefix normalized", stored: []*impb.Contact{viberContact}, sub: "+380937415826", iss: "viber_bm", via: ptr(gate), wantID: "existing"},
		{name: "viber_bm existing, spaces normalized", stored: []*impb.Contact{viberContact}, sub: " +380 93 741 5826 ", iss: "viber_bm", via: ptr(gate), wantID: "existing"},
		{name: "viber_bm unknown is created and linked", sub: "+380937415826", iss: "viber_bm", via: ptr(gate), wantID: "new", wantCreates: 1, wantVias: 1},
		{name: "viber_bm create race falls back to search", stored: []*impb.Contact{viberContact}, createErr: status.Error(codes.AlreadyExists, "exists"), sub: "380937415826", iss: "viber_bm", via: ptr(gate), wantID: "existing"},
		{name: "viber_bm via failure does not fail send", viaErr: errors.New("boom"), sub: "380937415826", iss: "viber_bm", via: ptr(gate), wantID: "new", wantCreates: 1, wantVias: 1},
		{name: "viber_bm create failure", createErr: status.Error(codes.Internal, "boom"), sub: "380937415826", iss: "viber_bm", via: ptr(gate), wantErr: true, wantCreates: 1},
		{name: "viber_bm without via is not created", sub: "380937415826", iss: "viber_bm", wantErr: true},
		{name: "viber_bm with empty via is not created", sub: "380937415826", iss: "viber_bm", via: ptr(""), wantErr: true},
		{name: "other issuer unknown stays not found", sub: "+380937415826", iss: "facebook", via: ptr(gate), wantErr: true},
		{name: "other issuer sub is not normalized", stored: []*impb.Contact{fbContact}, sub: "+123", iss: "facebook", via: ptr(gate), wantID: "fb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contacts := &fakeRecipientContacts{stored: tt.stored, createErr: tt.createErr}
			vias := &fakeRecipientVias{err: tt.viaErr}
			m := &MessageService{logger: slog.New(slog.DiscardHandler), contacter: contacts, vias: vias}

			peer, err := m.resolveContact(context.Background(), tt.sub, tt.iss, tt.via, 1)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && peer.GetContactId() != tt.wantID {
				t.Errorf("contact id = %q, want %q", peer.GetContactId(), tt.wantID)
			}

			if len(contacts.creates) != tt.wantCreates {
				t.Errorf("creates = %d, want %d", len(contacts.creates), tt.wantCreates)
			}

			if len(vias.created) != tt.wantVias {
				t.Errorf("vias = %d, want %d", len(vias.created), tt.wantVias)
			}

			for _, c := range contacts.creates {
				if c.GetSubject() != "380937415826" || c.GetUsername() != c.GetSubject() || c.GetType() != "viber_bm" || c.GetDomainId() != 1 {
					t.Errorf("unexpected create request: %v", c)
				}
			}

			for _, v := range vias.created {
				if v.GetVia() != gate || v.GetContactId() != "new" {
					t.Errorf("unexpected via request: %v", v)
				}
			}
		})
	}
}
