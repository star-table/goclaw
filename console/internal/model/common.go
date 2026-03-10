package model

// VersionInfo represents version information
type VersionInfo struct {
	Version string `json:"version"`
}

// RootResponse represents root path response
type RootResponse struct {
	Message string `json:"message"`
	Docs    string `json:"docs"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// SPAFallbackResponse represents SPA fallback response
type SPAFallbackResponse struct {
	Message string `json:"message"`
	Path    string `json:"path"`
}
