package docs

import "embed"

//go:embed openapi.yaml
var openAPIFiles embed.FS

// ReadOpenAPI returns the manually maintained API contract.
func ReadOpenAPI() ([]byte, error) {
	return openAPIFiles.ReadFile("openapi.yaml")
}
