// Package buildinfo holds build-time metadata stamped via -ldflags.
package buildinfo

// Version is set at build time via
// -ldflags "-X github.com/belak/btta/internal/buildinfo.Version=...".
// When empty, versionx.Get falls back to VCS build info.
var Version string
