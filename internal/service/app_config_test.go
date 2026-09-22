package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

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

func TestResolvePolicy_EmptyAppID_ReturnsAllowAll(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := svc.ResolvePolicy(context.Background(), 1, "")
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("empty appID should return allow-all policy (nil DTO)")
	}

	if fake.callCount() != 0 {
		t.Errorf("empty appID should not call admin client, got %d calls", fake.callCount())
	}
}

func TestResolvePolicy_RPCError_FailsOpen(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.err = errors.New("admin service unreachable")
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := svc.ResolvePolicy(context.Background(), 1, "myapp")
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("RPC error should fail open (allow-all, nil DTO)")
	}
}

func TestResolvePolicy_NilResponse_FailsOpen(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = nil
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := svc.ResolvePolicy(context.Background(), 1, "myapp")
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("nil response should fail open (allow-all, nil DTO)")
	}
}

func TestResolvePolicy_EmptyData_FailsOpen(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{}}
	svc := NewAppConfigService(fake, slog.New(slog.NewTextHandler(io.Discard, nil)))

	policy := svc.ResolvePolicy(context.Background(), 1, "myapp")
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("empty data should fail open (allow-all, nil DTO)")
	}
}

func TestResolvePolicy_AllowSystemMessagesNil_AllowAll(t *testing.T) {
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

	policy := svc.ResolvePolicy(context.Background(), 1, "myapp")
	dto := policy.ToDTO()

	if dto != nil {
		t.Error("nil AllowSystemMessages should return allow-all policy (nil DTO)")
	}
}

func TestResolvePolicy_AllowSystemMessagesEmpty_DenyAll(t *testing.T) {
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

	policy := svc.ResolvePolicy(context.Background(), 1, "myapp")
	dto := policy.ToDTO()

	if dto == nil || len(dto.Types) != 0 {
		t.Error("empty allow-list should return restricted policy with empty types")
	}
}

func TestResolvePolicy_AllowSystemMessagesWithTypes_AllowSpecific(t *testing.T) {
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

	policy := svc.ResolvePolicy(context.Background(), 1, "myapp")
	dto := policy.ToDTO()

	if dto == nil || len(dto.Types) != 1 || dto.Types[0] != "user_joined" {
		t.Error("allow-list should contain user_joined")
	}
}

func TestResolvePolicy_ResolvesOnce_Cached(t *testing.T) {
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
	policy1 := svc.ResolvePolicy(context.Background(), 1, "myapp")
	// Second call (should hit cache, not call admin)
	policy2 := svc.ResolvePolicy(context.Background(), 1, "myapp")

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

func TestResolvePolicy_RequestCarriesDcAndFieldMask(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{{Id: "myapp"}},
	}
	svc := NewAppConfigService(fake, discardLogger())

	svc.ResolvePolicy(context.Background(), 42, "myapp")

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

	wantFields := []string{"id", "allow_system_messages"}
	gotFields := req.GetFields()
	if len(gotFields) != len(wantFields) || gotFields[0] != wantFields[0] || gotFields[1] != wantFields[1] {
		t.Errorf("expected Fields=%v, got %v", wantFields, gotFields)
	}
}

func TestResolvePolicy_StripsIdentityMetadata_PreservesOtherMetadata(t *testing.T) {
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

	svc.ResolvePolicy(callerCtx, 1, "myapp")

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

func TestResolvePolicy_CanceledCallerCtx_DoesNotAbortLookup(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{{Id: "myapp"}},
	}
	svc := NewAppConfigService(fake, discardLogger())

	callerCtx, cancel := context.WithCancel(context.Background())
	cancel()

	policy := svc.ResolvePolicy(callerCtx, 1, "myapp")
	if policy.ToDTO() != nil {
		t.Error("a pre-canceled caller ctx should not prevent a normal successful resolution")
	}

	if fake.callCount() != 1 {
		t.Errorf("expected exactly 1 admin call, got %d", fake.callCount())
	}
}

func TestResolvePolicy_DomainScoped_SameAppDifferentDomainsResolveIndependently(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{{Id: "myapp"}},
	}
	svc := NewAppConfigService(fake, discardLogger())

	svc.ResolvePolicy(context.Background(), 1, "myapp")
	svc.ResolvePolicy(context.Background(), 2, "myapp")

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

func TestResolvePolicy_IdMismatch_FailsOpenAndCachesDefinitive(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{
		Data: []*adminv1.Application{{Id: "some-other-app"}},
	}
	svc := NewAppConfigService(fake, discardLogger())

	policy := svc.ResolvePolicy(context.Background(), 1, "myapp")
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
	svc.ResolvePolicy(context.Background(), 1, "myapp")
	if fake.callCount() != 1 {
		t.Errorf("expected exactly 1 admin call (suppressed by notFoundCache), got %d", fake.callCount())
	}
}

func TestResolvePolicy_NilResponse_CachedAsTransientNotDefinitive(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = nil
	svc := NewAppConfigService(fake, discardLogger())

	svc.ResolvePolicy(context.Background(), 1, "myapp")

	key := cacheKey(1, "myapp")
	if _, ok := svc.failureCache.Get(key); !ok {
		t.Error("a nil response with a nil error should be cached as a transient failure")
	}

	if _, ok := svc.notFoundCache.Get(key); ok {
		t.Error("a nil response with a nil error should NOT be cached as a definitive not-found")
	}
}

func TestResolvePolicy_EmptyData_CachedAsDefinitiveNotFound(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.response = &adminv1.ApplicationList{Data: []*adminv1.Application{}}
	svc := NewAppConfigService(fake, discardLogger())

	svc.ResolvePolicy(context.Background(), 1, "myapp")

	key := cacheKey(1, "myapp")
	if _, ok := svc.notFoundCache.Get(key); !ok {
		t.Error("an empty Data list should be cached as a definitive not-found")
	}

	if _, ok := svc.failureCache.Get(key); ok {
		t.Error("an empty Data list should NOT be cached as a transient failure")
	}
}

func TestResolvePolicy_RPCError_CachedAsTransientNotDefinitive(t *testing.T) {
	fake := newFakeAdminAppSearcher()
	fake.err = errors.New("admin service unreachable")
	svc := NewAppConfigService(fake, discardLogger())

	svc.ResolvePolicy(context.Background(), 1, "myapp")

	key := cacheKey(1, "myapp")
	if _, ok := svc.failureCache.Get(key); !ok {
		t.Error("an RPC error should be cached as a transient failure")
	}

	if _, ok := svc.notFoundCache.Get(key); ok {
		t.Error("an RPC error should NOT be cached as a definitive not-found")
	}
}
