package grpc

import (
	"context"
	"log/slog"

	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/service"
)

var _ impb.BotsServer = (*BotService)(nil)

type BotService struct {
	impb.UnimplementedBotsServer

	logger *slog.Logger
	botter service.Botter
}

func (b *BotService) CreateBot(ctx context.Context, request *impb.CreateBotRequest) (*impb.Bot, error) {
	bot, err := b.botter.CreateBot(ctx, mapper.MapToCreateBotRequest(request))
	if err != nil {
		return nil, err
	}

	return mapper.MapToBot(bot), nil
}

func (b *BotService) DeleteBot(ctx context.Context, request *impb.DeleteBotRequest) (*impb.Bot, error) {
	bot, err := b.botter.DeleteBot(ctx, mapper.MapToDeleteBotRequest(request))
	if err != nil {
		return nil, err
	}

	return mapper.MapToBot(bot), nil
}

func NewBotService(logger *slog.Logger, botter service.Botter) *BotService {
	return &BotService{
		logger: logger,
		botter: botter,
	}
}
