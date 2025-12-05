package serve

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/lechgu/tichy/internal/injectors"
	"github.com/lechgu/tichy/internal/servers"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the OpenAI-compatible API server",
	RunE:  doServe,
}

func doServe(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	//server, err := do.Invoke[*servers.Server](injectors.Default)
	server, err := do.Invoke[servers.WebServer](injectors.Default)
	if err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	return server.Run(ctx)
}
