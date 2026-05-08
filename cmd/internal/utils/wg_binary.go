//go:build embed_wireguard
// +build embed_wireguard

package utils

import _ "embed"

//go:embed wg-embedded
var EmbeddedWgBinary []byte
