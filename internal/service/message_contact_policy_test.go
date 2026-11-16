package service

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/webitel/webitel-go-kit/pkg/errors"

	adminv1 "github.com/webitel/im-gateway-service/gen/go/admin/v1"
	impb "github.com/webitel/im-gateway-service/gen/go/contact/v1"
	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	"github.com/webitel/im-gateway-service/infra/auth/standard"
	"github.com/webitel/im-gateway-service/internal/domain/shared"
)

type fakeAppConfig struct {
	policy AppPolicy
	err    error
	calls  int
}

func (f *fakeAppConfig) ResolveAppPolicy(_ context.Context, _ int64, _ string) (AppPolicy, error) {
	f.calls++

	return f.policy, f.err
}

// fakePolicyContacts.SearchContact matches by Ids when set, else by sub+iss.
type fakePolicyContacts struct {
	stored   []*impb.Contact
	searches []*impb.SearchContactRequest
	creates  []*impb.CreateContactRequest
}

func (f *fakePolicyContacts) SearchContact(_ context.Context, req *impb.SearchContactRequest) (*impb.ContactList, error) {
	f.searches = append(f.searches, req)

	var out []*impb.Contact

	for _, c := range f.stored {
		if len(req.GetIds()) > 0 {
			if slices.Contains(req.GetIds(), c.GetId()) {
				out = append(out, c)
			}

			continue
		}

		if slices.Contains(req.GetSubjects(), c.GetSubject()) && slices.Contains(req.GetIssId(), c.GetIssId()) {
			out = append(out, c)
		}
	}

	return &impb.ContactList{Contacts: out}, nil
}

func (f *fakePolicyContacts) CreateContact(_ context.Context, req *impb.CreateContactRequest) (*impb.Contact, error) {
	f.creates = append(f.creates, req)

	return &impb.Contact{Id: "new", IssId: req.GetIssId(), Subject: req.GetSubject(), Name: req.GetName()}, nil
}

func policyStoredContacts() []*impb.Contact {
	return []*impb.Contact{
		{Id: "bot-1", Subject: "sub-bot-1", IssId: "webitel", Name: "Bot One"},
		{Id: "bot-2", Subject: "sub-bot-2", IssId: "webitel", Name: "Bot Two"},
		{Id: "stranger", Subject: "sub-stranger", IssId: "webitel", Name: "Stranger"},
	}
}

func newPolicyTestService(contacts *fakePolicyContacts, appCfg AppConfigProvider) *MessageService {
	return &MessageService{
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		contacter: contacts,
		appConfig: appCfg,
	}
}

func policyWidgetIdentity() *standard.Identity {
	return &standard.Identity{ContactID: "end-user", DomainID: 1, AppID: "widget-app", Issuer: "widget"}
}

func policyRules(rules ...*adminv1.ContactAllowRule) AppPolicy {
	return AppPolicy{Contacts: NewContactPolicy(&adminv1.ContactAllowList{Rules: rules})}
}

func policyRule(id string, def bool) *adminv1.ContactAllowRule {
	return &adminv1.ContactAllowRule{ContactId: id, Default: def}
}

func TestResolveSendMetadata_ContactPolicy(t *testing.T) {
	t.Parallel()

	botRules := AppPolicy{Contacts: NewContactPolicy(&adminv1.ContactAllowList{
		Rules: []*adminv1.ContactAllowRule{policyRule("bot-1", true), policyRule("bot-2", false)},
	})}
	emptyRules := AppPolicy{Contacts: NewContactPolicy(&adminv1.ContactAllowList{})}
	unrestricted := AppPolicy{Contacts: NewContactPolicy(nil)}
	noDefault := policyRules(policyRule("bot-2", false))
	ghostDefault := policyRules(policyRule("ghost", true))

	botTo := shared.Peer{ID: "sub-bot-2", Issuer: "webitel", Type: shared.PeerContact}
	strangerTo := shared.Peer{ID: "sub-stranger", Issuer: "webitel", Type: shared.PeerContact}

	tests := []struct {
		name        string
		policy      AppPolicy
		appErr      error
		noAppConfig bool
		identity    *standard.Identity
		to          shared.Peer
		wantCode    codes.Code
		check       func(t *testing.T, to *threadv1.Peer, contacts *fakePolicyContacts, app *fakeAppConfig)
	}{
		{
			name:   "empty to with default resolves default contact",
			policy: botRules,
			to:     shared.Peer{},
			check: func(t *testing.T, to *threadv1.Peer, contacts *fakePolicyContacts, _ *fakeAppConfig) {
				t.Helper()

				if to.GetContactId() != "bot-1" {
					t.Fatalf("contact id = %q, want bot-1", to.GetContactId())
				}

				if to.GetIdentity().GetName() != "Bot One" {
					t.Fatalf("name = %q, want Bot One", to.GetIdentity().GetName())
				}

				if len(contacts.searches) != 1 {
					t.Fatalf("searches = %d, want 1", len(contacts.searches))
				}

				req := contacts.searches[0]
				if !slices.Equal(req.GetIds(), []string{"bot-1"}) || req.GetDomainId() != 1 {
					t.Fatalf("search request ids=%v domain=%d, want [bot-1] domain 1", req.GetIds(), req.GetDomainId())
				}
			},
		},
		{
			name:     "empty to with default missing in domain is not found",
			policy:   ghostDefault,
			to:       shared.Peer{},
			wantCode: codes.NotFound,
		},
		{
			name:     "empty to with restricted policy without default is invalid argument",
			policy:   noDefault,
			to:       shared.Peer{},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "empty to with unrestricted policy is invalid argument",
			policy:   unrestricted,
			to:       shared.Peer{},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "empty to without app id is invalid argument and skips policy lookup",
			policy:   botRules,
			identity: &standard.Identity{ContactID: "operator", DomainID: 1, Issuer: "webitel"},
			to:       shared.Peer{},
			wantCode: codes.InvalidArgument,
			check: func(t *testing.T, _ *threadv1.Peer, _ *fakePolicyContacts, app *fakeAppConfig) {
				t.Helper()

				if app.calls != 0 {
					t.Fatalf("policy lookups = %d, want 0", app.calls)
				}
			},
		},
		{
			name:   "explicit allowed contact is sent",
			policy: botRules,
			to:     botTo,
			check: func(t *testing.T, to *threadv1.Peer, _ *fakePolicyContacts, _ *fakeAppConfig) {
				t.Helper()

				if to.GetContactId() != "bot-2" {
					t.Fatalf("contact id = %q, want bot-2", to.GetContactId())
				}
			},
		},
		{
			name:     "explicit contact outside allow-list is denied",
			policy:   botRules,
			to:       strangerTo,
			wantCode: codes.PermissionDenied,
		},
		{
			name:     "unknown implicit contact is not created for restricted app",
			policy:   botRules,
			to:       shared.Peer{ID: "+380501234567", Issuer: "viber_bm", Type: shared.PeerContact, Via: ptr("gate-1")},
			wantCode: codes.PermissionDenied, // unknown == forbidden: no existence oracle
			check: func(t *testing.T, _ *threadv1.Peer, contacts *fakePolicyContacts, _ *fakeAppConfig) {
				t.Helper()

				if len(contacts.creates) != 0 {
					t.Fatalf("creates = %d, want 0", len(contacts.creates))
				}
			},
		},
		{
			name:     "unknown contact is denied like a forbidden one for restricted app",
			policy:   botRules,
			to:       shared.Peer{ID: "sub-nobody", Issuer: "webitel", Type: shared.PeerContact},
			wantCode: codes.PermissionDenied,
		},
		{
			name:     "unknown contact stays not found for unrestricted app",
			policy:   unrestricted,
			to:       shared.Peer{ID: "sub-nobody", Issuer: "webitel", Type: shared.PeerContact},
			wantCode: codes.NotFound,
		},
		{
			name:   "thread peer passes through restricted policy",
			policy: emptyRules,
			to:     shared.Peer{ID: "thread-1", Type: shared.PeerThread},
			check: func(t *testing.T, to *threadv1.Peer, _ *fakePolicyContacts, _ *fakeAppConfig) {
				t.Helper()

				if to.GetThreadId() != "thread-1" {
					t.Fatalf("thread id = %q, want thread-1", to.GetThreadId())
				}
			},
		},
		{
			name:     "group peer is denied for restricted app",
			policy:   botRules,
			to:       shared.Peer{ID: "group-1", Type: shared.PeerGroup},
			wantCode: codes.PermissionDenied,
		},
		{
			name:     "channel peer is denied for restricted app",
			policy:   botRules,
			to:       shared.Peer{ID: "channel-1", Type: shared.PeerChannel},
			wantCode: codes.PermissionDenied,
		},
		{
			name:   "unrestricted app sends to any contact",
			policy: unrestricted,
			to:     strangerTo,
			check: func(t *testing.T, to *threadv1.Peer, _ *fakePolicyContacts, _ *fakeAppConfig) {
				t.Helper()

				if to.GetContactId() != "stranger" {
					t.Fatalf("contact id = %q, want stranger", to.GetContactId())
				}
			},
		},
		{
			name:   "unrestricted app sends to group",
			policy: unrestricted,
			to:     shared.Peer{ID: "group-1", Type: shared.PeerGroup},
			check: func(t *testing.T, to *threadv1.Peer, _ *fakePolicyContacts, _ *fakeAppConfig) {
				t.Helper()

				if to.GetGroupId() != "group-1" {
					t.Fatalf("group id = %q, want group-1", to.GetGroupId())
				}
			},
		},
		{
			name:     "policy lookup unavailable fails closed",
			appErr:   errors.Unavailable("application policy is temporarily unavailable"),
			to:       botTo,
			wantCode: codes.Unavailable,
			check: func(t *testing.T, _ *threadv1.Peer, contacts *fakePolicyContacts, _ *fakeAppConfig) {
				t.Helper()

				if len(contacts.searches) != 0 {
					t.Fatalf("contact searches = %d, want 0", len(contacts.searches))
				}
			},
		},
		{
			name:     "policy lookup denied propagates permission denied",
			appErr:   errors.Forbidden("application not found"),
			to:       botTo,
			wantCode: codes.PermissionDenied,
		},
		{
			name:        "nil app config behaves as unrestricted",
			noAppConfig: true,
			to:          strangerTo,
			check: func(t *testing.T, to *threadv1.Peer, _ *fakePolicyContacts, _ *fakeAppConfig) {
				t.Helper()

				if to.GetContactId() != "stranger" {
					t.Fatalf("contact id = %q, want stranger", to.GetContactId())
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			contacts := &fakePolicyContacts{stored: policyStoredContacts()}
			app := &fakeAppConfig{policy: tc.policy, err: tc.appErr}

			var appCfg AppConfigProvider = app
			if tc.noAppConfig {
				appCfg = nil
			}

			m := newPolicyTestService(contacts, appCfg)

			identity := tc.identity
			if identity == nil {
				identity = policyWidgetIdentity()
			}

			to, _, err := m.resolveSendMetadata(context.Background(), tc.to, nil, identity)

			gotCode := codes.OK
			if err != nil {
				gotCode = errors.Code(err)
			}

			if gotCode != tc.wantCode {
				t.Fatalf("code = %v, want %v (err=%v)", gotCode, tc.wantCode, err)
			}

			if tc.check != nil {
				tc.check(t, to, contacts, app)
			}
		})
	}
}

func TestResolveSendMetadata_ContactPolicyDeniedMessageHasNoContactIDs(t *testing.T) {
	t.Parallel()

	app := &fakeAppConfig{policy: policyRules(policyRule("bot-1", true))}
	m := newPolicyTestService(&fakePolicyContacts{stored: policyStoredContacts()}, app)

	_, _, err := m.resolveSendMetadata(context.Background(),
		shared.Peer{ID: "sub-stranger", Issuer: "webitel", Type: shared.PeerContact},
		nil, policyWidgetIdentity())
	if err == nil {
		t.Fatal("expected denial, got nil")
	}

	msg := err.Error()
	for _, leak := range []string{"bot-1", "sub-bot-1", "sub-stranger"} {
		if strings.Contains(msg, leak) {
			t.Fatalf("denial message %q leaks %q", msg, leak)
		}
	}
}
