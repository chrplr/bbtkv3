// Interface to [Black Box Toolkit BBTKv3](https://www.blackboxtoolkit.com/bbtkv3.html)
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

package bbtkv3

import (
	"fmt"
	"runtime/debug"
)

// shortBuildLen is the number of characters of the build identifier
// (a git hash) that VersionString reports.
const shortBuildLen = 8

// VersionString renders the version and build identifiers of a command.
//
// Both are normally injected at link time by the Makefile. They are empty in a
// binary produced by "go install", which applies no -ldflags, so in that case
// the values are recovered from the build information that the Go toolchain
// embeds in every module-aware binary. Whatever their origin, the identifiers
// may be shorter than shortBuildLen and must never be sliced blindly.
func VersionString(version, build string) string {
	if version == "" || build == "" {
		infoVersion, infoRevision := buildInfo()
		if version == "" {
			version = infoVersion
		}
		if build == "" {
			build = infoRevision
		}
	}
	if version == "" {
		version = "unknown"
	}
	if build == "" {
		build = "unknown"
	} else if len(build) > shortBuildLen {
		build = build[:shortBuildLen]
	}
	return fmt.Sprintf("Version: %s  Build: %s", version, build)
}

// buildInfo returns the module version and VCS revision recorded by the Go
// toolchain, or empty strings when they are unavailable (as in a binary built
// from a plain "go build" in a working tree).
func buildInfo() (version, revision string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", ""
	}
	version = info.Main.Version
	if version == "(devel)" {
		version = ""
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			revision = setting.Value
			break
		}
	}
	return version, revision
}
