package grpc

import (
	"go.uber.org/fx"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	providerv1 "github.com/webitel/im-gateway-service/gen/go/provider/v1"
	grpcsrv "github.com/webitel/im-gateway-service/infra/server/grpc"
)

var Module = fx.Module("grpc",
	fx.Provide(
		NewMessageService,
		NewMessageHistoryService,
		NewThreadService,
		NewThreadPermissionServer,
		NewContactSettingsServer,
	),
	fx.Invoke(
		RegisterMessageService,
		RegisterHistoryMessageService,
		RegisterThreadService,
		RegisterThreadPermissionService,
		RegisterContactSettingsServer,
	),
	fx.Provide(
		NewContactService,
		NewBotService,
		NewAccountService,
		newViaServer,
	),
	fx.Invoke(
		RegisterContactService,
		RegisterBotService,
		RegisterAccountService,
		RegisterViaServer,
	),
	fx.Provide(
		NewFacebookServiceHandler,
		NewGateServiceHandler,
		NewWhatsAppServiceHandler,
		NewMetaAppServiceHandler,
		NewMetaOAuthServiceHandler,
	),
	fx.Invoke(
		RegisterFacebookServiceHandler,
		RegisterGateServiceHandler,
		RegisterWhatsAppServiceHandler,
		RegisterMetaAppServiceHandler,
		RegisterMetaOAuthServiceHandler,
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
func RegisterAccountService(server *grpcsrv.Server, service *AccountService) {
	impb.RegisterAccountServer(server, service)
}

func RegisterThreadService(server *grpcsrv.Server, service *ThreadService) {
	impb.RegisterThreadManagementServer(server, service)
}

func RegisterThreadPermissionService(server *grpcsrv.Server, service *ThreadPermissionServer) {
	impb.RegisterThreadPermissionServer(server, service)
}

func RegisterContactSettingsServer(server *grpcsrv.Server, service *ContactSettingsServer) {
	impb.RegisterContactSettingsManagementServer(server, service)
}

func RegisterViaServer(server *grpcsrv.Server, service *ViaServer) {
	impb.RegisterViasServiceServer(server, service)
}

func RegisterFacebookServiceHandler(server *grpcsrv.Server, h *FacebookServiceHandler) {
	providerv1.RegisterFacebookServiceServer(server.Server, h)
}

func RegisterGateServiceHandler(server *grpcsrv.Server, h *GateServiceHandler) {
	providerv1.RegisterGateServiceServer(server.Server, h)
}

func RegisterWhatsAppServiceHandler(server *grpcsrv.Server, h *WhatsAppServiceHandler) {
	providerv1.RegisterWhatsAppServiceServer(server.Server, h)
}

func RegisterMetaAppServiceHandler(server *grpcsrv.Server, h *MetaAppServiceHandler) {
	providerv1.RegisterMetaAppServiceServer(server.Server, h)
}

func RegisterMetaOAuthServiceHandler(server *grpcsrv.Server, h *MetaOAuthServiceHandler) {
	providerv1.RegisterMetaOAuthServiceServer(server.Server, h)
}
