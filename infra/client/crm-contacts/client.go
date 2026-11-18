// Package crmcontacts checks which Webitel CRM contacts the caller may read or write.
// It calls webitel.go (Consul name go.webitel.app) with the caller's own
// Webitel access token, so the CRM applies its RBAC and record ACL.
package crmcontacts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"

	crmpb "github.com/webitel/im-gateway-service/gen/go/webitel/contacts"
	webitel "github.com/webitel/im-gateway-service/infra/client"
)

const (
	// ServiceName is the Consul name of webitel.go; it serves gRPC in plaintext inside the cluster.
	ServiceName = "go.webitel.app"
	// accessTokenHeader carries the caller's Webitel token.
	accessTokenHeader = "x-webitel-access"
	// batchSize bounds the ids of one CRM search.
	batchSize = 100
)

// ErrNoAccessToken means the caller did not authenticate with a Webitel token,
// so the CRM cannot tell what it may read.
var ErrNoAccessToken = errors.New("caller has no webitel access token")

// ErrNoCRMAccess means the CRM refused the caller contacts altogether: the
// caller's role has no access to the Contacts section.
var ErrNoCRMAccess = errors.New("caller has no access to crm contacts")

type Client struct {
	logger   *slog.Logger
	contacts *rpc.Client[crmpb.ContactsClient]
	// search runs one CRM contact search; it is the seam tests replace.
	search func(ctx context.Context, req *crmpb.SearchContactsRequest) (*crmpb.ContactList, error)
}

func New(logger *slog.Logger, dp discovery.DiscoveryProvider) (*Client, error) {
	c, err := webitel.New(logger, dp, ServiceName, nil, func(cc *grpc.ClientConn) crmpb.ContactsClient { return crmpb.NewContactsClient(cc) })
	if err != nil {
		return nil, fmt.Errorf("[crm-contacts-client] initialization failed: %w", err)
	}

	client := &Client{logger: logger.With(slog.String("component", "crm-contacts-client")), contacts: c}
	client.search = client.searchContacts

	return client, nil
}

// VisibleContacts returns the subset of ids the caller may read.
func (c *Client) VisibleContacts(ctx context.Context, ids []string) (map[string]bool, error) {
	return c.contactsWith(ctx, ids, crmpb.AccessMode_READ)
}

// WritableContacts returns the subset of ids the caller may change.
func (c *Client) WritableContacts(ctx context.Context, ids []string) (map[string]bool, error) {
	return c.contactsWith(ctx, ids, crmpb.AccessMode_WRITE)
}

func (c *Client) contactsWith(ctx context.Context, ids []string, mode crmpb.AccessMode) (map[string]bool, error) {
	visible := make(map[string]bool, len(ids))
	if len(ids) == 0 {
		return visible, nil
	}

	token := incomingToken(ctx)
	if token == "" {
		return nil, ErrNoAccessToken
	}

	outCtx := metadata.AppendToOutgoingContext(ctx, accessTokenHeader, token)

	for batch := range slices.Chunk(ids, batchSize) {
		resp, err := c.search(outCtx, &crmpb.SearchContactsRequest{
			Id:     batch,
			Fields: []string{"id"},
			Size:   batchSize,
			Page:   1,
			Mode:   mode,
		})
		if status.Code(err) == codes.PermissionDenied {
			return nil, fmt.Errorf("%w: %w", ErrNoCRMAccess, err)
		}

		if err != nil {
			return nil, fmt.Errorf("checking crm contact access: %w", err)
		}

		for _, ct := range resp.GetData() {
			visible[ct.GetId()] = true
		}
	}

	return visible, nil
}

func (c *Client) searchContacts(ctx context.Context, req *crmpb.SearchContactsRequest) (*crmpb.ContactList, error) {
	var resp *crmpb.ContactList

	err := c.contacts.Execute(ctx, func(api crmpb.ContactsClient) error {
		var err error

		resp, err = api.SearchContacts(ctx, req)

		return err
	})

	return resp, err
}

func (c *Client) Close() error {
	return c.contacts.Close()
}

func incomingToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// The last value, as im-account reads it when it resolves the caller, so
	// the CRM rights checked here are the authenticated caller's.
	if v := md.Get(accessTokenHeader); len(v) > 0 {
		return v[len(v)-1]
	}

	return ""
}
