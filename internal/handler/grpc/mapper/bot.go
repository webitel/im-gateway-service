package mapper

import (
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

// goverter:converter
// goverter:matchIgnoreCase
// goverter:output:file ./generated/bot.go
type BotMapperV1 interface {
	// goverter:map SchemaId SchemaID
	ToCreateBotRequest(source *impb.CreateBotRequest) *dto.CreateBotRequest
	// goverter:map FieldMask.Paths Fields
	// goverter:useZeroValueOnPointerInconsistency
	ToUpdateBotRequest(in *impb.UpdateBotRequest) *dto.UpdateBotRequest

	ToDeleteBotRequest(in *impb.DeleteBotRequest) *dto.DeleteBotRequest
	// goverter:ignoreUnexported
	ToBot(in *dto.Bot) *impb.Bot
}
