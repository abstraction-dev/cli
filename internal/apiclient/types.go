package apiclient

// AskRequest is the POST /api/cli/ask body. Field names match the backend
// CLIController.
type AskRequest struct {
	Workspace string `json:"workspace"`
	Question  string `json:"question"`
	// PR optionally scopes the ask to a diff report — a GitHub PR URL or a
	// workspace diff report id.
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

// PRStatusCompleted is the status value a diff-report review reports once it has
// finished processing — the only value that counts as "ready" to scope against.
// The backend serialises the report's raw job-status string (PENDING /
// IN_PROGRESS / COMPLETED / FAILED / CANCELLED), not a JOB_STATUS_* enum name.
const PRStatusCompleted = "COMPLETED"

// PRReview is one pull-request review from
// GET /api/workspaces/{slug}/pr-reviews (protojson, camelCase). Only the fields
// the CLI needs to list and validate a PR are decoded.
type PRReview struct {
	// ID is the workspace diff-report id. protojson stringifies the int64.
	ID       string `json:"id"`
	PRNumber int    `json:"prNumber"`
	PRTitle  string `json:"prTitle"`
	PRURL    string `json:"prUrl"`
	PRAuthor string `json:"prAuthor"`
	// Status is the review's job status (see PRStatusCompleted).
	Status string `json:"status"`
	// Verdict is PASS / PARTIAL / FAIL once the review has one; empty otherwise.
	Verdict string `json:"verdict"`
}

// Ready reports whether the review has finished and can be scoped against.
func (r PRReview) Ready() bool { return r.Status == PRStatusCompleted }

type listPRReviewsResponse struct {
	Reviews []PRReview `json:"reviews"`
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
