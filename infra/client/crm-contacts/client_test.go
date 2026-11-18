package crmcontacts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	crmpb "github.com/webitel/im-gateway-service/gen/go/webitel/contacts"
)

// fakeCRM answers a search with the requested ids it allows and records what
// reached the CRM.
type fakeCRM struct {
	allowed map[string]bool
	err     error

	reqs   []*crmpb.SearchContactsRequest
	tokens [][]string
}

func (f *fakeCRM) search(ctx context.Context, req *crmpb.SearchContactsRequest) (*crmpb.ContactList, error) {
	f.reqs = append(f.reqs, req)

	md, _ := metadata.FromOutgoingContext(ctx)
	f.tokens = append(f.tokens, md.Get(accessTokenHeader))

	if f.err != nil {
		return nil, f.err
	}

	out := &crmpb.ContactList{}

	for _, id := range req.GetId() {
		if f.allowed[id] {
			out.Data = append(out.Data, &crmpb.Contact{Id: id})
		}
	}

	return out, nil
}

func newTestClient(f *fakeCRM) *Client {
	return &Client{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), search: f.search}
}

func withTokens(tokens ...string) context.Context {
	md := metadata.MD{}
	for _, t := range tokens {
		md.Append(accessTokenHeader, t)
	}

	return metadata.NewIncomingContext(context.Background(), md)
}

func TestClient_AccessMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(c *Client, ctx context.Context, ids []string) (map[string]bool, error)
		want crmpb.AccessMode
	}{
		{name: "visible asks for read access", call: (*Client).VisibleContacts, want: crmpb.AccessMode_READ},
		{name: "writable asks for write access", call: (*Client).WritableContacts, want: crmpb.AccessMode_WRITE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := &fakeCRM{allowed: map[string]bool{"1": true}}

			got, err := tt.call(newTestClient(f), withTokens("test-token-operator"), []string{"1", "2"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(f.reqs) != 1 || f.reqs[0].GetMode() != tt.want {
				t.Fatalf("requests = %v, want one with mode %v", f.reqs, tt.want)
			}

			if !got["1"] || got["2"] || len(got) != 1 {
				t.Fatalf("result = %v, want only 1", got)
			}
		})
	}
}

func TestClient_ForwardsTheCallersToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ctx       func() context.Context
		wantToken string
		wantErr   error
	}{
		{name: "single token", ctx: func() context.Context { return withTokens("test-token-operator") }, wantToken: "test-token-operator"},
		{name: "last value wins, as im-account reads it", ctx: func() context.Context { return withTokens("test-token-first", "test-token-last") }, wantToken: "test-token-last"},
		{name: "empty last value fails closed", ctx: func() context.Context { return withTokens("test-token-first", "") }, wantErr: ErrNoAccessToken},
		{name: "no token", ctx: func() context.Context { return withTokens() }, wantErr: ErrNoAccessToken},
		{name: "no incoming metadata", ctx: context.Background, wantErr: ErrNoAccessToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := &fakeCRM{}

			_, err := newTestClient(f).VisibleContacts(tt.ctx(), []string{"1"})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}

				if len(f.reqs) != 0 {
					t.Fatal("the crm was called without a token")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := f.tokens[0]; !slices.Equal(got, []string{tt.wantToken}) {
				t.Fatalf("forwarded tokens = %v, want exactly [%s]", got, tt.wantToken)
			}
		})
	}
}

func TestClient_Batches(t *testing.T) {
	t.Parallel()

	ids := make([]string, 0, 2*batchSize+1)
	allowed := map[string]bool{}

	for i := range 2*batchSize + 1 {
		id := strconv.Itoa(i + 1)
		ids = append(ids, id)

		if i%2 == 0 {
			allowed[id] = true
		}
	}

	f := &fakeCRM{allowed: allowed}

	got, err := newTestClient(f).VisibleContacts(withTokens("test-token-operator"), ids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(f.reqs) != 3 {
		t.Fatalf("requests = %d, want 3 batches", len(f.reqs))
	}

	for i, want := range []int{batchSize, batchSize, 1} {
		if n := len(f.reqs[i].GetId()); n != want || f.reqs[i].GetSize() != batchSize {
			t.Fatalf("batch %d: %d ids, size %d; want %d ids, size %d", i, n, f.reqs[i].GetSize(), want, batchSize)
		}
	}

	if len(got) != len(allowed) {
		t.Fatalf("visible = %d, want %d", len(got), len(allowed))
	}

	for id := range allowed {
		if !got[id] {
			t.Fatalf("visible misses %s", id)
		}
	}
}

func TestClient_NoIDsMakesNoCall(t *testing.T) {
	t.Parallel()

	f := &fakeCRM{}

	got, err := newTestClient(f).WritableContacts(context.Background(), nil)
	if err != nil || len(got) != 0 || len(f.reqs) != 0 {
		t.Fatalf("got %v, err %v, calls %d: want an empty result without a call", got, err, len(f.reqs))
	}
}

func TestClient_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		err          error
		wantNoAccess bool
	}{
		{name: "permission denied: no access to contacts", err: status.Error(codes.PermissionDenied, "denied"), wantNoAccess: true},
		{name: "unavailable is a failure, not a refusal", err: status.Error(codes.Unavailable, "down")},
		{name: "plain error", err: fmt.Errorf("dial")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestClient(&fakeCRM{err: tt.err}).VisibleContacts(withTokens("test-token-operator"), []string{"1"})
			if err == nil || got != nil {
				t.Fatalf("got %v, err %v: want an error and no result", got, err)
			}

			if errors.Is(err, ErrNoCRMAccess) != tt.wantNoAccess {
				t.Fatalf("errors.Is(ErrNoCRMAccess) = %v, want %v (%v)", !tt.wantNoAccess, tt.wantNoAccess, err)
			}

			if !errors.Is(err, tt.err) {
				t.Fatalf("cause lost: %v", err)
			}
		})
	}
}
