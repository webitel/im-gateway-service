package mapper

import (
	impb "github.com/webitel/im-gateway-service/gen/go/gateway/v1"
	"github.com/webitel/im-gateway-service/internal/service/dto"
)

func MapToCreateBotRequest(in *impb.CreateBotRequest) *dto.CreateBotRequest {
	if in == nil {
		return nil
	}

	return &dto.CreateBotRequest{
		Username: in.GetUsername(),
		Name:     in.GetName(),
		SchemaID: in.GetSchemaId(),
		Metadata: in.GetMetadata(),
	}
}

func MapToUpdateBotRequest(in *impb.UpdateBotRequest) *dto.UpdateBotRequest {
	if in == nil {
		return nil
	}

	return &dto.UpdateBotRequest{
		ID:       in.GetId(),
		Username: in.GetUsername(),
		Name:     in.GetName(),
		SchemaID: in.GetSchemaId(),
		Metadata: in.GetMetadata(),
		Fields:   in.FieldMask.GetPaths(),
	}
}

func MapToDeleteBotRequest(in *impb.DeleteBotRequest) *dto.DeleteBotRequest {
	if in == nil {
		return nil
	}

	return &dto.DeleteBotRequest{
		ID: in.GetId(),
	}
}

func MapToBot(in *dto.Bot) *impb.Bot {
	return &impb.Bot{
		Id:       in.ID,
		Name:     in.Name,
		Username: in.Username,
		SchemaId: in.SchemaID,
		Metadata: in.Metadata,
	}
}
