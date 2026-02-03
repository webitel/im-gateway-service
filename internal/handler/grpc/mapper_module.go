package grpc

import (
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper"
	"github.com/webitel/im-gateway-service/internal/handler/grpc/mapper/generated"
	"go.uber.org/fx"
)

var MapperModule = fx.Module("mapper",
	fx.Provide(
		fx.Annotate(
			func() *generated.ThreadConverterImpl {
				return new(generated.ThreadConverterImpl)
			},
			fx.As(new(mapper.ThreadConverter)),
		),
	),
)