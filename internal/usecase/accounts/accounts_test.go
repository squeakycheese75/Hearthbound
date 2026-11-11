package accounts_test

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/usecase/accounts"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

var ctx = context.Background()

// memStore is an in-memory Store; fail names a method that should error.
type memStore struct {
	accounts map[string]domain.Account
	sessions map[string]domain.Session
	invites  map[string]domain.Invite
	keys     map[string][]byte
	last4    map[string]string
	usage    map[string]int
	fail     string
	failKey  string // an account whose keys can't be read
}

func newStore() *memStore {
	return &memStore{
		accounts: map[string]domain.Account{}, sessions: map[string]domain.Session{}, invites: map[string]domain.Invite{},
		keys: map[string][]byte{}, last4: map[string]string{}, usage: map[string]int{},
	}
}

var errBoom = errors.New("boom")

func (m *memStore) err(op string) error {
	if m.fail == op {
		return errBoom
	}
	return nil
}

func (m *memStore) CountAccounts(context.Context) (int, error) {
	return len(m.accounts), m.err("count")
}

func (m *memStore) CreateAccount(_ context.Context, a domain.Account) error {
	if err := m.err("create"); err != nil {
		return err
	}
	m.accounts[a.ID] = a
	return nil
}

func (m *memStore) Account(_ context.Context, id string) (domain.Account, error) {
	a, ok := m.accounts[id]
	if !ok || m.fail == "account" {
		return domain.Account{}, domain.ErrNotFound
	}
	return a, nil
}

func (m *memStore) AccountByUsername(_ context.Context, name string) (domain.Account, error) {
	if err := m.err("byname"); err != nil {
		return domain.Account{}, err
	}
	for _, a := range m.accounts {
		if a.Username == name {
			return a, nil
		}
	}
	return domain.Account{}, domain.ErrNotFound
}

func (m *memStore) SetDailyTokenCap(_ context.Context, id string, c int) error {
	a := m.accounts[id]
	a.DailyTokenCap = c
	m.accounts[id] = a
	return m.err("cap")
}

func (m *memStore) SetAgentModel(_ context.Context, id, model string) error {
	a := m.accounts[id]
	a.AgentModel = model
	m.accounts[id] = a
	return m.err("model")
}

func (m *memStore) Accounts(context.Context) ([]domain.Account, error) {
	out := make([]domain.Account, 0, len(m.accounts))
	for _, a := range m.accounts {
		out = append(out, a)
	}
	return out, m.err("accounts")
}

func (m *memStore) SetSponsor(_ context.Context, id, sponsor string, c int) error {
	a := m.accounts[id]
	a.SponsoredBy, a.SponsorCap = sponsor, c
	m.accounts[id] = a
	return m.err("sponsor")
}

func (m *memStore) Invite(_ context.Context, code string) (domain.Invite, error) {
	inv, ok := m.invites[code]
	if !ok || m.fail == "loadinvite" {
		return domain.Invite{}, domain.ErrNotFound
	}
	return inv, nil
}

func (m *memStore) CreateSession(_ context.Context, s domain.Session) error {
	m.sessions[s.Token] = s
	return m.err("session")
}

func (m *memStore) Session(_ context.Context, token string) (domain.Session, error) {
	s, ok := m.sessions[token]
	if !ok {
		return domain.Session{}, domain.ErrNotFound
	}
	return s, nil
}

func (m *memStore) DeleteSession(_ context.Context, token string) error {
	delete(m.sessions, token)
	return m.err("logout")
}

func (m *memStore) CreateInvite(_ context.Context, inv domain.Invite) error {
	m.invites[inv.Code] = inv
	return m.err("invite")
}

func (m *memStore) Invites(context.Context, string) ([]domain.Invite, error) {
	var out []domain.Invite
	for _, i := range m.invites {
		out = append(out, i)
	}
	return out, m.err("invites")
}

func (m *memStore) RegisterWithInvite(_ context.Context, code string, a domain.Account) error {
	inv, ok := m.invites[code]
	if !ok || inv.UsedBy != "" {
		return fmt.Errorf("%w: invite", domain.ErrInvalid)
	}
	inv.UsedBy = a.ID
	m.invites[code] = inv
	m.accounts[a.ID] = a
	return nil
}

func (m *memStore) PutKey(_ context.Context, id string, kind domain.KeyKind, sealed []byte, last4 string) error {
	m.keys[id+string(kind)] = sealed
	m.last4[id+string(kind)] = last4
	return m.err("putkey")
}

func (m *memStore) Key(_ context.Context, id string, kind domain.KeyKind) ([]byte, string, error) {
	if err := m.err("key"); err != nil || id == m.failKey {
		return nil, "", errBoom
	}
	k, ok := m.keys[id+string(kind)]
	if !ok {
		return nil, "", domain.ErrNotFound
	}
	return k, m.last4[id+string(kind)], nil
}

func (m *memStore) DeleteKey(_ context.Context, id string, kind domain.KeyKind) error {
	delete(m.keys, id+string(kind))
	return m.err("delkey")
}

func (m *memStore) AddUsage(_ context.Context, id, day string, n int) error {
	m.usage[id+day] += n
	return m.err("addusage")
}

func (m *memStore) Usage(_ context.Context, id, day string) (int, error) {
	return m.usage[id+day], m.err("usage")
}

type hasher struct{ fail bool }

func (h hasher) Hash(pw string) (string, error) {
	if h.fail {
		return "", errBoom
	}
	return "h:" + pw, nil
}
func (hasher) Matches(hash, pw string) bool { return hash == "h:"+pw }

type cipher struct{ failSeal, failOpen bool }

func (c cipher) Seal(p []byte) ([]byte, error) {
	if c.failSeal {
		return nil, errBoom
	}
	return append([]byte("enc:"), p...), nil
}

func (c cipher) Open(s []byte) ([]byte, error) {
	if c.failOpen {
		return nil, errBoom
	}
	return []byte(strings.TrimPrefix(string(s), "enc:")), nil
}

type seqTokens struct{ n int }

func (t *seqTokens) New() string { t.n++; return fmt.Sprintf("tok%d", t.n) }

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

type rig struct {
	uc    *accounts.UseCase
	store *memStore
	clock *clock
}

func setup(t *testing.T) rig {
	t.Helper()
	r := rig{store: newStore(), clock: &clock{now: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)}}
	r.uc = accounts.New(r.store, hasher{}, cipher{}, &seqTokens{}, r.clock)
	return r
}

const pw = "correct horse battery"

func TestBootstrapInviteRegisterLogin(t *testing.T) {
	t.Parallel()
	r := setup(t)
	created, err := r.uc.Bootstrap(ctx, "james", pw)
	if err != nil || !created {
		t.Fatalf("bootstrap: %v %v", created, err)
	}
	if again, _ := r.uc.Bootstrap(ctx, "other", pw); again {
		t.Fatal("bootstrap must only run on an empty server")
	}
	admin, _ := r.store.AccountByUsername(ctx, "james")
	inv, err := r.uc.CreateInvite(ctx, admin.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if list, err := r.uc.Invites(ctx, admin.ID); err != nil || len(list) != 1 {
		t.Fatalf("invites %v %v", list, err)
	}
	sess, err := r.uc.Register(ctx, " "+inv.Code+" ", "pip", pw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.uc.Register(ctx, inv.Code, "pip2", pw); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("invite reuse: %v", err)
	}
	who, err := r.uc.Authenticate(ctx, sess.Token)
	if err != nil || who.Username != "pip" || who.Admin {
		t.Fatalf("authenticate %+v %v", who, err)
	}
	if _, err := r.uc.CreateInvite(ctx, who.ID, false); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("non-admin invite: %v", err)
	}
	if _, err := r.uc.Invites(ctx, who.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("non-admin list: %v", err)
	}
	login, err := r.uc.Login(ctx, "pip", pw)
	if err != nil || login.Token == sess.Token {
		t.Fatalf("login %v", err)
	}
	if err := r.uc.Logout(ctx, login.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := r.uc.Authenticate(ctx, login.Token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("after logout: %v", err)
	}
	r.clock.now = r.clock.now.Add(accounts.SessionTTL + time.Second)
	if _, err := r.uc.Authenticate(ctx, sess.Token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expired session: %v", err)
	}
}

func TestLoginFailures(t *testing.T) {
	t.Parallel()
	r := setup(t)
	_, _ = r.uc.Bootstrap(ctx, "james", pw)
	for _, c := range [][2]string{{"james", "wrong password!!"}, {"nobody", pw}} {
		if _, err := r.uc.Login(ctx, c[0], c[1]); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("login %v: %v", c, err)
		}
	}
	r.store.fail = "session"
	if _, err := r.uc.Login(ctx, "james", pw); !errors.Is(err, errBoom) {
		t.Fatalf("session store failure: %v", err)
	}
	r.store.fail = "logout"
	if err := r.uc.Logout(ctx, "x"); !errors.Is(err, errBoom) {
		t.Fatalf("logout failure: %v", err)
	}
	r.store.fail = "account"
	r.store.sessions["s"] = domain.Session{Token: "s", AccountID: "gone", ExpiresAt: r.clock.now.Add(time.Hour)}
	if _, err := r.uc.Authenticate(ctx, "s"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("missing account: %v", err)
	}
}

func TestValidationAndBootstrapErrors(t *testing.T) {
	t.Parallel()
	r := setup(t)
	if _, err := r.uc.Bootstrap(ctx, "x", pw); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("short username: %v", err)
	}
	if _, err := r.uc.Bootstrap(ctx, "james", "short"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("short password: %v", err)
	}
	if _, err := r.uc.Register(ctx, "code", "bad name!", pw); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("register validation: %v", err)
	}
	for _, op := range []string{"count", "create"} {
		r := setup(t)
		r.store.fail = op
		if _, err := r.uc.Bootstrap(ctx, "james", pw); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", op, err)
		}
	}
	bad := accounts.New(newStore(), hasher{fail: true}, cipher{}, &seqTokens{}, &clock{})
	if _, err := bad.Bootstrap(ctx, "james", pw); !errors.Is(err, errBoom) {
		t.Fatalf("hash failure: %v", err)
	}
	r = setup(t)
	_, _ = r.uc.Bootstrap(ctx, "james", pw)
	admin, _ := r.store.AccountByUsername(ctx, "james")
	for _, op := range []string{"invite", "invites"} {
		r.store.fail = op
		var err error
		if op == "invite" {
			_, err = r.uc.CreateInvite(ctx, admin.ID, false)
		}
		if op == "invites" {
			_, err = r.uc.Invites(ctx, admin.ID)
		}
		if !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", op, err)
		}
	}
	if _, err := r.uc.CreateInvite(ctx, "ghost", false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown admin: %v", err)
	}
}

func TestLocalAccount(t *testing.T) {
	t.Parallel()
	r := setup(t)
	a, err := r.uc.Local(ctx)
	if err != nil || a.Username != "local" || !a.Admin {
		t.Fatalf("local %+v %v", a, err)
	}
	again, _ := r.uc.Local(ctx)
	if again.ID != a.ID {
		t.Fatal("local account should be reused")
	}
	for _, op := range []string{"byname", "create"} {
		r := setup(t)
		r.store.fail = op
		if _, err := r.uc.Local(ctx); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", op, err)
		}
	}
}

func TestKeysAndSpend(t *testing.T) {
	t.Parallel()
	r := setup(t)
	_, _ = r.uc.Bootstrap(ctx, "james", pw)
	id := func() string { a, _ := r.store.AccountByUsername(ctx, "james"); return a.ID }()

	if _, err := r.uc.Credential(ctx, id, domain.KeyAnthropic); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("no key: %v", err)
	}
	if err := r.uc.SetKey(ctx, id, domain.KeyAnthropic, "short"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad key: %v", err)
	}
	if err := r.uc.SetKey(ctx, id, domain.KeyAnthropic, " sk-ant-0123456789abcdefWXYZ "); err != nil {
		t.Fatal(err)
	}
	keys, err := r.uc.Keys(ctx, id)
	if err != nil || !keys[0].Set || keys[0].Last4 != "WXYZ" || keys[1].Set {
		t.Fatalf("keys %+v %v", keys, err)
	}
	if strings.Contains(fmt.Sprint(keys), "sk-ant") {
		t.Fatal("key info must never contain the key")
	}
	key, err := r.uc.Credential(ctx, id, domain.KeyAnthropic)
	if err != nil || key != "sk-ant-0123456789abcdefWXYZ" {
		t.Fatalf("credential %q %v", key, err)
	}
	if err := r.uc.SetDailyTokenCap(ctx, id, 100); err != nil {
		t.Fatal(err)
	}
	if err := r.uc.RecordUsage(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	if err := r.uc.RecordUsage(ctx, id, 150); err != nil {
		t.Fatal(err)
	}
	if spend, _ := r.uc.Today(ctx, id); spend.Used != 150 || spend.Cap != 100 {
		t.Fatalf("spend %+v", spend)
	}
	if _, err := r.uc.Credential(ctx, id, domain.KeyAnthropic); !errors.Is(err, domain.ErrOverBudget) {
		t.Fatalf("over budget: %v", err)
	}
	r.clock.now = r.clock.now.Add(24 * time.Hour) // a new day resets the budget
	if _, err := r.uc.Credential(ctx, id, domain.KeyAnthropic); err != nil {
		t.Fatalf("new day: %v", err)
	}
	if err := r.uc.SetKey(ctx, id, domain.KeyAnthropic, ""); err != nil {
		t.Fatal(err)
	}
	if keys, _ := r.uc.Keys(ctx, id); keys[0].Set {
		t.Fatal("key should be removed")
	}
	if err := r.uc.SetDailyTokenCap(ctx, id, -1); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("negative cap: %v", err)
	}
	if err := r.uc.SetAgentModel(ctx, id, "claude-haiku-5-5"); err != nil {
		t.Fatal(err)
	}
	if m, err := r.uc.AgentModel(ctx, id); err != nil || m != "claude-haiku-5-5" {
		t.Fatalf("agent model %q %v", m, err)
	}
	if err := r.uc.SetAgentModel(ctx, id, "gpt-9"); !errors.Is(err, domain.ErrUnknown) {
		t.Fatalf("unknown model: %v", err)
	}
	if err := r.uc.SetAgentModel(ctx, id, ""); err != nil {
		t.Fatal(err)
	}
	if m, _ := r.uc.AgentModel(ctx, id); m != "" {
		t.Fatal("empty resets to the table default")
	}
}

func TestKeyAndSpendFailures(t *testing.T) {
	t.Parallel()
	r := setup(t)
	_, _ = r.uc.Bootstrap(ctx, "james", pw)
	a, _ := r.store.AccountByUsername(ctx, "james")
	good := "sk-ant-0123456789abcdefWXYZ"
	_ = r.uc.SetKey(ctx, a.ID, domain.KeyOpenAI, good)
	_ = r.uc.SetKey(ctx, a.ID, domain.KeyAnthropic, good)

	cases := []struct {
		op string
		fn func() error
	}{
		{"delkey", func() error { return r.uc.SetKey(ctx, a.ID, domain.KeyAnthropic, "") }},
		{"putkey", func() error { return r.uc.SetKey(ctx, a.ID, domain.KeyAnthropic, good) }},
		{"key", func() error { _, err := r.uc.Keys(ctx, a.ID); return err }},
		{"key", func() error { _, err := r.uc.Credential(ctx, a.ID, domain.KeyAnthropic); return err }},
		{"usage", func() error { _, err := r.uc.Credential(ctx, a.ID, domain.KeyAnthropic); return err }},
		{"usage", func() error { _, err := r.uc.Today(ctx, a.ID); return err }},
		{"cap", func() error { return r.uc.SetDailyTokenCap(ctx, a.ID, 5) }},
		{"model", func() error { return r.uc.SetAgentModel(ctx, a.ID, "claude-haiku-5-5") }},
		{"addusage", func() error { return r.uc.RecordUsage(ctx, a.ID, 5) }},
	}
	for _, c := range cases {
		r.store.fail = c.op
		if err := c.fn(); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", c.op, err)
		}
	}
	r.store.fail = "account"
	if _, err := r.uc.Today(ctx, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("today without account: %v", err)
	}
	if _, err := r.uc.AgentModel(ctx, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("agent model without account: %v", err)
	}
	if _, err := r.uc.Username(ctx, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("username without account: %v", err)
	}
	r.store.fail = ""
	if name, err := r.uc.Username(ctx, a.ID); err != nil || name != "james" {
		t.Fatalf("username %q %v", name, err)
	}
	if admin, err := r.uc.IsAdmin(ctx, a.ID); err != nil || !admin {
		t.Fatalf("james is the admin: %v %v", admin, err)
	}
	r.store.fail = "account"
	if _, err := r.uc.IsAdmin(ctx, a.ID); err == nil {
		t.Fatal("admin check without the account fails")
	}
	r.store.fail = ""
	r.store.fail = ""

	sealFail := accounts.New(r.store, hasher{}, cipher{failSeal: true}, &seqTokens{}, r.clock)
	if err := sealFail.SetKey(ctx, a.ID, domain.KeyAnthropic, good); !errors.Is(err, errBoom) {
		t.Fatalf("seal failure: %v", err)
	}
	openFail := accounts.New(r.store, hasher{}, cipher{failOpen: true}, &seqTokens{}, r.clock)
	if _, err := openFail.Credential(ctx, a.ID, domain.KeyOpenAI); !errors.Is(err, errBoom) {
		t.Fatalf("open failure: %v", err)
	}
}

func TestSponsoredPlayersPlayOnTheirSponsorsKey(t *testing.T) {
	t.Parallel()
	r := setup(t)
	_, _ = r.uc.Bootstrap(ctx, "james", pw)
	admin, _ := r.store.AccountByUsername(ctx, "james")
	if _, err := r.uc.CreateInvite(ctx, admin.ID, true); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("sponsoring needs a key of your own: %v", err)
	}
	_ = r.uc.SetKey(ctx, admin.ID, domain.KeyAnthropic, "sk-ant-admin-0123456789abcdef")
	inv, err := r.uc.CreateInvite(ctx, admin.ID, true)
	if err != nil || !inv.Sponsored {
		t.Fatalf("sponsored invite %+v %v", inv, err)
	}
	if _, err := r.uc.Register(ctx, inv.Code, "pip", pw); err != nil {
		t.Fatal(err)
	}
	pip, _ := r.store.AccountByUsername(ctx, "pip")
	if pip.SponsoredBy != admin.ID || pip.SponsorCap != domain.DefaultSponsorCap {
		t.Fatalf("sponsored on registration: %+v", pip)
	}
	key, err := r.uc.Credential(ctx, pip.ID, domain.KeyAnthropic)
	if err != nil || key != "sk-ant-admin-0123456789abcdef" {
		t.Fatalf("plays on the sponsor's key: %q %v", key, err)
	}
	if _, err := r.uc.Credential(ctx, pip.ID, domain.KeyOpenAI); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("only Anthropic is sponsored: %v", err)
	}
	if spend, _ := r.uc.Today(ctx, pip.ID); spend.SponsorCap != domain.DefaultSponsorCap {
		t.Fatalf("spend shows the sponsor's cap: %+v", spend)
	}
	_ = r.uc.RecordUsage(ctx, pip.ID, domain.DefaultSponsorCap)
	if _, err := r.uc.Credential(ctx, pip.ID, domain.KeyAnthropic); !errors.Is(err, domain.ErrOverBudget) {
		t.Fatalf("within what the sponsor pays for: %v", err)
	}
	_ = r.uc.SetKey(ctx, pip.ID, domain.KeyAnthropic, "sk-ant-pip-0123456789abcdefgh")
	_ = r.uc.SetDailyTokenCap(ctx, pip.ID, domain.MaxTokenCap)
	if key, err := r.uc.Credential(ctx, pip.ID, domain.KeyAnthropic); err != nil || key != "sk-ant-pip-0123456789abcdefgh" {
		t.Fatalf("a key of their own comes first: %q %v", key, err)
	}
	_ = r.uc.SetKey(ctx, pip.ID, domain.KeyAnthropic, "")
	if _, err := r.uc.Credential(ctx, pip.ID, domain.KeyAnthropic); !errors.Is(err, domain.ErrOverBudget) {
		t.Fatalf("raising their own cap doesn't raise the sponsor's: %v", err)
	}
	_ = r.uc.SetKey(ctx, admin.ID, domain.KeyAnthropic, "")
	if _, err := r.uc.Credential(ctx, pip.ID, domain.KeyAnthropic); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("the sponsor removed their key: %v", err)
	}
	if _, err := r.uc.Register(ctx, "nope", "sam", pw); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("unknown invite: %v", err)
	}
}

func TestAdminsSponsorPlayers(t *testing.T) {
	t.Parallel()
	r := setup(t)
	_, _ = r.uc.Bootstrap(ctx, "james", pw)
	admin, _ := r.store.AccountByUsername(ctx, "james")
	inv, _ := r.uc.CreateInvite(ctx, admin.ID, false)
	_, _ = r.uc.Register(ctx, inv.Code, "pip", pw)
	pip, _ := r.store.AccountByUsername(ctx, "pip")
	if _, err := r.uc.Credential(ctx, pip.ID, domain.KeyAnthropic); !errors.Is(err, domain.ErrNoKey) {
		t.Fatalf("not sponsored yet: %v", err)
	}
	cases := []struct {
		name      string
		by, whom  string
		tokenCap  int
		want      error
		sponsored bool
	}{
		{"players can't sponsor", pip.ID, admin.ID, 100, domain.ErrForbidden, false},
		{"not yourself", admin.ID, admin.ID, 100, domain.ErrInvalid, false},
		{"a sensible cap", admin.ID, pip.ID, -1, domain.ErrInvalid, false},
		{"someone real", admin.ID, "ghost", 100, domain.ErrNotFound, false},
		{"with a key to pay from", admin.ID, pip.ID, 100, domain.ErrNoKey, false},
		{"stopping needs no key", admin.ID, pip.ID, 0, nil, false},
	}
	for _, tc := range cases {
		if err := r.uc.Sponsor(ctx, tc.by, tc.whom, tc.tokenCap); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	_ = r.uc.SetKey(ctx, admin.ID, domain.KeyAnthropic, "sk-ant-admin-0123456789abcdef")
	if err := r.uc.Sponsor(ctx, admin.ID, pip.ID, 1000); err != nil {
		t.Fatal(err)
	}
	_ = r.uc.RecordUsage(ctx, pip.ID, 40)
	players, err := r.uc.Players(ctx, admin.ID)
	if err != nil || len(players) != 2 {
		t.Fatalf("players %+v %v", players, err)
	}
	byName := map[string]accounts.Player{}
	for _, p := range players {
		byName[p.Username] = p
	}
	if p := byName["pip"]; !p.Sponsored || p.SponsorCap != 1000 || p.UsedToday != 40 || p.OwnKey {
		t.Fatalf("pip %+v", p)
	}
	if p := byName["james"]; !p.Admin || !p.OwnKey || p.Sponsored {
		t.Fatalf("james %+v", p)
	}
	if _, err := r.uc.Players(ctx, pip.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("admins only: %v", err)
	}
}

func TestSponsorFailures(t *testing.T) {
	t.Parallel()
	r := setup(t)
	_, _ = r.uc.Bootstrap(ctx, "james", pw)
	admin, _ := r.store.AccountByUsername(ctx, "james")
	_ = r.uc.SetKey(ctx, admin.ID, domain.KeyAnthropic, "sk-ant-admin-0123456789abcdef")
	inv, _ := r.uc.CreateInvite(ctx, admin.ID, true)
	_, _ = r.uc.Register(ctx, inv.Code, "pip", pw)
	pip, _ := r.store.AccountByUsername(ctx, "pip")
	cases := []struct {
		op string
		fn func() error
	}{
		{"key", func() error { _, err := r.uc.CreateInvite(ctx, admin.ID, true); return err }},
		{"accounts", func() error { _, err := r.uc.Players(ctx, admin.ID); return err }},
		{"key", func() error { _, err := r.uc.Players(ctx, admin.ID); return err }},
		{"usage", func() error { _, err := r.uc.Players(ctx, admin.ID); return err }},
		{"usage", func() error { _, err := r.uc.Credential(ctx, pip.ID, domain.KeyAnthropic); return err }},
		{"sponsor", func() error { return r.uc.Sponsor(ctx, admin.ID, pip.ID, 0) }},
	}
	for _, c := range cases {
		r.store.fail = c.op
		if err := c.fn(); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", c.op, err)
		}
	}
	r.store.fail, r.store.failKey = "", admin.ID
	r.store.accounts[pip.ID] = pip // still sponsored
	if _, err := r.uc.Credential(ctx, pip.ID, domain.KeyAnthropic); !errors.Is(err, errBoom) {
		t.Fatalf("the sponsor's key can't be read: %v", err)
	}
	r.store.fail, r.store.failKey = "account", ""
	if _, err := r.uc.Credential(ctx, pip.ID, domain.KeyAnthropic); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("sponsored lookup without the account: %v", err)
	}
	r.store.fail = "loadinvite"
	if _, err := r.uc.Register(ctx, "x", "sam", pw); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("invite lookup: %v", err)
	}
}
