package storage

import (
	"context"
	"fmt"
	"io"
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

// SafeUploadFile opens a bidirectional streaming upload. The caller must call the
// returned release function when the stream is no longer needed to return the
// underlying connection to the pool. The stream is bound to the provided context.
func (c *Client) SafeUploadFile(ctx context.Context) (storagev1.FileService_SafeUploadFileClient, func(), error) {
	api, release, err := c.rpc.GetAPI(ctx)
	if err != nil {
		return nil, nil, err
	}

	stream, err := api.SafeUploadFile(ctx)
	if err != nil {
		_ = release()
		return nil, nil, err
	}

	return stream, func() { _ = release() }, nil
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

func (c *Client) GetUploadInfo(ctx context.Context, uploadID string) (int64, error) {
	stream, release, err := c.SafeUploadFile(ctx)
	if err != nil {
		return 0, err
	}
	defer release()

	if err = stream.Send(&storagev1.SafeUploadFileRequest{
		Data: &storagev1.SafeUploadFileRequest_UploadId{UploadId: uploadID},
	}); err != nil {
		return 0, err
	}

	if err = stream.CloseSend(); err != nil {
		return 0, err
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return 0, fmt.Errorf("upload session not found: %s", uploadID)
		}
		if err != nil {
			return 0, err
		}
		if part := msg.GetPart(); part != nil {
			return part.GetSize(), nil
		}
		if prog := msg.GetProgress(); prog != nil {
			return prog.GetUploaded(), nil
		}
	}
}

func (c *Client) Close() error {
	if c.rpc != nil {
		return c.rpc.Close()
	}
	return nil
}
