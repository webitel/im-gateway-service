package grpc

import (
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
	"go.uber.org/fx"
)

var Module = fx.Module("grpc",
	fx.Provide(
		NewMessageService,
		NewMessageHistoryService,
	),
	fx.Invoke(
		RegisterMessageService,
		RegisterHistoryMessageService,
	),
)

func RegisterMessageService(
	server *grpcsrv.Server,
	service *MessageService,
) {
	impb.RegisterMessageServer(server.Server, service)
}

func RegisterHistoryMessageService(server *grpcsrv.Server, service *MessageHistoryService) {
	impb.RegisterMessageHistoryServer(server, service)
}