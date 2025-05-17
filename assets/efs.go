package assets

import (
	"embed"
)

//go:embed "emails"
var EmbeddedFiles embed.FS
