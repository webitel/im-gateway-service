package cmd

import (
	"os"

	"github.com/urfave/cli/v2"
	"github.com/webitel/im-gateway-service/cmd/server"
	"github.com/webitel/im-gateway-service/internal/model"
)

func Run() error {
	app := &cli.App{
		Name:  model.ServiceName,
		Usage: "Microservice for Webitel platform",
		Commands: []*cli.Command{
			server.CMD(),
		},
	}

	return app.Run(os.Args)
}
