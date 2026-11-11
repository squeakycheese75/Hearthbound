package sqlite_test

import (
	"context"
	"dnd/internal/adapter/sqlite"
	"dnd/internal/domain"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestAccountsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	now := time.UnixMilli(1000).UTC()
	admin := domain.Account{ID: "a1", Username: "James", PasswordHash: "h", Admin: true, DailyTokenCap: 10, CreatedAt: now}
	if err := s.CreateAccount(ctx, admin); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateAccount(ctx, domain.Account{ID: "a2", Username: "james"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("duplicate username (case-insensitive): %v", err)
	}
	if n, _ := s.CountAccounts(ctx); n != 1 {
		t.Fatalf("count %d", n)
	}
	got, err := s.AccountByUsername(ctx, "JAMES")
	if err != nil || got.ID != "a1" || !got.Admin || !got.CreatedAt.Equal(now) {
		t.Fatalf("by username %+v %v", got, err)
	}
	if _, err := s.Account(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing account: %v", err)
	}
	if err := s.SetDailyTokenCap(ctx, "a1", 99); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.Account(ctx, "a1"); a.DailyTokenCap != 99 {
		t.Fatalf("cap %d", a.DailyTokenCap)
	}
	if err := s.SetAgentModel(ctx, "a1", "claude-sonnet-5-5"); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.Account(ctx, "a1"); a.AgentModel != "claude-sonnet-5-5" {
		t.Fatalf("agent model %q", a.AgentModel)
	}

	if err := s.CreateInvite(ctx, domain.Invite{Code: "inv", CreatedBy: "a1", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterWithInvite(ctx, "inv", domain.Account{ID: "a3", Username: "pip", PasswordHash: "h", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterWithInvite(ctx, "inv", domain.Account{ID: "a4", Username: "other", CreatedAt: now}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("reused invite: %v", err)
	}
	_ = s.CreateInvite(ctx, domain.Invite{Code: "inv2", CreatedBy: "a1", CreatedAt: now})
	if err := s.RegisterWithInvite(ctx, "inv2", domain.Account{ID: "a5", Username: "PIP", CreatedAt: now}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("taken username: %v", err)
	}
	invs, _ := s.Invites(ctx, "a1")
	used := map[string]string{}
	for _, i := range invs {
		used[i.Code] = i.UsedBy
	}
	if len(invs) != 2 || used["inv"] != "a3" || used["inv2"] != "" {
		t.Fatalf("invites %+v (taken-username registration must not consume inv2)", invs)
	}

	sess := domain.Session{Token: "t", AccountID: "a3", ExpiresAt: now}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Session(ctx, "t"); err != nil || got.AccountID != "a3" {
		t.Fatalf("session %+v %v", got, err)
	}
	_ = s.DeleteSession(ctx, "t")
	if _, err := s.Session(ctx, "t"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted session: %v", err)
	}

	if _, _, err := s.Key(ctx, "a3", domain.KeyAnthropic); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing key: %v", err)
	}
	_ = s.PutKey(ctx, "a3", domain.KeyAnthropic, []byte("sealed1"), "1111")
	_ = s.PutKey(ctx, "a3", domain.KeyAnthropic, []byte("sealed2"), "2222")
	sealed, last4, err := s.Key(ctx, "a3", domain.KeyAnthropic)
	if err != nil || string(sealed) != "sealed2" || last4 != "2222" {
		t.Fatalf("key %q %q %v", sealed, last4, err)
	}
	_ = s.DeleteKey(ctx, "a3", domain.KeyAnthropic)
	if _, _, err := s.Key(ctx, "a3", domain.KeyAnthropic); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted key: %v", err)
	}

	if n, _ := s.Usage(ctx, "a3", "2026-10-31"); n != 0 {
		t.Fatalf("no usage yet: %d", n)
	}
	_ = s.AddUsage(ctx, "a3", "2026-10-31", 100)
	_ = s.AddUsage(ctx, "a3", "2026-10-31", 50)
	if n, _ := s.Usage(ctx, "a3", "2026-10-31"); n != 150 {
		t.Fatalf("usage %d", n)
	}
}

func TestSponsorsAndSponsoredInvites(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	now := time.UnixMilli(1000).UTC()
	_ = s.CreateAccount(ctx, domain.Account{ID: "a1", Username: "james", Admin: true, CreatedAt: now})
	if err := s.CreateInvite(ctx, domain.Invite{Code: "inv", CreatedBy: "a1", Sponsored: true, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	inv, err := s.Invite(ctx, "inv")
	if err != nil || !inv.Sponsored || inv.CreatedBy != "a1" || !inv.CreatedAt.Equal(now) {
		t.Fatalf("invite %+v %v", inv, err)
	}
	if _, err := s.Invite(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing invite: %v", err)
	}
	if invs, _ := s.Invites(ctx, "a1"); len(invs) != 1 || !invs[0].Sponsored {
		t.Fatalf("invites %+v", invs)
	}
	pip := domain.Account{ID: "a2", Username: "pip", SponsoredBy: "a1", SponsorCap: 500, CreatedAt: now}
	if err := s.RegisterWithInvite(ctx, "inv", pip); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.Account(ctx, "a2"); !a.Sponsored() || a.SponsorCap != 500 {
		t.Fatalf("sponsored on registration: %+v", a)
	}
	if err := s.SetSponsor(ctx, "a2", "", 0); err != nil {
		t.Fatal(err)
	}
	all, err := s.Accounts(ctx)
	if err != nil || len(all) != 2 || all[0].Username != "james" || all[1].Sponsored() {
		t.Fatalf("accounts %+v %v", all, err)
	}
}
