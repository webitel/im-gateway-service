package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	werrors "github.com/webitel/webitel-go-kit/pkg/errors"

	adminv1 "github.com/webitel/im-gateway-service/gen/go/admin/v1"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeAdminAppSearcher struct {
	mu    sync.Mutex
	calls int
	// Configurable response
	response *adminv1.ApplicationList
	err      error
	// Captures the last request and outgoing metadata seen, so tests can assert
	// on what resolve() actually sends (Dc/Fields, stripped identity headers, ...).
	lastReq *adminv1.SearchAppRequest
	lastMD  metadata.MD
}

func newFakeAdminAppSearcher() *fakeAdminAppSearcher {
	return &fakeAdminAppSearcher{
		response: nil,
		err:      nil,
	}
}

func (f *fakeAdminAppSearcher) SearchApps(ctx context.Context, in *adminv1.SearchAppRequest, opts ...grpc.CallOption) (*adminv1.ApplicationList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls++
	f.lastReq = in
	f.lastMD, _ = metadata.FromOutgoingContext(ctx)

	return f.response, f.err
}

func (f *fakeAdminAppSearcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls
}

func (f *fakeAdminAppSearcher) lastRequest() *adminv1.SearchAppRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.lastReq
}

func (f *fakeAdminAppSearcher) lastMetadata() metadata.MD {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.lastMD
}

func TestResolveAppPolicy_SystemMessages_EmptyAppID_ReturnsAllowAll(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := systemMessages(svc.ResolveAppPolicy(context.Background(), 1, ""))
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("empty appID should return allow-all policy (nil DTO)")
	}

	if fake.callCount() != 0 {
		t.Errorf("empty appID should not call admin client, got %d calls", fake.callCount())
	}
}

func TestResolveAppPolicy_SystemMessages_RPCError_ZeroPolicy(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.err = errors.New("admin service unreachable")
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("RPC error should fail open (allow-all, nil DTO)")
	}
}

func TestResolveAppPolicy_SystemMessages_NilResponse_ZeroPolicy(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = nil
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("nil response should fail open (allow-all, nil DTO)")
	}
}

func TestResolveAppPolicy_SystemMessages_EmptyData_ZeroPolicy(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{}}
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("empty data should fail open (allow-all, nil DTO)")
	}
}

func TestResolveAppPolicy_SystemMessages_AllowSystemMessagesNil_AllowAll(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{
			{
				Id:                  "myapp",
				AllowSystemMessages: nil,
			},
		},
	}
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("nil AllowSystemMessages should return allow-all policy (nil DTO)")
	}
}

func TestResolveAppPolicy_SystemMessages_AllowSystemMessagesEmpty_DenyAll(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{
			{
				Id: "myapp",
				AllowSystemMessages: &adminv1.SystemMessageAllowList{
					Types: []string{},
				},
			},
		},
	}
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))
	dto := policy.ToDTO()

	if dto == nil || len(dto.Types) != 0 {
		t.Error("empty allow-list should return restricted policy with empty types")
	}
}

func TestResolveAppPolicy_SystemMessages_AllowSystemMessagesWithTypes_AllowSpecific(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{
			{
				Id: "myapp",
				AllowSystemMessages: &adminv1.SystemMessageAllowList{
					Types: []string{"user_joined"},
				},
			},
		},
	}
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))
	dto := policy.ToDTO()

	if dto == nil || len(dto.Types) != 1 || dto.Types[0] != "user_joined" {
		t.Error("allow-list should contain user_joined")
	}
}

func TestResolveAppPolicy_SystemMessages_ResolvesOnce_Cached(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{
			{
				Id: "myapp",
				AllowSystemMessages: &adminv1.SystemMessageAllowList{
					Types: []string{"user_joined"},
				},
			},
		},
	}
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	// First call
	policy1 := systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))
	// Second call (should hit cache, not call admin)
	policy2 := systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))

	if policy1.ToDTO() == nil || policy2.ToDTO() == nil {
		t.Error("both calls should resolve to restricted policy")
	}

	// Should have exactly 1 admin call
	if fake.callCount() != 1 {
		t.Errorf("expected exactly 1 admin call (cached), got %d", fake.callCount())
	}
}

func TestSystemMessagePolicy_ToDTO_Unrestricted_ReturnsNil(t *testing.T) {
	policy := SystemMessagePolicy{restricted: false}
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("unrestricted policy should return nil DTO")
	}
}

func TestSystemMessagePolicy_ToDTO_RestrictedEmpty_ReturnsNonNilEmpty(t *testing.T) {
	policy := SystemMessagePolicy{restricted: true, allowed: []string{}}
	dtoResult := policy.ToDTO()

	if dtoResult == nil {
		t.Error("restricted policy should return non-nil DTO")
	}

	if len(dtoResult.Types) != 0 {
		t.Error("restricted policy with empty allow list should have empty Types")
	}
}

func TestSystemMessagePolicy_ToDTO_RestrictedWithTypes_ReturnsNonNilWithTypes(t *testing.T) {
	policy := SystemMessagePolicy{restricted: true, allowed: []string{"user_joined", "user_left"}}
	dtoResult := policy.ToDTO()

	if dtoResult == nil {
		t.Error("restricted policy should return non-nil DTO")
	}

	if len(dtoResult.Types) != 2 {
		t.Errorf("expected 2 types, got %d", len(dtoResult.Types))
	}

	if dtoResult.Types[0] != "user_joined" || dtoResult.Types[1] != "user_left" {
		t.Error("types should match allowed list")
	}
}

func TestResolveAppPolicy_SystemMessages_RequestCarriesDcAndFieldMask(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{{Id: "myapp"}},
	}
	svc := NewAppConfigService(fake, discardLogger())

	systemMessages(svc.ResolveAppPolicy(context.Background(), 42, "myapp"))

	req := fake.lastRequest()
	if req == nil {
		t.Fatal("expected a request to be sent to admin service")
	}

	if req.GetDc() != 42 {
		t.Errorf("expected Dc=42, got %d", req.GetDc())
	}

	if req.GetId() != "myapp" {
		t.Errorf("expected Id=myapp, got %q", req.GetId())
	}

	wantFields := []string{"id", "type", "allow_system_messages", "allow_contacts"}
	if gotFields := req.GetFields(); !slices.Equal(gotFields, wantFields) {
		t.Errorf("expected Fields=%v, got %v", wantFields, gotFields)
	}
}

func TestResolveAppPolicy_SystemMessages_StripsIdentityMetadata_PreservesOtherMetadata(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{{Id: "myapp"}},
	}
	svc := NewAppConfigService(fake, discardLogger())

	callerCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		"x-jwt-payload", "secret-token",
		"x-webitel-device", "device-1",
		"x-trace-id", "trace-123",
	))

	systemMessages(svc.ResolveAppPolicy(callerCtx, 1, "myapp"))

	md := fake.lastMetadata()
	if len(md.Get("x-jwt-payload")) != 0 {
		t.Error("x-jwt-payload should be stripped from the admin-service call")
	}

	if len(md.Get("x-webitel-device")) != 0 {
		t.Error("x-webitel-device should be stripped from the admin-service call")
	}

	if got := md.Get("x-trace-id"); len(got) != 1 || got[0] != "trace-123" {
		t.Errorf("expected x-trace-id to be preserved, got %v", got)
	}
}

func TestResolveAppPolicy_SystemMessages_CanceledCallerCtx_DoesNotAbortLookup(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{{Id: "myapp"}},
	}
	svc := NewAppConfigService(fake, discardLogger())

	callerCtx, cancel := context.WithCancel(context.Background())
	cancel()

	policy := systemMessages(svc.ResolveAppPolicy(callerCtx, 1, "myapp"))
	if policy.ToDTO() != nil {
		t.Error("a pre-canceled caller ctx should not prevent a normal successful resolution")
	}

	if fake.callCount() != 1 {
		t.Errorf("expected exactly 1 admin call, got %d", fake.callCount())
	}
}

func TestResolveAppPolicy_SystemMessages_DomainScoped_SameAppDifferentDomainsResolveIndependently(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{{Id: "myapp"}},
	}
	svc := NewAppConfigService(fake, discardLogger())

	systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))
	systemMessages(svc.ResolveAppPolicy(context.Background(), 2, "myapp"))

	if fake.callCount() != 2 {
		t.Errorf("expected 2 admin calls (one per domain), got %d", fake.callCount())
	}

	if _, ok := svc.successCache.Get(cacheKey(1, "myapp")); !ok {
		t.Error("expected a cached entry for domain 1")
	}

	if _, ok := svc.successCache.Get(cacheKey(2, "myapp")); !ok {
		t.Error("expected a cached entry for domain 2")
	}
}

func TestResolveAppPolicy_SystemMessages_IdMismatch_ZeroPolicyAndCachesDefinitive(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{{Id: "some-other-app"}},
	}
	svc := NewAppConfigService(fake, discardLogger())

	policy := systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))
	if policy.ToDTO() != nil {
		t.Error("id mismatch should fail open (allow-all, nil DTO)")
	}

	key := cacheKey(1, "myapp")
	if _, ok := svc.notFoundCache.Get(key); !ok {
		t.Error("id mismatch should be cached as a definitive not-found")
	}

	if _, ok := svc.failureCache.Get(key); ok {
		t.Error("id mismatch should not be cached as a transient failure")
	}

	// A second call should hit the notFoundCache, not re-issue the RPC.
	systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))

	if fake.callCount() != 1 {
		t.Errorf("expected exactly 1 admin call (suppressed by notFoundCache), got %d", fake.callCount())
	}
}

func TestResolveAppPolicy_SystemMessages_NilResponse_CachedAsTransientNotDefinitive(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = nil
	svc := NewAppConfigService(fake, discardLogger())

	systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))

	key := cacheKey(1, "myapp")
	if _, ok := svc.failureCache.Get(key); !ok {
		t.Error("a nil response with a nil error should be cached as a transient failure")
	}

	if _, ok := svc.notFoundCache.Get(key); ok {
		t.Error("a nil response with a nil error should NOT be cached as a definitive not-found")
	}
}

func TestResolveAppPolicy_SystemMessages_EmptyData_CachedAsDefinitiveNotFound(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{}}
	svc := NewAppConfigService(fake, discardLogger())

	systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))

	key := cacheKey(1, "myapp")
	if _, ok := svc.notFoundCache.Get(key); !ok {
		t.Error("an empty Data list should be cached as a definitive not-found")
	}

	if _, ok := svc.failureCache.Get(key); ok {
		t.Error("an empty Data list should NOT be cached as a transient failure")
	}
}

func TestResolveAppPolicy_SystemMessages_RPCError_CachedAsTransientNotDefinitive(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.err = errors.New("admin service unreachable")
	svc := NewAppConfigService(fake, discardLogger())

	systemMessages(svc.ResolveAppPolicy(context.Background(), 1, "myapp"))

	key := cacheKey(1, "myapp")
	if _, ok := svc.failureCache.Get(key); !ok {
		t.Error("an RPC error should be cached as a transient failure")
	}

	if _, ok := svc.notFoundCache.Get(key); ok {
		t.Error("an RPC error should NOT be cached as a definitive not-found")
	}
}

func TestResolveAppPolicy_EmptyAppID_NoRPC(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	svc := NewAppConfigService(fake, discardLogger())

	policy, err := svc.ResolveAppPolicy(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("empty appID should not error, got %v", err)
	}

	if policy.Contacts.Restricted() || policy.SystemMessages.ToDTO() != nil {
		t.Error("empty appID should return the zero-value policy")
	}

	if fake.callCount() != 0 {
		t.Errorf("empty appID should not call admin client, got %d calls", fake.callCount())
	}
}

func TestResolveAppPolicy_RPCError_UnavailableAndCachedAsFailure(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.err = errors.New("admin service unreachable")
	svc := NewAppConfigService(fake, discardLogger())

	_, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if werrors.Code(err) != codes.Unavailable {
		t.Errorf("RPC error should be codes.Unavailable, got %v (%v)", werrors.Code(err), err)
	}

	_, err = svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if werrors.Code(err) != codes.Unavailable {
		t.Errorf("cached transient failure should be codes.Unavailable, got %v", werrors.Code(err))
	}

	if fake.callCount() != 1 {
		t.Errorf("expected 1 admin call (failure cache back-off), got %d", fake.callCount())
	}
}

func TestResolveAppPolicy_NilResponse_Unavailable(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = nil
	svc := NewAppConfigService(fake, discardLogger())

	_, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if werrors.Code(err) != codes.Unavailable {
		t.Errorf("nil response should be codes.Unavailable, got %v", werrors.Code(err))
	}
}

func TestResolveAppPolicy_EmptyData_PermissionDenied(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{}}
	svc := NewAppConfigService(fake, discardLogger())

	_, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if werrors.Code(err) != codes.PermissionDenied {
		t.Errorf("empty data should be codes.PermissionDenied, got %v", werrors.Code(err))
	}

	_, err = svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if werrors.Code(err) != codes.PermissionDenied {
		t.Errorf("cached not-found should be codes.PermissionDenied, got %v", werrors.Code(err))
	}

	if fake.callCount() != 1 {
		t.Errorf("expected 1 admin call (not-found cache), got %d", fake.callCount())
	}
}

func TestResolveAppPolicy_IdMismatch_PermissionDenied(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{{Id: "some-other-app"}}}
	svc := NewAppConfigService(fake, discardLogger())

	_, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if werrors.Code(err) != codes.PermissionDenied {
		t.Errorf("id mismatch should be codes.PermissionDenied, got %v", werrors.Code(err))
	}

	_, err = svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if werrors.Code(err) != codes.PermissionDenied {
		t.Errorf("cached id mismatch should be codes.PermissionDenied, got %v", werrors.Code(err))
	}

	if fake.callCount() != 1 {
		t.Errorf("expected 1 admin call (not-found cache), got %d", fake.callCount())
	}
}

func TestResolveAppPolicy_AfterSuccessExpired_RPCErrorFailsClosed(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{{Id: "myapp"}}}
	svc := NewAppConfigService(fake, discardLogger())

	if _, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp"); err != nil {
		t.Fatalf("first resolve should succeed, got %v", err)
	}

	// Simulate the 60s success entry expiring, then admin-service going down.
	svc.successCache.Remove(cacheKey(1, "myapp"))

	fake.response = nil
	fake.err = errors.New("admin service unreachable")

	_, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if werrors.Code(err) != codes.Unavailable {
		t.Errorf("expired success entry + RPC error should be codes.Unavailable (no stale fallback), got %v", werrors.Code(err))
	}
}

func TestResolveAppPolicy_AllowContactsUnset_Unrestricted(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{{Id: "myapp", Type: adminv1.AppType_APP_TYPE_PORTAL}}}
	svc := NewAppConfigService(fake, discardLogger())

	policy, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if policy.Contacts.Restricted() {
		t.Error("allow_contacts unset on a PORTAL app should be unrestricted")
	}

	if !policy.Contacts.Allows("any-contact") {
		t.Error("unrestricted policy should allow any contact")
	}

	if _, ok := policy.Contacts.DefaultContactID(); ok {
		t.Error("unrestricted policy should have no default contact")
	}
}

func TestResolveAppPolicy_AllowContactsEmptyRules_DenyAll(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{{
		Id:            "myapp",
		Type:          adminv1.AppType_APP_TYPE_PORTAL,
		AllowContacts: &adminv1.ContactAllowList{},
	}}}
	svc := NewAppConfigService(fake, discardLogger())

	policy, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !policy.Contacts.Restricted() {
		t.Error("set allow_contacts with empty rules should be restricted")
	}

	if policy.Contacts.Allows("any-contact") {
		t.Error("restricted policy with empty rules should allow no contact")
	}

	if _, ok := policy.Contacts.DefaultContactID(); ok {
		t.Error("restricted policy with empty rules should have no default contact")
	}
}

func TestResolveAppPolicy_AllowContactsWithDefault(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{{
		Id:   "myapp",
		Type: adminv1.AppType_APP_TYPE_PORTAL,
		AllowContacts: &adminv1.ContactAllowList{Rules: []*adminv1.ContactAllowRule{
			{ContactId: "c1", Default: true},
			{ContactId: "c2"},
		}},
	}}}
	svc := NewAppConfigService(fake, discardLogger())

	policy, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !policy.Contacts.Allows("c1") || !policy.Contacts.Allows("c2") {
		t.Error("listed contacts should be allowed")
	}

	if policy.Contacts.Allows("c3") {
		t.Error("unlisted contact should not be allowed")
	}

	if id, ok := policy.Contacts.DefaultContactID(); !ok || id != "c1" {
		t.Errorf("expected default contact c1, got %q (ok=%v)", id, ok)
	}
}

func TestResolveAppPolicy_BothPolicies_OneRPC(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{{
		Id:                  "myapp",
		Type:                adminv1.AppType_APP_TYPE_PORTAL,
		AllowSystemMessages: &adminv1.SystemMessageAllowList{Types: []string{"user_joined"}},
		AllowContacts:       &adminv1.ContactAllowList{Rules: []*adminv1.ContactAllowRule{{ContactId: "c1"}}},
	}}}
	svc := NewAppConfigService(fake, discardLogger())

	policy, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	again, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if err != nil {
		t.Fatalf("unexpected error on cached lookup: %v", err)
	}

	if fake.callCount() != 1 {
		t.Errorf("expected exactly 1 admin call for both lookups, got %d", fake.callCount())
	}

	if dto := policy.SystemMessages.ToDTO(); dto == nil || len(dto.Types) != 1 || dto.Types[0] != "user_joined" {
		t.Error("system message policy should be populated from the same lookup")
	}

	if dto := again.SystemMessages.ToDTO(); dto == nil || len(dto.Types) != 1 || dto.Types[0] != "user_joined" {
		t.Error("cached lookup should return the same system message policy")
	}

	if !policy.Contacts.Restricted() || !policy.Contacts.Allows("c1") || policy.Contacts.Allows("c2") {
		t.Error("contacts policy should be populated from the same lookup")
	}
}

func TestNewContactPolicy(t *testing.T) {
	tests := []struct {
		name         string
		list         *adminv1.ContactAllowList
		wantRestrict bool
		wantAllowed  []string
		wantDefault  string
		wantHasDef   bool
	}{
		{
			name:         "nil list is unrestricted",
			list:         nil,
			wantRestrict: false,
		},
		{
			name:         "empty list is restricted with no contacts",
			list:         &adminv1.ContactAllowList{},
			wantRestrict: true,
		},
		{
			name: "rules without default",
			list: &adminv1.ContactAllowList{Rules: []*adminv1.ContactAllowRule{
				{ContactId: "c1"}, {ContactId: "c2"},
			}},
			wantRestrict: true,
			wantAllowed:  []string{"c1", "c2"},
		},
		{
			name: "empty contact id rule is ignored",
			list: &adminv1.ContactAllowList{Rules: []*adminv1.ContactAllowRule{
				{ContactId: "", Default: true}, {ContactId: "c1"},
			}},
			wantRestrict: true,
			wantAllowed:  []string{"c1"},
		},
		{
			name: "default rule is reported",
			list: &adminv1.ContactAllowList{Rules: []*adminv1.ContactAllowRule{
				{ContactId: "c1"}, {ContactId: "c2", Default: true},
			}},
			wantRestrict: true,
			wantAllowed:  []string{"c1", "c2"},
			wantDefault:  "c2",
			wantHasDef:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewContactPolicy(tt.list)

			if policy.Restricted() != tt.wantRestrict {
				t.Fatalf("Restricted()=%v, want %v", policy.Restricted(), tt.wantRestrict)
			}

			for _, id := range tt.wantAllowed {
				if !policy.Allows(id) {
					t.Errorf("Allows(%q) should be true", id)
				}
			}

			if tt.wantRestrict && policy.Allows("") {
				t.Error("empty contact id should never be allowed by a restricted policy")
			}

			id, ok := policy.DefaultContactID()
			if ok != tt.wantHasDef || id != tt.wantDefault {
				t.Errorf("DefaultContactID()=(%q,%v), want (%q,%v)", id, ok, tt.wantDefault, tt.wantHasDef)
			}
		})
	}
}

func TestNewAppContactPolicy(t *testing.T) {
	tests := []struct {
		name         string
		app          *adminv1.Application
		wantRestrict bool
		wantAllows   string
		wantAllowed  bool
	}{
		{
			name:         "widget without allow_contacts is deny-all",
			app:          &adminv1.Application{Type: adminv1.AppType_APP_TYPE_WIDGET},
			wantRestrict: true,
			wantAllows:   "c1",
			wantAllowed:  false,
		},
		{
			name: "widget with rules applies them",
			app: &adminv1.Application{
				Type:          adminv1.AppType_APP_TYPE_WIDGET,
				AllowContacts: &adminv1.ContactAllowList{Rules: []*adminv1.ContactAllowRule{{ContactId: "c1"}}},
			},
			wantRestrict: true,
			wantAllows:   "c1",
			wantAllowed:  true,
		},
		{
			name:         "portal without allow_contacts is unrestricted",
			app:          &adminv1.Application{Type: adminv1.AppType_APP_TYPE_PORTAL},
			wantRestrict: false,
			wantAllows:   "c1",
			wantAllowed:  true,
		},
		{
			name:         "unspecified type without allow_contacts is unrestricted",
			app:          &adminv1.Application{Type: adminv1.AppType_APP_TYPE_UNSPECIFIED},
			wantRestrict: false,
			wantAllows:   "c1",
			wantAllowed:  true,
		},
		{
			name: "unspecified type with allow_contacts is restricted",
			app: &adminv1.Application{
				Type:          adminv1.AppType_APP_TYPE_UNSPECIFIED,
				AllowContacts: &adminv1.ContactAllowList{Rules: []*adminv1.ContactAllowRule{{ContactId: "c1"}}},
			},
			wantRestrict: true,
			wantAllows:   "c1",
			wantAllowed:  true,
		},
		{
			name: "unspecified type with allow_contacts is restricted (not allowed test)",
			app: &adminv1.Application{
				Type:          adminv1.AppType_APP_TYPE_UNSPECIFIED,
				AllowContacts: &adminv1.ContactAllowList{Rules: []*adminv1.ContactAllowRule{{ContactId: "c1"}}},
			},
			wantRestrict: true,
			wantAllows:   "c2",
			wantAllowed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewAppContactPolicy(tt.app)

			if policy.Restricted() != tt.wantRestrict {
				t.Fatalf("Restricted()=%v, want %v", policy.Restricted(), tt.wantRestrict)
			}

			if policy.Allows(tt.wantAllows) != tt.wantAllowed {
				t.Errorf("Allows(%q)=%v, want %v", tt.wantAllows, !tt.wantAllowed, tt.wantAllowed)
			}

			if _, ok := policy.DefaultContactID(); ok {
				t.Error("no app in this table should have a default contact")
			}
		})
	}
}

func TestResolveAppPolicy_WidgetWithoutAllowContacts_DenyAll(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{{
		Id:   "myapp",
		Type: adminv1.AppType_APP_TYPE_WIDGET,
	}}}
	svc := NewAppConfigService(fake, discardLogger())

	policy, err := svc.ResolveAppPolicy(context.Background(), 1, "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !policy.Contacts.Restricted() {
		t.Error("WIDGET app without allow_contacts should be restricted")
	}

	if policy.Contacts.Allows("x") {
		t.Error("WIDGET app without allow_contacts should allow no contact")
	}
}

func TestResolveAppPolicy_RequestCarriesTypeAndContactsFields(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{{Id: "myapp"}}}
	svc := NewAppConfigService(fake, discardLogger())

	if _, err := svc.ResolveAppPolicy(context.Background(), 7, "myapp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantFields := []string{"id", "type", "allow_system_messages", "allow_contacts"}
	if gotFields := fake.lastRequest().GetFields(); !slices.Equal(gotFields, wantFields) {
		t.Errorf("expected Fields=%v, got %v", wantFields, gotFields)
	}
}

// systemMessages mirrors the fail-open callers (history, updates): on error the zero policy is used.
func systemMessages(policy AppPolicy, _ error) SystemMessagePolicy {
	return policy.SystemMessages
}
