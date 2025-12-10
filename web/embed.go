package web

import "embed"

//go:embed index.html app.js style.css metrics.html metrics.js
var StaticFiles embed.FS
