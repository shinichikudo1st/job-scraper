package web

import _ "embed"

//go:embed index.html
var indexHTML []byte

func IndexHTML() []byte {
	return indexHTML
}
