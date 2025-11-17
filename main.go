package main

import (
	"github.com/lechgu/tichy/internal/commands"
	"github.com/spf13/cobra"
)

func main() {
	cobra.CheckErr(commands.Cmd.Execute())
}
