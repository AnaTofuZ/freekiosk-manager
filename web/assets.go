package assets

import "embed"

//go:embed templates/*.gohtml generated/*.tmpl static/* locales/*.json
var Files embed.FS
