package domain

import (
	"encoding/json"
	"fmt"
)

// Role is who wrote an agent message.
type Role string

// Roles.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ToolCall is a request from an agent to run a game tool.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult answers a ToolCall.
type ToolResult struct {
	CallID  string `json:"callId"`
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

// AgentMessage is one turn in an agent's transcript. Raw holds the provider's own
// encoding of an assistant turn so it can be replayed byte-for-byte (needed for
// prompt caching and preserved thinking).
type AgentMessage struct {
	Role        Role            `json:"role"`
	Text        string          `json:"text,omitempty"`
	ToolCalls   []ToolCall      `json:"toolCalls,omitempty"`
	ToolResults []ToolResult    `json:"toolResults,omitempty"`
	Raw         json.RawMessage `json:"raw,omitempty"`
}

// ToolSpec describes a tool an agent may call. Schema is a JSON Schema object.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

// Agent roles. Each role is defined by one definition file: its model,
// effort, tools and prompt.
const (
	RoleDM        = "dm"        // narrates and runs the rules
	RoleCharacter = "character" // plays one party member
	RoleChoices   = "choices"   // writes quick choices when the DM forgets
	RoleCast      = "cast"      // makes sure everyone the DM narrated is shown on the table
	RoleRecap     = "recap"     // the DM's private story-so-far at a chapter break
)

// AgentRole is how one kind of agent is set up. Prompt is a text/template
// rendered by the play use case; it must render identically for the same game
// so the system prompt stays cached.
type AgentRole struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Model       string   `json:"model"`
	Effort      string   `json:"effort"`
	Tools       []string `json:"tools,omitempty"`
	Prompt      string   `json:"-"`
}

// AgentModel is a model a player may run their characters' agents on.
type AgentModel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Note     string `json:"note"`
}

// AgentModels are the models players can choose for their characters. Only
// the Anthropic provider is wired up so far.
var AgentModels = []AgentModel{
	{ID: "claude-opus-5-5", Name: "Claude Opus 5.5", Provider: "anthropic", Note: "richest play, highest cost"},
	{ID: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5", Provider: "anthropic", Note: "about half the cost of Opus"},
	{ID: "claude-haiku-5-5", Name: "Claude Haiku 5.5", Provider: "anthropic", Note: "fast and very cheap"},
}

// ParseAgentModel validates a model choice. Empty means "the table's default".
func ParseAgentModel(id string) (string, error) {
	if id == "" {
		return "", nil
	}
	for _, m := range AgentModels {
		if m.ID == id {
			return id, nil
		}
	}
	return "", fmt.Errorf("%w: model %q", ErrUnknown, id)
}
