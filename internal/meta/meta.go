package meta

import (
	"fmt"
	"runtime/debug"
)

const (
	Name = "tichy"

	Major = 2025
	Minor = 11
	Patch = 1
)

var (
	Version = fmt.Sprintf("%d.%d.%d", Major, Minor, Patch)
	Commit  = getCommit()
)

func getCommit() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return ""
}
