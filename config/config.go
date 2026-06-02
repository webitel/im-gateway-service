package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/pflag"
	"github.com/webitel/webitel-go-kit/appconfig"
)

const minUploadChunkSize = 512

type Config struct {
	Service  ServiceConfig      `mapstructure:"service"`
	Log      appconfig.Log      `mapstructure:"log"`
	Postgres appconfig.Postgres `mapstructure:"postgres"`
	Redis    appconfig.Redis    `mapstructure:"redis"`
	Consul   appconfig.Consul   `mapstructure:"consul"`
	Pubsub   appconfig.Pubsub   `mapstructure:"pubsub"`
	Profiler appconfig.Profiler `mapstructure:"profiler"`
}

type ServiceConfig struct {
	ID              string     `mapstructure:"id"`
	GRPC            GRPCConfig `mapstructure:"grpc"`
	HTTP            HTTPConfig `mapstructure:"http"`
	MaxUploadSize   int64      `mapstructure:"max_upload_size"`
	UploadChunkSize int        `mapstructure:"upload_chunk_size"`
}

type GRPCConfig struct {
	Addr       string             `mapstructure:"addr"`
	Connection appconfig.GRPCConn `mapstructure:"conn"`
}

type HTTPConfig struct {
	Addr        string        `mapstructure:"addr"`
	VerifyCerts bool          `mapstructure:"verify_certs"`
	TLS         appconfig.TLS `mapstructure:"tls"`
	CORS        CORSConfig    `mapstructure:"cors"`
}

type CORSConfig struct {
	AllowedOrigins string `mapstructure:"allowed_origins"`
}

func LoadConfig() (*Config, error) {
	loader := appconfig.NewLoader(appconfig.Sections{
		Log:      true,
		Postgres: true,
		Redis:    true,
		Consul:   true,
		Pubsub:   true,
		Profiler: true,
	})
	loader.RegisterFlags(pflag.CommandLine)
	registerServiceFlags()
	pflag.Parse()

	cfg := &Config{}
	if err := loader.Load(pflag.CommandLine, cfg); err != nil {
		return nil, err
	}

	loader.Watch(func(e fsnotify.Event) {
		slog.Info("config file changed", "name", e.Name)
		newCfg := &Config{}
		if err := loader.Viper().Unmarshal(newCfg); err != nil {
			slog.Error("config reload: unmarshal failed", "error", err)
			return
		}
		if err := newCfg.validate(); err != nil {
			slog.Error("config reload: validation failed", "error", err)
			return
		}
		*cfg = *newCfg
		slog.Info("config reloaded")
	})

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func registerServiceFlags() {
	pflag.String("service.id", "", "Service instance ID (required)")
	pflag.Int64("service.max_upload_size", 0, "Max upload body size in bytes (0 = unlimited)")
	pflag.Int("service.upload_chunk_size", 4096, "Upload chunk size in bytes for streaming uploads to storage")

	pflag.String("service.grpc.addr", "localhost:8080", "gRPC listen address")
	pflag.Bool("service.grpc.conn.verify_certs", true, "Verify TLS certificates on outbound gRPC connections")
	pflag.String("service.grpc.conn.ca", "", "CA certificate path")
	pflag.String("service.grpc.conn.cert", "", "Server certificate path")
	pflag.String("service.grpc.conn.key", "", "Server certificate key path")
	pflag.String("service.grpc.conn.client.ca", "", "Client CA certificate path")
	pflag.String("service.grpc.conn.client.cert", "", "Client certificate path")
	pflag.String("service.grpc.conn.client.key", "", "Client certificate key path")

	pflag.String("service.http.addr", "localhost:8081", "HTTP listen address")
	pflag.Bool("service.http.verify_certs", false, "Enable TLS for HTTP")
	pflag.String("service.http.tls.ca", "", "HTTP CA certificate path")
	pflag.String("service.http.tls.cert", "", "HTTP certificate path")
	pflag.String("service.http.tls.key", "", "HTTP certificate key path")
	pflag.String("service.http.cors.allowed_origins", "*", "Allowed CORS origins")
}

func (c *Config) validate() error {
	if c.Service.ID == "" {
		return fmt.Errorf("config: service.id is required (use --service.id or SERVICE_ID env)")
	}
	if c.Service.GRPC.Addr == "" {
		return fmt.Errorf("config: service.grpc.addr is required")
	}
	if c.Service.UploadChunkSize < minUploadChunkSize {
		return fmt.Errorf("config: service.upload_chunk_size must be >= %d bytes (mime sniff window)", minUploadChunkSize)
	}
	if err := appconfig.ValidateGRPCConn("service.grpc.conn", c.Service.GRPC.Connection); err != nil {
		return err
	}
	if c.Service.HTTP.VerifyCerts {
		if err := appconfig.ValidateTLS("service.http.tls", c.Service.HTTP.TLS); err != nil {
			return err
		}
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Postgres.DSN == "" {
		return fmt.Errorf("config: postgres.dsn is required (use --postgres.dsn or POSTGRES_DSN env)")
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("config: redis.addr is required")
	}
	if c.Consul.Addr == "" {
		return fmt.Errorf("config: consul.addr is required")
	}
	if c.Pubsub.URL == "" {
		return fmt.Errorf("config: pubsub.url is required (use --pubsub.url or PUBSUB_URL env)")
	}
	if !strings.HasPrefix(c.Pubsub.URL, "amqp://") && !strings.HasPrefix(c.Pubsub.URL, "amqps://") {
		return fmt.Errorf("config: pubsub.url must start with amqp:// or amqps://")
	}
	return nil
}
