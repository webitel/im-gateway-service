package grpc

import (
	"go.uber.org/fx"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
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
	fx.Provide(
		NewContactService,
		NewBotService,
	),
	fx.Invoke(
		RegisterContactService,
		RegisterBotService,
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

func RegisterContactService(server *grpcsrv.Server, service *ContactService) {
	impb.RegisterContactsServer(server, service)
}

func RegisterBotService(server *grpcsrv.Server, service *BotService) {
	impb.RegisterBotsServer(server, service)
}
