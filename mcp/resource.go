package mcp

// Resource represents an MCP resource exposed by a server.
type Resource struct {
	// URI is the resource identifier (e.g. "file:///path" or "db://table").
	URI string

	// Name is a human-readable name for the resource.
	Name string

	// Description explains what the resource contains.
	Description string

	// MIMEType is the content type (e.g. "text/plain", "application/json").
	MIMEType string
}
