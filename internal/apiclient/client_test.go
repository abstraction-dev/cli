package apiclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient serves one handler and points a Client at it.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(server.URL, "abs_test_key")
}

func TestListChats(t *testing.T) {
	var gotPath, gotAuth string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Write([]byte(`{"chats":[
			{"slug":"c1","title":"Where is auth handled","type":"DEFAULT","createdAt":"2026-07-20T10:11:12Z"},
			{"slug":"c2","title":"CLI ask · ws","type":"PR","diffReportId":"42","prNumber":"123","createdAt":"2026-07-21T09:00:00Z"}
		]}`))
	})

	chats, err := client.ListChats(context.Background(), "ws-slug")
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/api/workspaces/ws-slug/chats" {
		t.Fatalf("path: got %q", gotPath)
	}
	if gotAuth != "Bearer abs_test_key" {
		t.Fatalf("auth header: got %q", gotAuth)
	}
	if len(chats) != 2 {
		t.Fatalf("expected 2 chats, got %d", len(chats))
	}
	if chats[0].Slug != "c1" || chats[0].Title != "Where is auth handled" || chats[0].IsPR() {
		t.Fatalf("first chat: %+v", chats[0])
	}

	// protojson stringifies int64 fields, so the PR scope arrives as text and is
	// usable as an AskRequest.PR value unchanged.
	if !chats[1].IsPR() || chats[1].DiffReportID != "42" || chats[1].PRNumber != "123" {
		t.Fatalf("pr chat: %+v", chats[1])
	}
}

func TestListChatsError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no access to workspace", http.StatusForbidden)
	})

	_, err := client.ListChats(context.Background(), "ws-slug")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an *APIError, got %v", err)
	}
	if apiErr.Status != http.StatusForbidden || !apiErr.IsAuth() {
		t.Fatalf("expected a 403 auth error, got %+v", apiErr)
	}
	if apiErr.Message != "no access to workspace" {
		t.Fatalf("message: got %q", apiErr.Message)
	}
}

func TestGetChat(t *testing.T) {
	exchange := `[
		{"role":"user","content":[{"type":"text","text":"where is auth handled"}]},
		{"role":"assistant","content":[{"type":"thinking","text":"let me look"}]},
		{"role":"assistant","tool_calls":[{"id":"t1","name":"aql"}]},
		{"role":"tool","tool_call_id":"t1","content":[{"type":"text","text":"rows"}]},
		{"role":"assistant","content":[{"type":"text","text":"In internal/auth."}]}
	]`

	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, err := json.Marshal(map[string]any{
			"chat": map[string]any{
				"chat": map[string]any{"slug": "c1", "title": "Auth", "type": "DEFAULT"},
				// protojson base64-encodes a bytes field.
				"messages": []map[string]any{{
					"messages":  base64.StdEncoding.EncodeToString([]byte(exchange)),
					"createdAt": "2026-07-20T10:11:12Z",
				}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		w.Write(body)
	})

	loaded, err := client.GetChat(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/api/chats/c1" {
		t.Fatalf("path: got %q", gotPath)
	}
	if loaded.Chat.Slug != "c1" || loaded.Chat.Title != "Auth" {
		t.Fatalf("chat: %+v", loaded.Chat)
	}
	if len(loaded.Messages) != 1 {
		t.Fatalf("expected 1 exchange, got %d", len(loaded.Messages))
	}

	turns, err := loaded.Messages[0].Turns()
	if err != nil {
		t.Fatal(err)
	}
	want := []Turn{
		{Role: TurnUser, Text: "where is auth handled"},
		{Role: TurnAssistant, Text: "In internal/auth."},
	}
	if len(turns) != len(want) {
		t.Fatalf("expected %d turns, got %d: %+v", len(want), len(turns), turns)
	}
	for i := range want {
		if turns[i] != want[i] {
			t.Fatalf("turn %d: got %+v, want %+v", i, turns[i], want[i])
		}
	}
}

func TestGetChatNotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetChat(context.Background(), "missing")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an *APIError, got %v", err)
	}
	if apiErr.Status != http.StatusNotFound {
		t.Fatalf("expected a 404, got %+v", apiErr)
	}
}

// The backend names the conversation an ask ran in, and both transports report it —
// on a conversation's first question that reply is the only place its id appears.
func TestAskBufferedReportsConversation(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"answer":"In internal/auth.","chat_slug":"chat-9"}`))
	})

	res, err := client.AskBuffered(context.Background(), AskRequest{Workspace: "ws", Question: "where is auth"})
	if err != nil {
		t.Fatal(err)
	}

	if res.Answer != "In internal/auth." || res.ChatSlug != "chat-9" {
		t.Fatalf("got %+v", res)
	}
}

func TestAskStreamReportsConversation(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("event: conversation\ndata: {\"chat_slug\":\"chat-9\"}\n\n" +
			"event: output\ndata: {\"text\":\"hi\"}\n\n" +
			"event: stream_end\n\n"))
	})

	var slug, text string
	err := client.AskStream(context.Background(), AskRequest{Workspace: "ws", Question: "q"}, StreamHandlers{
		OnOutput:       func(t string) { text += t },
		OnConversation: func(s string) { slug = s },
	})
	if err != nil {
		t.Fatal(err)
	}

	if slug != "chat-9" {
		t.Fatalf("conversation: got %q, want chat-9", slug)
	}
	if text != "hi" {
		t.Fatalf("output: got %q", text)
	}
}

func TestChatMessageTurns(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		want  []Turn
		fails bool
	}{
		{
			name: "multiple text parts join",
			raw:  `[{"role":"assistant","content":[{"type":"text","text":"one"},{"type":"text","text":"two"}]}]`,
			want: []Turn{{Role: TurnAssistant, Text: "one\ntwo"}},
		},
		{
			name: "system messages are not conversation",
			raw:  `[{"role":"system","content":[{"type":"text","text":"you are astrid"}]}]`,
		},
		{
			name: "an answerless assistant turn is dropped",
			raw:  `[{"role":"assistant","content":[{"type":"text","text":"   "}]}]`,
		},
		{
			name:  "malformed payload is reported",
			raw:   `not json`,
			fails: true,
		},
		{
			name: "empty history",
			raw:  `[]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			turns, err := ChatMessage{Messages: []byte(tc.raw)}.Turns()
			if tc.fails {
				if err == nil {
					t.Fatalf("expected an error, got %+v", turns)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(turns) != len(tc.want) {
				t.Fatalf("expected %d turns, got %d: %+v", len(tc.want), len(turns), turns)
			}
			for i := range tc.want {
				if turns[i] != tc.want[i] {
					t.Fatalf("turn %d: got %+v, want %+v", i, turns[i], tc.want[i])
				}
			}
		})
	}
}
