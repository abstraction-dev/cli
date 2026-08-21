package transport

import (
	"encoding/json"
	"strings"
)

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
	// SessionID names the conversation this ask belongs to — always a chat slug the
	// backend handed out, either when the conversation started or from the chat list.
	// Empty starts a conversation, and the reply carries the slug it was given: see
	// AskResult.ChatSlug and StreamHandlers.OnConversation. An id the backend doesn't
	// recognise is refused rather than answered in some other conversation.
	SessionID string `json:"session_id,omitempty"`
}

// askResponse is the buffered (non-stream) reply.
type askResponse struct {
	Answer   string `json:"answer"`
	ChatSlug string `json:"chat_slug"`
}

// AskResult is one buffered ask's reply.
type AskResult struct {
	Answer string
	// ChatSlug names the conversation the ask ran in. Send it as the next ask's
	// SessionID to continue that conversation.
	ChatSlug string
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

// ChatTypePR is the Chat.Type of a conversation scoped to a pull request's diff
// report; a workspace conversation is "DEFAULT".
const ChatTypePR = "PR"

// Chat is one stored conversation from GET /api/workspaces/{slug}/chats
// (protojson, camelCase). Only the fields the picker lists and resumes are
// decoded.
type Chat struct {
	// Slug identifies the conversation. Sent as an ask's SessionID it continues
	// this conversation, wherever it was started — the app or an earlier CLI run.
	Slug string `json:"slug"`
	// Title is the conversation's name, which the backend replaces with what the
	// conversation turned out to be about once it has been summarized.
	Title   string `json:"title"`
	Summary string `json:"summary"`
	// Type is DEFAULT or PR — see ChatTypePR.
	Type string `json:"type"`
	// DiffReportID is the diff report a PR conversation reads, and doubles as an
	// AskRequest.PR value. Empty for a workspace conversation. protojson
	// stringifies the int64.
	DiffReportID string `json:"diffReportId"`
	// PRNumber labels which pull request a PR conversation belongs to. protojson
	// stringifies the int64.
	PRNumber string `json:"prNumber"`
	// CreatedAt is an RFC 3339 timestamp (protojson).
	CreatedAt string `json:"createdAt"`
}

// IsPR reports whether the conversation is scoped to a pull request.
func (c Chat) IsPR() bool { return c.Type == ChatTypePR }

// ChatMessage is one stored exchange of a conversation.
type ChatMessage struct {
	// Messages is the exchange as the model saw it: the user's question followed by
	// the assistant and tool messages that answered it. protojson base64-encodes a
	// bytes field and encoding/json decodes it back, leaving the JSON the backend
	// stored — see Turns.
	Messages []byte `json:"messages"`
	// CreatedAt is an RFC 3339 timestamp (protojson).
	CreatedAt string `json:"createdAt"`
}

// ChatWithMessages is a conversation and its full history, oldest exchange first,
// from GET /api/chats/{chatSlug}.
type ChatWithMessages struct {
	Chat     Chat          `json:"chat"`
	Messages []ChatMessage `json:"messages"`
}

// Roles a replayed Turn can carry.
const (
	TurnUser      = "user"
	TurnAssistant = "assistant"
)

// Turn is one side of a stored exchange, flattened to the text a transcript shows.
type Turn struct {
	Role string
	Text string
}

// Turns flattens an exchange into the question and answer a transcript replays.
//
// Everything that is how the answer was reached rather than part of the
// conversation is dropped: tool calls and their results, system messages, and the
// model's thinking. What's left is what the terminal printed the first time round.
func (m ChatMessage) Turns() ([]Turn, error) {
	var stored []storedMessage
	if err := json.Unmarshal(m.Messages, &stored); err != nil {
		return nil, err
	}

	var turns []Turn
	for _, msg := range stored {
		if msg.Role != TurnUser && msg.Role != TurnAssistant {
			continue
		}

		text := msg.text()
		if text == "" {
			continue
		}
		turns = append(turns, Turn{Role: msg.Role, Text: text})
	}
	return turns, nil
}

// storedMessage is one llm message as the backend stores it. Content parts are
// flat, tagged objects; only "text" parts carry conversation.
type storedMessage struct {
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (m storedMessage) text() string {
	var parts []string
	for _, part := range m.Content {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

type listChatsResponse struct {
	Chats []Chat `json:"chats"`
}

type getChatResponse struct {
	Chat ChatWithMessages `json:"chat"`
}

// StreamHandlers receive decoded frames from a streaming ask.
type StreamHandlers struct {
	// OnOutput receives a chunk of formatted answer text.
	OnOutput func(text string)
	// OnStatus receives a transient progress label (safe to ignore).
	OnStatus func(message string)
	// OnConversation receives the slug of the conversation this ask runs in, before
	// any answer text. Hold onto it and send it as the next ask's SessionID — for a
	// conversation that just started, this is the only place its id appears.
	OnConversation func(chatSlug string)
}

// SSE frame payloads emitted by the terminal sink.
type outputFrame struct {
	Text string `json:"text"`
}

type messageFrame struct {
	Message string `json:"message"`
}

type conversationFrame struct {
	ChatSlug string `json:"chat_slug"`
}
