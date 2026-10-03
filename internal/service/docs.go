package service

import _ "embed"

//go:embed openapi.json
var openapiJSON []byte

//go:embed docs.html
var docsHTML []byte
