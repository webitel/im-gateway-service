package imthread

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"

	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"

	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
)

// ContactIdentificationClient reads the CRM contact candidates im-thread
// identified for a thread.
type ContactIdentificationClient struct {
	logger *slog.Logger
	rpc    *rpc.Client[threadv1.ContactIdentificationClient]
}

func NewContactIdentificationClient(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config) (*ContactIdentificationClient, error) {
	log := logger.With(slog.String("component", "im-thread-contact-identification-client"))

	c, err := webitel.New(log, discovery, ServiceName, tls, func(conn *grpc.ClientConn) threadv1.ContactIdentificationClient {
		return threadv1.NewContactIdentificationClient(conn)
	})
	if err != nil {
		return nil, err
	}

	return &ContactIdentificationClient{logger: log, rpc: c}, nil
}

func (c *ContactIdentificationClient) ListCandidates(ctx context.Context, req *threadv1.ListContactCandidatesRequest) (*threadv1.ListContactCandidatesResponse, error) {
	var resp *threadv1.ListContactCandidatesResponse

	err := c.rpc.Execute(ctx, func(api threadv1.ContactIdentificationClient) error {
		var err error

		resp, err = api.ListCandidates(ctx, req)

		return err
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *ContactIdentificationClient) Close() error {
	return c.rpc.Close()
}
