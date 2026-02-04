package imthread

import (
	"context"
	"log/slog"

	threadv1 "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"
	"google.golang.org/grpc"
)

type ThreadClient struct {
	logger *slog.Logger

	rpc *rpc.Client[threadv1.ThreadManagementClient]
	converter mapper.ThreadConverter
}

func NewThreadClient(logger *slog.Logger, discovery discovery.DiscoveryProvider, tls *infratls.Config, converter mapper.ThreadConverter) (*ThreadClient, error) {
	log := logger.With(slog.String("component", "im-thread-management-client"))
	
	factory := func(conn *grpc.ClientConn) threadv1.ThreadManagementClient {
		return threadv1.NewThreadManagementClient(conn)
	}

	c, err := webitel.New(log, discovery, ServiceName, tls, factory)
	if err != nil {
		log.Error("initialization failed", slog.Any("error", err))
		return nil, err
	} 

	return &ThreadClient{
		logger:    log,
		rpc:       c,
		converter: converter,
	}, err
}

func (c *ThreadClient) Search(ctx context.Context, searchQuery *threadv1.ThreadSearchRequest) (*threadv1.SearchThreadResponse, error) {
	log := c.logger.With(
		slog.Any("domain_ids", searchQuery.DomainIds),
		slog.Any("member_ids", searchQuery.MemberIds),
	)

	var (
		err error
		resp *threadv1.SearchThreadResponse
	)

	err = c.rpc.Execute(ctx, func(tmc threadv1.ThreadManagementClient) error {
		resp, err = tmc.Search(ctx, searchQuery)
		return err
	})

	if err != nil {
		log.Error("failed to fetch thread information from provider", slog.Any("error", err))
		return nil, err
	}

	return resp, nil
}

func (c *ThreadClient) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}

	return nil
}