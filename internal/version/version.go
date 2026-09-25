// Package version names this build of Yogurt.
package version

// Version is the one version number every release shares, stamped at build
// time with -ldflags "-X github.com/gabe-santos/yogurt/internal/version.Version=v1.2.3".
// An unstamped build is "dev".
var Version = "dev"
