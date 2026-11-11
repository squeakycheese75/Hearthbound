package claude_test

import (
	"context"
	"dnd/internal/adapter/claude"
	"dnd/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

func sse(events ...string) string {
	var b strings.Builder
	for _, e := range events {
		var probe struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal([]byte(e), &probe)
		fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", probe.Type, e)
	}
	return b.String()
}

const toolTurn = `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],"stop_reason":null,"usage":{"input_tokens":100,"output_tokens":1,"cache_read_input_tokens":80,"cache_creation_input_tokens":20}}}`

func server(t *testing.T, body string, seen *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if seen != nil {
			if err := json.Unmarshal(raw, seen); err != nil {
				t.Errorf("request body: %v", err)
			}
			(*seen)["_beta"] = r.Header.Get("anthropic-beta")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, body)
	}))
}

func client(url string) *claude.Client {
	return claude.New(claude.Config{}, slog.New(slog.DiscardHandler),
		option.WithBaseURL(url), option.WithAPIKey("test"), option.WithMaxRetries(0))
}

func TestCompleteStreamsTextAndToolCalls(t *testing.T) {
	t.Parallel()
	body := sse(
		toolTurn,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"The door "}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"creaks."}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"ability_check","input":{}}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"character_id\":\"pc1\",\"dc\":12}"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":42}}`,
		`{"type":"message_stop"}`,
	)
	seen := map[string]any{}
	srv := server(t, body, &seen)
	defer srv.Close()

	var streamed strings.Builder
	prior := domain.AgentMessage{Role: domain.RoleAssistant, Raw: json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"Earlier."}]}`)}
	res, err := client(srv.URL).Complete(context.Background(), claude.Request{
		System: "You are the DM.",
		Tools: []domain.ToolSpec{{Name: "ability_check", Description: "Roll", Schema: json.RawMessage(
			`{"type":"object","properties":{"dc":{"type":"integer"}},"required":["dc"],"additionalProperties":false}`)}},
		History: []domain.AgentMessage{
			{Role: domain.RoleUser, Text: "Begin."},
			prior,
			{Role: domain.RoleUser, ToolResults: []domain.ToolResult{{CallID: "toolu_0", Content: "ok"}}},
			{Role: domain.RoleUser, Text: "The party acts."},
		},
		Effort: "low",
		OnText: func(s string) { streamed.WriteString(s) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if streamed.String() != "The door creaks." || res.Message.Text != "The door creaks." {
		t.Fatalf("text %q / %q", streamed.String(), res.Message.Text)
	}
	tc := res.Message.ToolCalls
	if len(tc) != 1 || tc[0].ID != "toolu_1" || !strings.Contains(string(tc[0].Input), `"dc":12`) {
		t.Fatalf("tool calls %+v", tc)
	}
	if res.Model != "claude-opus-5-5" {
		t.Fatalf("answered by %q", res.Model)
	}
	if res.Usage.CacheRead != 80 || res.Usage.Output != 42 || len(res.Message.Raw) == 0 {
		t.Fatalf("usage %+v raw %d", res.Usage, len(res.Message.Raw))
	}
	assertRequest(t, seen)
}

func assertRequest(t *testing.T, seen map[string]any) {
	t.Helper()
	if seen["model"] != "claude-opus-5-5" || seen["fallbacks"] != "default" || seen["stream"] != true {
		t.Errorf("model/fallbacks/stream: %v %v %v", seen["model"], seen["fallbacks"], seen["stream"])
	}
	if !strings.Contains(fmt.Sprint(seen["_beta"]), "server-side-fallback-2026-07-01") {
		t.Errorf("beta header %v", seen["_beta"])
	}
	if eff := seen["output_config"].(map[string]any)["effort"]; eff != "low" {
		t.Errorf("effort %v", eff)
	}
	sys := seen["system"].([]any)[0].(map[string]any)
	if sys["cache_control"] == nil || seen["cache_control"] == nil {
		t.Error("system prompt and history should carry cache_control")
	}
	msgs := seen["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("consecutive user turns should merge: %d messages", len(msgs))
	}
	last := msgs[2].(map[string]any)["content"].([]any)
	if len(last) != 2 || last[0].(map[string]any)["type"] != "tool_result" || last[1].(map[string]any)["type"] != "text" {
		t.Errorf("merged user turn %v", last)
	}
	replayed := msgs[1].(map[string]any)["content"].([]any)[0].(map[string]any)
	if replayed["text"] != "Earlier." {
		t.Errorf("assistant turn should replay from raw: %v", replayed)
	}
	tool := seen["tools"].([]any)[0].(map[string]any)
	if tool["input_schema"].(map[string]any)["additionalProperties"] != false {
		t.Errorf("tool schema %v", tool)
	}
}

func TestPerRequestModelAndHaikuHasNoFallbacks(t *testing.T) {
	t.Parallel()
	body := sse(strings.Replace(toolTurn, "claude-opus-5-5", "claude-haiku-5-5", 1),
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`, `{"type":"message_stop"}`)
	seen := map[string]any{}
	srv := server(t, body, &seen)
	defer srv.Close()
	res, err := client(srv.URL).Complete(context.Background(), claude.Request{
		Model: "claude-haiku-5-5", System: "x", History: []domain.AgentMessage{{Role: domain.RoleUser, Text: "Hi."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen["model"] != "claude-haiku-5-5" || seen["fallbacks"] != nil || strings.Contains(fmt.Sprint(seen["_beta"]), "fallback") {
		t.Fatalf("haiku request: model %v fallbacks %v beta %v", seen["model"], seen["fallbacks"], seen["_beta"])
	}
	if res.Model != "claude-haiku-5-5" {
		t.Fatalf("answered by %q", res.Model)
	}
}

func TestRefusalIsAnError(t *testing.T) {
	t.Parallel()
	body := sse(toolTurn,
		`{"type":"message_delta","delta":{"stop_reason":"refusal","stop_details":{"type":"refusal","category":"cyber"}},"usage":{"output_tokens":1}}`,
		`{"type":"message_stop"}`)
	srv := server(t, body, nil)
	defer srv.Close()
	_, err := client(srv.URL).Complete(context.Background(), claude.Request{System: "x", History: []domain.AgentMessage{{Role: domain.RoleUser, Text: "hi"}}})
	if !errors.Is(err, claude.ErrRefused) {
		t.Fatalf("got %v", err)
	}
}

func TestBadInputs(t *testing.T) {
	t.Parallel()
	c := client("http://127.0.0.1:1")
	_, err := c.Complete(context.Background(), claude.Request{Tools: []domain.ToolSpec{{Name: "x", Schema: json.RawMessage(`[`)}}})
	if err == nil {
		t.Fatal("bad schema should fail")
	}
	_, err = c.Complete(context.Background(), claude.Request{History: []domain.AgentMessage{{Role: domain.RoleAssistant}}})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("assistant turn without raw: %v", err)
	}
	_, err = c.Complete(context.Background(), claude.Request{History: []domain.AgentMessage{{Role: domain.RoleUser, Text: "hi"}}})
	if err == nil {
		t.Fatal("unreachable server should fail")
	}
}
