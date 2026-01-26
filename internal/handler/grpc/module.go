package grpc

import (
	impb "github.com/webitel/im-gateway-service/gen/go/thread/v1"
	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
	"go.uber.org/fx"
)

var Module = fx.Module("grpc",
	fx.Provide(
		NewMessageService,
	),
	fx.Invoke(RegisterMessageService),
)

func RegisterMessageService(
	server *grpcsrv.Server,
	service *MessageService,
) {
	impb.RegisterMessageServer(server.Server, service)
}
