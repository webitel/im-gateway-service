package service

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"service",

	fx.Provide(
		// Domain services
		fx.Annotate(
			NewMessageService,
			fx.As(new(Messenger)),
		),

		fx.Annotate(
			NewAuthService,
			fx.As(new(Auther)),
		),

		fx.Annotate(
			NewContactService,
			fx.As(new(Contacter)),
		),

		fx.Annotate(
			NewMessageHistory,
			fx.As(new(MessageHistorySearcher)),
		),

		fx.Annotate(
			NewBotService,
			fx.As(new(Botter)),
		),

		fx.Annotate(
			NewThread,
			fx.As(new(ThreadSearcher)),
		),
	),
)
