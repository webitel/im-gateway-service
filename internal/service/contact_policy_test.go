package service

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/webitel/webitel-go-kit/pkg/errors"
	"google.golang.org/grpc/codes"

	adminv1 "github.com/webitel/im-gateway-service/gen/go/admin/v1"
	contactv1 "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
	"github.com/webitel/im-gateway-service/infra/auth/standard"
)

type fakeContactBackend struct {
	searches []*contactv1.SearchContactRequest
	locates  []*contactv1.LocateContactRequest
}

func (f *fakeContactBackend) SearchContact(_ context.Context, req *contactv1.SearchContactRequest) (*contactv1.ContactList, error) {
	f.searches = append(f.searches, req)

	return &contactv1.ContactList{}, nil
}

func (f *fakeContactBackend) CreateContact(context.Context, *contactv1.CreateContactRequest) (*contactv1.Contact, error) {
	return &contactv1.Contact{}, nil
}

func (f *fakeContactBackend) LocateContact(_ context.Context, req *contactv1.LocateContactRequest) (*contactv1.LocateContactResponse, error) {
	f.locates = append(f.locates, req)

	return &contactv1.LocateContactResponse{}, nil
}

func newPolicyContactService(app *fakeAppConfig) (*ContactService, *fakeContactBackend) {
	backend := &fakeContactBackend{}

	return &ContactService{
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		contactClient: backend,
		appConfig:     app,
	}, backend
}

func identityCtx(appID string) context.Context {
	return context.WithValue(context.Background(), auth.AuthContextKey, auth.Identifier(&standard.Identity{AppID: appID, DomainID: 1}))
}

func allowPolicy(ids ...string) AppPolicy {
	rules := make([]*adminv1.ContactAllowRule, 0, len(ids))
	for _, id := range ids {
		rules = append(rules, &adminv1.ContactAllowRule{ContactId: id})
	}

	return AppPolicy{Contacts: NewContactPolicy(&adminv1.ContactAllowList{Rules: rules})}
}

func TestContactService_SearchContact_ContactPolicy(t *testing.T) {
	tests := []struct {
		name     string
		appID    string
		app      *fakeAppConfig
		reqIDs   []string
		wantCode codes.Code
		wantRPC  bool
		wantIDs  []string
	}{
		{name: "no app id: unfiltered", appID: "", app: &fakeAppConfig{policy: allowPolicy("a")}, wantRPC: true},
		{name: "unrestricted app: request ids kept", appID: "app", app: &fakeAppConfig{}, reqIDs: []string{"x"}, wantRPC: true, wantIDs: []string{"x"}},
		{name: "restricted, no ids: whole allow-list", appID: "app", app: &fakeAppConfig{policy: allowPolicy("a", "b")}, wantRPC: true, wantIDs: []string{"a", "b"}},
		{name: "restricted, ids narrowed to allow-list", appID: "app", app: &fakeAppConfig{policy: allowPolicy("a", "b")}, reqIDs: []string{"b", "x"}, wantRPC: true, wantIDs: []string{"b"}},
		{name: "restricted, no overlap: empty list without rpc", appID: "app", app: &fakeAppConfig{policy: allowPolicy("a")}, reqIDs: []string{"x"}},
		{name: "widget deny-all: empty list without rpc", appID: "app", app: &fakeAppConfig{policy: AppPolicy{Contacts: NewAppContactPolicy(&adminv1.Application{Type: adminv1.AppType_APP_TYPE_WIDGET})}}},
		{name: "policy unavailable", appID: "app", app: &fakeAppConfig{err: errors.Unavailable("down", errors.WithID("test"))}, wantCode: codes.Unavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, backend := newPolicyContactService(tt.app)

			res, err := svc.SearchContact(identityCtx(tt.appID), &contactv1.SearchContactRequest{Ids: tt.reqIDs})
			if got := errors.Code(err); got != tt.wantCode {
				t.Fatalf("code = %v, want %v (err: %v)", got, tt.wantCode, err)
			}

			if tt.wantCode != codes.OK {
				return
			}

			if len(res.GetContacts()) != 0 {
				t.Fatalf("unexpected contacts: %v", res.GetContacts())
			}

			if (len(backend.searches) == 1) != tt.wantRPC {
				t.Fatalf("SearchContact rpc calls = %d, wantRPC %v", len(backend.searches), tt.wantRPC)
			}

			if !tt.wantRPC {
				return
			}

			got := slices.Sorted(slices.Values(backend.searches[0].GetIds()))
			if !slices.Equal(got, tt.wantIDs) {
				t.Fatalf("Ids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func TestContactService_Locate_ContactPolicy(t *testing.T) {
	tests := []struct {
		name     string
		appID    string
		app      *fakeAppConfig
		id       string
		wantCode codes.Code
	}{
		{name: "no app id", appID: "", app: &fakeAppConfig{policy: allowPolicy("a")}, id: "x"},
		{name: "unrestricted app", appID: "app", app: &fakeAppConfig{}, id: "x"},
		{name: "restricted, allowed", appID: "app", app: &fakeAppConfig{policy: allowPolicy("a")}, id: "a"},
		{name: "restricted, not allowed", appID: "app", app: &fakeAppConfig{policy: allowPolicy("a")}, id: "x", wantCode: codes.PermissionDenied},
		{name: "restricted, empty id", appID: "app", app: &fakeAppConfig{policy: allowPolicy("a")}, id: "", wantCode: codes.PermissionDenied},
		{name: "policy unavailable", appID: "app", app: &fakeAppConfig{err: errors.Unavailable("down", errors.WithID("test"))}, id: "a", wantCode: codes.Unavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, backend := newPolicyContactService(tt.app)

			_, err := svc.Locate(identityCtx(tt.appID), &contactv1.LocateContactRequest{Id: tt.id})
			if got := errors.Code(err); got != tt.wantCode {
				t.Fatalf("code = %v, want %v (err: %v)", got, tt.wantCode, err)
			}

			// The policy is checked before the backend: a denied id never reaches im-contact.
			if wantRPC := tt.wantCode == codes.OK; (len(backend.locates) == 1) != wantRPC {
				t.Fatalf("LocateContact rpc calls = %d, wantRPC %v", len(backend.locates), wantRPC)
			}
		})
	}
}
