// Package version holds build identity shared by the client, panel and release
// packager. Release builds override Version through -ldflags -X.
package version

// Version is the project release version. Keep it in sync with the release tag
// and the assets produced by cmd/tes3mp-easy-pack.
var Version = "0.1.0"
