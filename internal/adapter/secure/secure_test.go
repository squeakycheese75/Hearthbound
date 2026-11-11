package secure_test

import (
	"bytes"
	"crypto/rand"
	"dnd/internal/adapter/secure"
	"encoding/base64"
	"errors"
	"testing"
)

func key() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func TestVault(t *testing.T) {
	t.Parallel()
	v, err := secure.NewVault(key())
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := v.Seal([]byte("sk-ant-secret"))
	if err != nil || bytes.Contains(sealed, []byte("sk-ant")) {
		t.Fatalf("seal: %v", err)
	}
	plain, err := v.Open(sealed)
	if err != nil || string(plain) != "sk-ant-secret" {
		t.Fatalf("open %q %v", plain, err)
	}
	other, _ := secure.NewVault(key())
	if _, err := other.Open(sealed); !errors.Is(err, secure.ErrTampered) {
		t.Fatalf("wrong key: %v", err)
	}
	if _, err := v.Open([]byte("x")); !errors.Is(err, secure.ErrTampered) {
		t.Fatalf("short: %v", err)
	}
	for _, bad := range []string{"", "not base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := secure.NewVault(bad); !errors.Is(err, secure.ErrBadMasterKey) {
			t.Errorf("NewVault(%q) = %v", bad, err)
		}
	}
}

func TestPasswordsAndTokens(t *testing.T) {
	t.Parallel()
	h, err := secure.Passwords{}.Hash("correct horse battery")
	if err != nil || !(secure.Passwords{}).Matches(h, "correct horse battery") || (secure.Passwords{}).Matches(h, "wrong") {
		t.Fatalf("bcrypt: %v", err)
	}
	a, b := secure.Tokens{}.New(), secure.Tokens{}.New()
	if a == b || len(a) < 40 {
		t.Fatalf("tokens %q %q", a, b)
	}
}
