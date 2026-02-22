package certs

import (
	"embed"
)

// To embed your CA certificate into the binary at build time:
//
//	cp /path/to/your/ca.crt agent/certs/ca.crt
//	cd agent && go build .
//
// The embedded cert is used when config.broker.caFile is empty.
// If no ca.crt was embedded, the agent falls back to system root CAs.

// Embed the entire certs directory. The "all:" prefix includes hidden files
// and files that would normally be ignored, ensuring the embed always succeeds
// even with only .gitkeep present.
//
//go:embed all:*
var certFS embed.FS

// LoadEmbeddedCA returns the embedded CA certificate, or nil if none was embedded.
func LoadEmbeddedCA() []byte {
	data, err := certFS.ReadFile("ca.crt")
	if err != nil {
		return nil
	}
	return data
}
