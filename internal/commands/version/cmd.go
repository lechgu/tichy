package version

import (
	"fmt"
	"runtime"

	"github.com/lechgu/tichy/internal/meta"
	"github.com/spf13/cobra"
)

var verbose bool

var Cmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE:  doVersion,
}

func init() {
	Cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show verbose version information")
}

func doVersion(cmd *cobra.Command, args []string) error {
	fmt.Printf("%s (%s/%s)\n", meta.Version, runtime.GOOS, runtime.GOARCH)
	if verbose && meta.Commit != "" {
		fmt.Printf("commit: %s\n", meta.Commit)
	}
	return nil
}
