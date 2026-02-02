package tls

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"go.uber.org/fx"

	"github.com/webitel/im-gateway-service/config"
)

var Module = fx.Module("tls",
	fx.Provide(
		ProvideTLSConfig,
	))

type Config struct {
	Client *tls.Config
	Server *tls.Config
}

func ProvideTLSConfig(cfg *config.Config) (*Config, error) {
	var (
		connConfig = cfg.Service.Connection
		conf       = &Config{}
		err        error
	)

	if !connConfig.VerifyCerts {
		conf.Server = nil
		conf.Client = nil
		return conf, nil
	}

	conf.Server, err = Load(connConfig.TLS, tls.VerifyClientCertIfGiven)
	if err != nil {
		return nil, err
	}

	conf.Client, err = Load(connConfig.Client, tls.RequireAndVerifyClientCert)
	if err != nil {
		return nil, err
	}

	return conf, nil
}

func Load(connConfig config.TLSConfig, authType tls.ClientAuthType) (*tls.Config, error) {
	if connConfig.Cert == "" || connConfig.Key == "" {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(connConfig.Cert, connConfig.Key)
	if err != nil {
		return nil, err
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   authType,
	}

	if connConfig.CA != "" {
		caCert, err := os.ReadFile(connConfig.CA)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		tlsConf.ClientCAs = caCertPool
		tlsConf.RootCAs = caCertPool
	}

	return tlsConf, nil
}
