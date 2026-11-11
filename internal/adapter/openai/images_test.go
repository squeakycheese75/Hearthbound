package openai_test

import (
	"context"
	"dnd/internal/adapter/openai"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		_, _ = io.WriteString(w, `{"created":1,"data":[{"b64_json":"`+base64.StdEncoding.EncodeToString([]byte("WEBP"))+`"}]}`)
	}))
	defer srv.Close()
	img, err := openai.New(openai.Config{APIKey: "k", BaseURL: srv.URL}).Generate(context.Background(), "a goblin", "1024x1024")
	if err != nil || string(img) != "WEBP" {
		t.Fatalf("img=%q err=%v", img, err)
	}
	if auth != "Bearer k" || got["model"] != "gpt-image-2" || got["quality"] != "medium" || got["output_format"] != "webp" || got["size"] != "1024x1024" {
		t.Fatalf("request %v auth %q", got, auth)
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		status int
		body   string
		want   string
	}{
		"api error":  {400, `{"error":{"message":"Your request was rejected","code":"moderation_blocked"}}`, "moderation_blocked"},
		"empty data": {200, `{"data":[]}`, "no image"},
		"bad json":   {502, `<html>`, "undecodable"},
		"bad base64": {200, `{"data":[{"b64_json":"!!"}]}`, "decode image"},
	}
	for name, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, tc.body)
		}))
		_, err := openai.New(openai.Config{APIKey: "k", BaseURL: srv.URL, Model: "m", Quality: "low"}).Generate(context.Background(), "p", "1024x1024")
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", name, err, tc.want)
		}
	}
	if _, err := openai.New(openai.Config{BaseURL: "http://127.0.0.1:1"}).Generate(context.Background(), "p", "1"); err == nil {
		t.Error("unreachable server should fail")
	}
}
