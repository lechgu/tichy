package tests

import (
	"github.com/lechgu/tichy/internal/commands/tests/evaluate"
	"github.com/lechgu/tichy/internal/commands/tests/generate"
	"github.com/spf13/cobra"
)

var TestsCmd = &cobra.Command{
	Use:   "tests",
	Short: "Test generation and evaluation commands",
}

func init() {
	TestsCmd.AddCommand(generate.Cmd)
	TestsCmd.AddCommand(evaluate.Cmd)
}
