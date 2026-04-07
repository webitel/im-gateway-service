package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	Service  ServiceConfig  `mapstructure:"service"`
	Log      LogConfig      `mapstructure:"log"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Consul   ConsulConfig   `mapstructure:"consul"`
	Pubsub   PubsubConfig   `mapstructure:"pubsub"`
	Profiler ProfilerConfig `mapstructure:"profiler"`
}

type ServiceConfig struct {
	Id            string     `mapstructure:"id"`
	GRPC          GRPCConfig `mapstructure:"grpc"`
	HTTP          HTTPConfig `mapstructure:"http"`
	MaxUploadSize int64      `mapstructure:"max_upload_size"`
}

type GRPCConfig struct {
	Address    string           `mapstructure:"addr"`
	Connection ConnectionConfig `mapstructure:"conn"`
}

type HTTPConfig struct {
	Address     string     `mapstructure:"addr"`
	VerifyCerts bool       `mapstructure:"verify_certs"`
	TLS         TLSConfig  `mapstructure:"tls"`
	CORS        CORSConfig `mapstructure:"cors"`
}

type CORSConfig struct {
	AllowedOrigins string `mapstructure:"allowed_origins"`
}

type ConnectionConfig struct {
	TLS         TLSConfig `mapstructure:",squash"`
	VerifyCerts bool      `mapstructure:"verify_certs"`
	Client      TLSConfig `mapstructure:"client"`
}

type TLSConfig struct {
	CA   string `mapstructure:"ca"`
	Cert string `mapstructure:"cert"`
	Key  string `mapstructure:"key"`
}

type LogConfig struct {
	Level   string `mapstructure:"level"`
	JSON    bool   `mapstructure:"json"`
	Otel    bool   `mapstructure:"otel"`
	File    string `mapstructure:"file"`
	Console bool   `mapstructure:"console"`
}

type PostgresConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type ConsulConfig struct {
	Address string `mapstructure:"addr"`
}

type PubsubConfig struct {
	URL    string `mapstructure:"broker_url"`
	Driver string `mapstructure:"broker_driver"`
}

type ProfilerConfig struct {
	Addr          string `mapstructure:"addr"`
	MutexFraction int    `mapstructure:"mutex_fraction"`
	BlockRate     int    `mapstructure:"block_rate"`
}

func LoadConfig() (*Config, error) {
	defineFlags()
	pflag.Parse()

	viper.AutomaticEnv()

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return nil, err
	}

	cfg := &Config{}

	configFile := viper.GetString("config_file")
	if configFile != "" {
		viper.SetConfigFile(configFile)
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		viper.OnConfigChange(func(e fsnotify.Event) {
			log.Printf("Config file changed: %s", e.Name)

			newCfg := &Config{}
			if err := viper.Unmarshal(newCfg); err != nil {
				log.Printf("Reload error: unable to decode: %v", err)
				return
			}

			if err := newCfg.validate(); err != nil {
				log.Printf("Reload error: invalid config: %v", err)
				return
			}

			*cfg = *newCfg
			log.Println("Config reloaded successfully")
		})

		viper.WatchConfig()
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %v", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defineFlags() {
	pflag.String("config_file", "", "Configuration file (YAML, JSON, etc.)")

	pflag.String("service.id", "", "Service ID")
	pflag.Int64("service.max_upload_size", 0, "Max upload body size in bytes (0 = unlimited)")

	pflag.String("service.grpc.addr", "localhost:8080", "gRPC service address")
	pflag.Bool("service.grpc.conn.verify_certs", true, "Determine whether to verify certificates")
	pflag.String("service.grpc.conn.ca", "", "Server CA certificate path")
	pflag.String("service.grpc.conn.key", "", "Server certificate key path")
	pflag.String("service.grpc.conn.cert", "", "Server certificate path")
	pflag.String("service.grpc.conn.client.ca", "", "Client CA certificate path")
	pflag.String("service.grpc.conn.client.key", "", "Client certificate key path")
	pflag.String("service.grpc.conn.client.cert", "", "Client certificate path")

	pflag.String("service.http.addr", "localhost:8081", "HTTP service address")
	pflag.Bool("service.http.verify_certs", false, "Determine whether to use TLS for HTTP")
	pflag.String("service.http.tls.ca", "", "HTTP CA certificate path")
	pflag.String("service.http.tls.cert", "", "HTTP certificate path")
	pflag.String("service.http.tls.key", "", "HTTP certificate key path")
	pflag.String("service.http.cors.allowed_origins", "*", "Allowed CORS origins")

	pflag.String("log.level", "info", "Log level")
	pflag.Bool("log.json", false, "Log in JSON format")
	pflag.String("log.file", "", "Log file path")
	pflag.Bool("log.console", true, "Enable console logging")
	pflag.Bool("log.otel", false, "Enable OTEL logging")

	pflag.String("postgres.dsn", "", "Postgres DSN")

	pflag.String("redis.addr", "localhost:6379", "Redis address")
	pflag.String("redis.password", "", "Redis password")
	pflag.Int("redis.db", 0, "Redis database number")

	pflag.String("consul.addr", "localhost:8500", "Consul address")

	pflag.String("pubsub.broker_url", "", "PubSub broker URL")
	pflag.String("pubsub.broker_driver", "", "PubSub broker driver")

	pflag.String("profiler.addr", "", "Profiler address")
	pflag.Int("profiler.mutex_fraction", 1, "Profiler mutex fraction")
	pflag.Int("profiler.block_rate", 1, "Profiler block rate")
}

func (c *Config) validate() error {
	if c.Service.Id == "" {
		return fmt.Errorf("config: service.id is required (use --service.id or SERVICE_ID env)")
	}

	if c.Service.GRPC.Address == "" {
		return fmt.Errorf("config: service.grpc.addr is required")
	}

	if err := validateConnectionConfig(c.Service.GRPC.Connection); err != nil {
		return err
	}

	if c.Service.HTTP.VerifyCerts {
		if err := validateTLSConfig("service.http.tls", c.Service.HTTP.TLS); err != nil {
			return err
		}
	}

	if c.Log.Level == "" {
		c.Log.Level = "info"
	}

	if c.Postgres.DSN == "" {
		return fmt.Errorf("config: postgres.dsn is required (use --postgres.dsn or DATA_SOURCE env)")
	}

	if c.Redis.Addr == "" {
		return fmt.Errorf("config: redis.addr is required")
	}

	if c.Consul.Address == "" {
		return fmt.Errorf("config: consul.addr is required")
	}

	if c.Pubsub.URL == "" {
		return fmt.Errorf("config: pubsub.broker_url is required (use --pubsub.broker_url or PUBSUB env)")
	}

	if !strings.HasPrefix(c.Pubsub.URL, "amqp://") && !strings.HasPrefix(c.Pubsub.URL, "amqps://") {
		return fmt.Errorf("config: pubsub.broker_url must start with amqp:// or amqps://")
	}

	return nil
}

func validateConnectionConfig(conn ConnectionConfig) error {
	if conn.VerifyCerts {
		if err := validateTLSConfig("service.grpc.conn", conn.TLS); err != nil {
			return err
		}
	}
	return nil
}

func validateTLSConfig(prefix string, tls TLSConfig) error {
	if tls.CA == "" {
		return fmt.Errorf("config: %s.ca is required when verify_certs is true", prefix)
	}
	if tls.Cert == "" {
		return fmt.Errorf("config: %s.cert is required when verify_certs is true", prefix)
	}
	if tls.Key == "" {
		return fmt.Errorf("config: %s.key is required when verify_certs is true", prefix)
	}
	return nil
}
