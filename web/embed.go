// Package web — bitta oddiy sahifa (HTML/CSS/JS, build'siz). Fayllar Go binary ichiga joylanadi.
package web

import "embed"

//go:embed index.html style.css app.js
var FS embed.FS
