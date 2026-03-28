package storage

import (
	"context"
	"fmt"
	"log/slog"

	storagev1 "github.com/webitel/im-gateway-service/gen/go/storage/v1"
	webitel "github.com/webitel/im-gateway-service/infra/client"
	infratls "github.com/webitel/im-gateway-service/infra/tls"
	"github.com/webitel/webitel-go-kit/infra/discovery"
	rpc "github.com/webitel/webitel-go-kit/infra/transport/gRPC"
	"google.golang.org/grpc"
)

const ServiceName string = "storage"

type Client struct {
	logger *slog.Logger
	rpc    *rpc.Client[storagev1.FileServiceClient]
}

func New(logger *slog.Logger, dp discovery.DiscoveryProvider, tls *infratls.Config) (*Client, error) {
	factory := func(conn *grpc.ClientConn) storagev1.FileServiceClient {
		return storagev1.NewFileServiceClient(conn)
	}

	c, err := webitel.New(logger, dp, ServiceName, tls, factory)
	if err != nil {
		return nil, fmt.Errorf("[storage-client] initialization failed: %w", err)
	}

	return &Client{logger: logger, rpc: c}, nil
}

// DownloadFile initiates a server-streaming download. The returned stream must be consumed
// and closed by the caller. The stream is bound to the provided context.
func (c *Client) DownloadFile(ctx context.Context, req *storagev1.DownloadFileRequest) (storagev1.FileService_DownloadFileClient, error) {
	var stream storagev1.FileService_DownloadFileClient

	err := c.rpc.Execute(ctx, func(api storagev1.FileServiceClient) error {
		var err error
		stream, err = api.DownloadFile(ctx, req)
		return err
	})

	return stream, err
}

func (c *Client) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}