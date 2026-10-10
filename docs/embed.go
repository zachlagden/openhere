package docs

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte

//go:embed agent-guide.md
var AgentGuide []byte
