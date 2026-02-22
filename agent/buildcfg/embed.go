package buildcfg

import (
	"embed"
)

// To embed your config into the binary at build time:
//
//   cp /path/to/your/config.json agent/buildcfg/config.json
//   cd agent && go build .
//
// The embedded config is used when no --config flag or config file is found.
// CLI flags (--id, --broker, --user, --pass) override embedded values.

//go:embed all:*
var cfgFS embed.FS

// LoadEmbeddedConfig returns the embedded config.json, or nil if none was embedded.
func LoadEmbeddedConfig() []byte {
	data, err := cfgFS.ReadFile("config.json")
	if err != nil {
		return nil
	}
	return data
}
