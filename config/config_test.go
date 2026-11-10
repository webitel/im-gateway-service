package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"

	"github.com/webitel/webitel-go-kit/appconfig/secret"

	"github.com/webitel/im-gateway-service/config"
)

// startAsSystemdUnit makes Load* see what a systemd unit with
// LoadCredential*= sees: no secret env vars, only $CREDENTIALS_DIRECTORY
// holding the given credentials. It also resets the global flag set, since
// the loaders register into pflag.CommandLine and parse os.Args.
func startAsSystemdUnit(t *testing.T, creds map[string]string) string {
	t.Helper()

	for _, env := range []string{
		"CONFIG_FILE",
		"POSTGRES_DSN", "POSTGRES_DSN_FILE",
		"REDIS_PASSWORD", "REDIS_PASSWORD_FILE",
		"PUBSUB_URL", "PUBSUB_URL_FILE",
		"SERVICE_HTTP_TLS_KEY",
		"SERVICE_CONN_KEY", "SERVICE_CONN_CLIENT_KEY",
	} {
		t.Setenv(env, "")
		os.Unsetenv(env)
	}

	dir := t.TempDir()
	for name, value := range creds {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value+"\n"), 0o400); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("CREDENTIALS_DIRECTORY", dir)

	args, flags := os.Args, pflag.CommandLine
	os.Args = []string{"server"}
	pflag.CommandLine = pflag.NewFlagSet("server", pflag.ContinueOnError)

	t.Cleanup(func() { os.Args, pflag.CommandLine = args, flags })

	return dir
}

func TestLoadConfigSecretsFromCredentials(t *testing.T) {
	dir := startAsSystemdUnit(t, map[string]string{
		"postgres.dsn":         "postgres://db.example.com/im_test",
		"redis.password":       "test-redis-password",
		"pubsub.url":           "amqp://mq.example.com/",
		"service.conn.key":     "test-only-key",
		"service.http.tls.key": "test-only-http-key",
	})
	t.Setenv("SERVICE_CONN_CA", "/etc/test/ca.crt")
	t.Setenv("SERVICE_CONN_CERT", "/etc/test/service.crt")
	t.Setenv("SERVICE_HTTP_VERIFY_CERTS", "true")
	t.Setenv("SERVICE_HTTP_TLS_CA", "/etc/test/ca.crt")
	t.Setenv("SERVICE_HTTP_TLS_CERT", "/etc/test/http.crt")

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	for name, got := range map[string]string{
		"postgres.dsn":         cfg.Postgres.DSN,
		"redis.password":       cfg.Redis.Password,
		"pubsub.url":           cfg.Pubsub.URL,
		"service.conn.key":     cfg.Service.Connection.Key,
		"service.http.tls.key": cfg.Service.HTTP.TLS.Key,
	} {
		want := map[string]string{
			"postgres.dsn":         "postgres://db.example.com/im_test",
			"redis.password":       "test-redis-password",
			"pubsub.url":           "amqp://mq.example.com/",
			"service.conn.key":     filepath.Join(dir, "service.conn.key"),
			"service.http.tls.key": filepath.Join(dir, "service.http.tls.key"),
		}[name]
		if got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

func TestLoadConfigPlainSecretShadowingCredentialFails(t *testing.T) {
	startAsSystemdUnit(t, map[string]string{"postgres.dsn": "postgres://db.example.com/new"})
	t.Setenv("POSTGRES_DSN", "postgres://db.example.com/old")

	_, err := config.LoadConfig()
	if !errors.Is(err, secret.ErrConflict) {
		t.Fatalf("want secret.ErrConflict, got %v", err)
	}
}
