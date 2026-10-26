package grpc

import (
	"context"
	"testing"

	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	"github.com/webitel/im-gateway-service/infra/auth"
)

type gateTestIdentity struct{ domainID int64 }

func (i gateTestIdentity) GetContactID() string     { return "" }
func (i gateTestIdentity) GetDomainID() int64       { return i.domainID }
func (i gateTestIdentity) GetIssuer() string        { return "" }
func (i gateTestIdentity) GetName() string          { return "" }
func (i gateTestIdentity) GetChatName() string      { return "" }
func (i gateTestIdentity) GetApplicationID() string { return "" }
func (i gateTestIdentity) GetVia() string           { return "" }
func (i gateTestIdentity) GetViaPtr() *string       { return nil }

func TestScopeListGatesRequest(t *testing.T) {
	ctx := context.WithValue(context.Background(), auth.AuthContextKey, gateTestIdentity{domainID: 3})
	req := &providerv1.ProviderListGatesRequest{DomainId: 99, Size: 10, Q: "shop"}

	got, err := scopeListGatesRequest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetDomainId() != 3 {
		t.Errorf("DomainId = %d, want the caller's domain 3", got.GetDomainId())
	}

	if got.GetSize() != 10 || got.GetQ() != "shop" {
		t.Errorf("filters not forwarded: %+v", got)
	}

	if req.GetDomainId() != 99 {
		t.Error("the incoming request must not be mutated")
	}
}

func TestScopeListGatesRequest_NoIdentity(t *testing.T) {
	if _, err := scopeListGatesRequest(context.Background(), &providerv1.ProviderListGatesRequest{DomainId: 99}); err == nil {
		t.Fatal("expected an error without a caller identity")
	}
}
