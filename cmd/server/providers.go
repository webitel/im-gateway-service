package server

import (
	"context"
	"log/slog"
	"net/url"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/fx"

	"github.com/webitel/webitel-go-kit/infra/discovery"
	otelsdk "github.com/webitel/webitel-go-kit/infra/otel/sdk"
	"github.com/webitel/webitel-go-kit/infra/profiler"
	"github.com/webitel/webitel-go-kit/pkg/depenlog"
	"github.com/webitel/webitel-go-kit/pkg/logger"

	"github.com/webitel/im-gateway-service/config"
	"github.com/webitel/im-gateway-service/internal/model"

	_ "github.com/webitel/webitel-go-kit/infra/discovery/consul"
	_ "github.com/webitel/webitel-go-kit/infra/otel/sdk/log/otlp"
	_ "github.com/webitel/webitel-go-kit/infra/otel/sdk/log/stdout"
	_ "github.com/webitel/webitel-go-kit/infra/otel/sdk/metric/otlp"
	_ "github.com/webitel/webitel-go-kit/infra/otel/sdk/metric/stdout"
	_ "github.com/webitel/webitel-go-kit/infra/otel/sdk/trace/otlp"
	_ "github.com/webitel/webitel-go-kit/infra/otel/sdk/trace/stdout"
)

func ProvideLogger(cfg *config.Config, lc fx.Lifecycle) (*slog.Logger, logger.Logger, error) {
	logSettings := cfg.Log

	if !logSettings.Console && !logSettings.Otel && logSettings.File == "" {
		logSettings.Console = true
	}

	depCfg := depenlog.Config{
		Level:   logSettings.Level,
		JSON:    logSettings.JSON,
		File:    logSettings.File,
		Console: logSettings.Console,
	}

	var opts []depenlog.Option

	if logSettings.Otel {
		service := resource.NewSchemaless(
			semconv.ServiceName(model.ServiceName),
			semconv.ServiceVersion(model.Version),
			semconv.ServiceInstanceID(discovery.GenerateInstanceID(model.ServiceName)),
			semconv.ServiceNamespace(model.ServiceNamespace),
		)

		// The bridge hook fires (synchronously, during Configure) only when an
		// OTel logs exporter is active; otherwise otelHandler stays nil and we
		// fall back to depenlog's console/file output.
		var otelHandler slog.Handler
		shutdown, err := otelsdk.Configure(context.Background(), otelsdk.WithResource(service),
			otelsdk.WithLogBridge(
				func() {
					otelHandler = otelslog.NewHandler("slog")
				},
			),
		)
		if err != nil {
			return nil, nil, err
		}

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return shutdown(ctx)
			},
		})

		if otelHandler != nil {
			opts = append(opts, depenlog.WithHandler(otelHandler))
		}
	}

	l := depenlog.New(depCfg, opts...)

	// depenlog.New calls slog.SetDefault, so slog.Default() is the unified logger.
	return slog.Default(), l, nil
}

func ProvideSD(cfg *config.Config, log *slog.Logger, lc fx.Lifecycle) (discovery.DiscoveryProvider, error) {
	provider, err := discovery.DefaultFactory.CreateProvider(
		discovery.ProviderConsul,
		log,
		cfg.Consul.Addr,
		discovery.WithHeartbeat[discovery.DiscoveryProvider](true),
		discovery.WithTimeout[discovery.DiscoveryProvider](time.Second*30),
	)
	if err != nil {
		return nil, err
	}

	si := new(discovery.ServiceInstance)
	{
		si.Id = discovery.GenerateInstanceID(model.ServiceName)
		si.Name = model.ServiceName
		si.Version = model.Version
		si.Metadata = map[string]string{
			"commit":         model.Commit,
			"commitDate":     model.CommitDate,
			"branch":         model.Branch,
			"buildTimestamp": model.BuildTimestamp,
		}
		si.Endpoints = []string{(&url.URL{Scheme: "grpc", Host: cfg.Service.Addr}).String()}
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := provider.Register(ctx, si); err != nil {
				return err
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := provider.Deregister(ctx, si); err != nil {
				return err
			}

			return nil
		},
	})

	return provider, nil
}

func ProvideProfile(cfg *config.Config) profiler.Config {
	return profiler.Config{
		Addr:                 cfg.Profiler.Addr,
		MutexProfileFraction: cfg.Profiler.MutexFraction,
		BlockProfileRate:     cfg.Profiler.BlockRate,
	}
}
