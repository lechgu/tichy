package commands

import (
	"github.com/lechgu/tichy/internal/commands/chat"
	"github.com/lechgu/tichy/internal/commands/db"
	"github.com/lechgu/tichy/internal/commands/ingest"
	"github.com/lechgu/tichy/internal/commands/serve"
	"github.com/lechgu/tichy/internal/commands/tests"
	"github.com/lechgu/tichy/internal/commands/version"
	"github.com/lechgu/tichy/internal/meta"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   meta.Name,
	Short: meta.Name,
	Long:  meta.Name,
}

func init() {
	Cmd.AddCommand(version.Cmd)
	Cmd.AddCommand(db.Cmd)
	Cmd.AddCommand(ingest.Cmd)
	Cmd.AddCommand(chat.Cmd)
	Cmd.AddCommand(serve.Cmd)
	Cmd.AddCommand(tests.TestsCmd)
}
