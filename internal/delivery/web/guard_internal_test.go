package web

import (
	"fmt"
	"testing"
	"time"
)

func TestAttemptsExpireAndStayBounded(t *testing.T) {
	t.Parallel()
	a := newAttempts()
	t0 := time.Unix(0, 0)
	for range failsPerUser {
		a.fail(t0, "user:x")
	}
	if !a.blocked(t0, "user:x") || a.blocked(t0.Add(attemptWindow), "user:x") {
		t.Fatal("failures count within the window only")
	}
	if len(a.fails) != 0 {
		t.Fatal("expired keys are dropped")
	}
	for i := range attemptKeysCap + 2 {
		a.fail(t0, fmt.Sprint("addr:", i))
	}
	a.fail(t0.Add(attemptWindow), "addr:new")
	if len(a.fails) != 1 {
		t.Fatalf("a full table is swept of expired keys: %d left", len(a.fails))
	}
}
