package db

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "db",
	Short: "Database commands",
}

func init() {
	Cmd.AddCommand(up)
	Cmd.AddCommand(down)
	Cmd.AddCommand(reset)
	Cmd.AddCommand(status)
}
