package grpc

import (
	"go.uber.org/fx"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
)

var Module = fx.Module("grpc",
	fx.Provide(
		NewMessageService,
	),
	fx.Provide(
		NewContactService,
	),
	fx.Invoke(RegisterMessageService),
	fx.Invoke(RegisterContactService),
)

func RegisterMessageService(
	server *grpcsrv.Server,
	service *MessageService,
) {
	impb.RegisterMessageServer(server.Server, service)
}

func RegisterContactService(
	server *grpcsrv.Server,
	service *ContactService,
) {
	impb.RegisterContactsServer(server.Server, service)
}
