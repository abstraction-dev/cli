package apiclient

// AskRequest is the POST /api/cli/ask body. Field names match the backend
// CLIController.
type AskRequest struct {
	Workspace string `json:"workspace"`
	Question  string `json:"question"`
	// PR optionally scopes the ask to a diff report — a GitHub PR URL or number.
	PR string `json:"pr,omitempty"`
	// Stream selects SSE streaming over a buffered JSON reply.
	Stream bool `json:"stream"`
	// SessionID tags a conversation for future server-side multi-turn context.
	// The backend currently treats each ask independently; sending it is
	// harmless and forward-compatible.
	SessionID string `json:"session_id,omitempty"`
}

// askResponse is the buffered (non-stream) reply.
type askResponse struct {
	Answer string `json:"answer"`
}

// Workspace is one entry from GET /api/workspaces (protojson, camelCase).
type Workspace struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

type listWorkspacesResponse struct {
	Workspaces []Workspace `json:"workspaces"`
}

// StreamHandlers receive decoded frames from a streaming ask.
type StreamHandlers struct {
	// OnOutput receives a chunk of formatted answer text.
	OnOutput func(text string)
	// OnStatus receives a transient progress label (safe to ignore).
	OnStatus func(message string)
}

// SSE frame payloads emitted by the terminal sink.
type outputFrame struct {
	Text string `json:"text"`
}

type messageFrame struct {
	Message string `json:"message"`
}
