package api

import "time"

// The endpoint an assistant talks to, and the budget one key gets there. The
// settings card reads all three numbers off the API rather than repeating them.
const (
	mcpEndpointPath = "/mcp"

	mcpStartsPerHour    = 12
	mcpStartCooldown    = 15 * time.Minute
	mcpItemStartsPerDay = 4
)
