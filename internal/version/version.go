// Package version holds build identity of the shipped components. Release
// builds override the version of the component they build through -ldflags -X;
// the values below are the fallbacks used by local builds.
package version

// Client is the desktop client version. It matches the client-v<version> tag
// and the client archives published with that GitHub release.
var Client = "0.2.6"

// Server is the server release version: ops, the panel and the archive built by
// cmd/tes3mp-easy-pack. It matches the server-v<version> tag.
var Server = "0.3.1"
