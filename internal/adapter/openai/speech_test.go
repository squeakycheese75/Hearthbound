package openai_test

import (
	"context"
	"dnd/internal/adapter/openai"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSpeech(t *testing.T) {
	t.Parallel()
	var got map[string]string
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		switch got["input"] {
		case "fail":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"bad key","code":"invalid_api_key"}}`))
		case "garbled":
			w.WriteHeader(http.StatusBadGateway)
		default:
			_, _ = w.Write([]byte("ID3audio"))
		}
	}))
	defer srv.Close()
	sp := openai.NewSpeech("", srv.URL)

	audio, err := sp.Speak(context.Background(), "sk-me", "Hello there.", "fable", "warm")
	if err != nil || string(audio) != "ID3audio" {
		t.Fatalf("speak: %q %v", audio, err)
	}
	if auth != "Bearer sk-me" || got["model"] != "gpt-4o-mini-tts" || got["voice"] != "fable" || got["instructions"] != "warm" || got["response_format"] != "mp3" {
		t.Fatalf("request: %s %v", auth, got)
	}
	if _, err := sp.Speak(context.Background(), "k", "fail", "fable", ""); err == nil || !strings.Contains(err.Error(), "bad key") {
		t.Fatalf("API error: %v", err)
	}
	if _, err := sp.Speak(context.Background(), "k", "garbled", "fable", ""); err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("bare HTTP error: %v", err)
	}
	if _, err := openai.NewSpeech("m", "http://127.0.0.1:1").Speak(context.Background(), "k", "x", "fable", ""); err == nil {
		t.Fatal("unreachable server")
	}
}
