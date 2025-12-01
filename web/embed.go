package web

import "embed"

//go:embed index.html app.js style.css
var StaticFiles embed.FS
