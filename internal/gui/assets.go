package gui

import (
	_ "embed"
	"strings"
)

//go:embed assets/index.html
var indexHTML string

//go:embed assets/app.css
var appCSS string

//go:embed assets/app.js
var appJS string

// assetsHTML returns the single-page UI with CSS and JS inlined.
func assetsHTML() string {
	html := strings.Replace(indexHTML, "<!--{{CSS}}-->", appCSS, 1)
	return strings.Replace(html, "<!--{{JS}}-->", appJS, 1)
}
