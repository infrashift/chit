package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPISpec []byte // semscope: allow(NASA-P10-06) go:embed requires a package variable
