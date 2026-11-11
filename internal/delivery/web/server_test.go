package web_test

import (
	"bufio"
	"context"
	"dnd/internal/adapter/hub"
	"dnd/internal/delivery/web"
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"dnd/internal/usecase/accounts"
	"dnd/internal/usecase/characters"
	"dnd/internal/usecase/economy"
	"dnd/internal/usecase/market"
	"dnd/internal/usecase/newgame"
	"dnd/internal/usecase/speech"
	"dnd/internal/usecase/tavern"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

type clock struct{}

func (clock) Now() time.Time { return time.Unix(0, 0) }

type fakeCreator struct{ got newgame.Input }

func (*fakeCreator) Options() newgame.Options { return newgame.Options{} }
func (f *fakeCreator) Execute(_ context.Context, in newgame.Input) (newgame.Output, error) {
	f.got = in
	return newgame.Output{GameID: "g1"}, nil
}

type fakeLibrary struct{ err error }

func (f fakeLibrary) List(context.Context, string) ([]domain.Summary, error) {
	return []domain.Summary{{ID: "g1", Title: "Quest", SceneKey: "tavern", Scene: "The Gilded Flagon"}, {ID: "g2", Title: "Fresh"}}, f.err
}

func (f fakeLibrary) Get(_ context.Context, accountID, id string) (*domain.Game, error) {
	if f.err != nil {
		return nil, f.err
	}
	if accountID != "u1" {
		return nil, domain.ErrNotFound
	}
	return &domain.Game{
		ID: id, LeaderID: "u1", Transcripts: map[string][]domain.AgentMessage{"dm": {{Text: "secret"}}},
		Mode: domain.ModeExploration, Started: true, Awaiting: true, Party: []string{"pc1", "pc2"}, Scene: domain.Scene{Key: "tavern"},
		NPCs: []*domain.NPC{{ID: "npc-snik", Name: "Snik", Type: "kobold", Monster: true, Here: true}},
		Creatures: []*domain.Creature{
			{ID: "pc1", Name: "Aria", OwnerID: "u1", Kind: domain.KindPC, Controller: domain.ControlHuman, HP: 5},
			{ID: "pc2", Name: "Mae", OwnerID: "friend", Kind: domain.KindPC, Controller: domain.ControlHuman, HP: 5},
		},
	}, nil
}
func (f fakeLibrary) Delete(context.Context, string, string) error { return f.err }

type fakeTable struct {
	err  error
	text string
	busy bool
}

func (f *fakeTable) Autopilot(_ context.Context, _, _, c string, on bool) error {
	f.text = fmt.Sprintf("autopilot %s %v", c, on)
	return f.err
}

func (f *fakeTable) Cover(_ context.Context, _, _, c string, allow bool) error {
	f.text = fmt.Sprintf("cover %s %v", c, allow)
	return f.err
}

func (f *fakeTable) Recruit(_ context.Context, _, _, c string) error {
	f.text = "recruit " + c
	return f.err
}

func (f *fakeTable) SendHome(_ context.Context, _, _, c string) error {
	f.text = "send home " + c
	return f.err
}

func (f *fakeTable) SetTyping(_ context.Context, _, _ string, m domain.TypingMode) error {
	f.text = "typing " + string(m)
	return f.err
}

func (f *fakeTable) Equip(_ context.Context, _, _, c, item string, slot domain.Slot) error {
	f.text = "equip " + c + " " + item + " " + string(slot)
	return f.err
}

func (f *fakeTable) Improve(_ context.Context, _, _, c string, inc map[domain.Ability]int) error {
	f.text = "improve " + c + " " + domain.ImprovementText(inc)
	return f.err
}

func (f *fakeTable) OfferSeat(_ context.Context, _, _, c string, open bool) error {
	f.text = fmt.Sprintf("offer %s %v", c, open)
	return f.err
}

func (f *fakeTable) PostWanted(_ context.Context, _, _, text string) error {
	f.text = "wanted " + text
	return f.err
}

func (f *fakeTable) AskForHero(_ context.Context, _, _, c string) error {
	f.text = "ask " + c
	return f.err
}

func (f *fakeTable) AnswerCall(_ context.Context, _, _, c string, yes bool) error {
	f.text = fmt.Sprintf("answer %s %v", c, yes)
	return f.err
}

func (f *fakeTable) GiveSeatBack(_ context.Context, _, _, c string) error {
	f.text = "give back " + c
	return f.err
}

type fakeBoard struct {
	err  error
	took string
}

func (f *fakeBoard) Board(context.Context, string) ([]tavern.Seat, error) {
	return []tavern.Seat{{GameID: "g9", Title: "Tale", Leader: "lead", Character: domain.Creature{ID: "pc1", Name: "Bram", Kind: domain.KindPC, Race: "Elf", Class: "Wizard"}}}, f.err
}

func (f *fakeBoard) Offer(_ context.Context, a, g, c string) error {
	f.took = "offer " + a + " " + g + " " + c
	return f.err
}

func (f *fakeBoard) Notices(context.Context, string) (tavern.Notices, error) {
	return tavern.Notices{Posts: []tavern.Post{{GameID: "g9", Text: "A healer"}}}, f.err
}

func (f *fakeBoard) Take(_ context.Context, a, g, c string) error {
	f.took = a + " " + g + " " + c
	return f.err
}

func (f *fakeTable) CheckCast(context.Context, string, string) error {
	f.text = "cast"
	return f.err
}
func (f *fakeTable) Limits() (int, int)                          { return 60000, 25000 }
func (f *fakeTable) Busy(string) bool                            { return f.busy }
func (f *fakeTable) Begin(context.Context, string, string) error { return f.err }
func (f *fakeTable) Fight(context.Context, string, string, rules.Option) error {
	return f.err
}

func (f *fakeTable) Act(_ context.Context, _, _, text string) error {
	f.text = text
	return f.err
}

func (f *fakeTable) Options(context.Context, string, string) ([]rules.Option, error) {
	return []rules.Option{{ActionID: "longsword"}}, nil
}

type fakeGallery struct{ err error }

func (fakeGallery) CanPaint(context.Context, string) bool { return true }

func (fakeGallery) Pending(key string) bool                                { return strings.HasSuffix(key, "/pc2") }
func (fakeGallery) Ensure(*domain.Game)                                    {}
func (fakeGallery) EnsureCharacter(*domain.Character)                      {}
func (f fakeGallery) Reroll(context.Context, string, string, string) error { return f.err }
func (fakeGallery) Picture(key string, stockKeys ...string) (string, bool) {
	if strings.HasSuffix(key, "/pc1") {
		return "/media/" + key + ".webp", true
	}
	for _, k := range stockKeys {
		if k == "scenes/tavern" || k == "battlemaps/tavern" || k == "monsters/kobold" || k == "npcs/innkeeper-f" {
			return "/stock/candlelit/" + k + ".webp", true
		}
	}
	return "", false
}

type fakeMarket struct {
	err  error
	last string
}

func (f *fakeMarket) Browse(context.Context, string) (market.View, error) {
	l := domain.Listing{ID: "l1", SellerID: "friend", Price: 20, Item: domain.Item{Name: "Sword"}}
	return market.View{Open: []domain.Listing{l}, Mine: []domain.Listing{}, FeePercent: 12, BuyBackPercent: 45}, f.err
}

type fakeSpeech struct {
	err  error
	last string
}

func (f *fakeSpeech) Execute(_ context.Context, in speech.Input) (speech.Output, error) {
	f.last = string(in.Style)
	return speech.Output{Audio: []byte("ID3 " + in.AccountID + " " + in.Voice), ContentType: "audio/mpeg"}, f.err
}

type fakeEconomy struct {
	err error
	set domain.Economy
}

func (f *fakeEconomy) Report(context.Context, string) (economy.Report, error) {
	return economy.Report{Supply: 777}, f.err
}

func (f *fakeEconomy) SetEconomy(_ context.Context, _ string, e domain.Economy) error {
	f.set = e
	return f.err
}

func (f *fakeMarket) List(_ context.Context, in market.ListInput) (domain.Listing, error) {
	f.last = "list " + in.AccountID + " " + in.ItemID
	return domain.Listing{ID: "l2"}, f.err
}

func (f *fakeMarket) Buy(_ context.Context, a, l, c string) error {
	f.last = "buy " + a + " " + l + " " + c
	return f.err
}

func (f *fakeMarket) Withdraw(_ context.Context, a, l, c string) error {
	f.last = "withdraw " + l + " " + c
	return f.err
}

func (f *fakeMarket) Collect(_ context.Context, a, l, c string) error {
	f.last = "collect " + l + " " + c
	return f.err
}

func (f *fakeMarket) Shop() []market.ShopItem {
	return []market.ShopItem{{BaseItem: domain.BaseItem{Key: "rope"}, Price: 1}}
}

func (f *fakeMarket) ShopBuy(_ context.Context, a, c, key string, qty int) error {
	f.last = fmt.Sprintf("shopbuy %s %s %d", c, key, qty)
	return f.err
}

func (f *fakeMarket) ShopSell(_ context.Context, a, c, item string) (int, error) {
	f.last = "shopsell " + c + " " + item
	return 7, f.err
}

type fakeRoster struct {
	err    error
	got    characters.CreateInput
	hire   domain.HirePolicy
	gender domain.Gender
	equip  string
	// improved is the last Ability Score Improvement asked for.
	improved string
}

func (f *fakeRoster) Create(_ context.Context, in characters.CreateInput) (domain.Character, error) {
	f.got = in
	return domain.Character{ID: "ch1", Sheet: &domain.Creature{Name: in.Name}}, f.err
}

func (f *fakeRoster) Random(_ context.Context, in characters.RandomInput) (domain.Character, error) {
	f.got.AccountID = in.AccountID
	return domain.Character{ID: "rnd", Sheet: &domain.Creature{Name: "Pip"}}, f.err
}

func (f *fakeRoster) List(context.Context, string) ([]domain.Character, error) {
	return []domain.Character{{ID: "pc1", Status: domain.CharIdle}}, f.err
}

func (f *fakeRoster) ForHire(context.Context, string) ([]domain.Character, error) {
	return []domain.Character{{ID: "pip", OwnerID: "friend", Status: domain.CharIdle, Hire: domain.HireAuto}}, f.err
}

func (f *fakeRoster) Get(_ context.Context, _, id string) (domain.Character, error) {
	return domain.Character{ID: id, Status: domain.CharIdle}, f.err
}

func (f *fakeRoster) SetHire(_ context.Context, _, _ string, p domain.HirePolicy) error {
	f.hire = p
	return f.err
}

func (f *fakeRoster) Improve(_ context.Context, _, id string, inc map[domain.Ability]int) error {
	f.improved = id + " " + domain.ImprovementText(inc)
	return f.err
}

func (f *fakeRoster) SetGender(_ context.Context, _, _ string, g domain.Gender) error {
	f.gender = g
	return f.err
}

func (f *fakeRoster) Equip(_ context.Context, _, id, item string, slot domain.Slot) error {
	f.equip = id + " " + item + " " + string(slot)
	return f.err
}

func (f *fakeRoster) Retire(context.Context, string, string) error { return f.err }

// fakeAccounts knows one user "u1" signed in with token "good".
type fakeAccounts struct {
	err     error
	key     string
	cap     int
	model   string
	logout  string
	sponsor int
}

var user = domain.Account{ID: "u1", Username: "james", Admin: true}

func (f *fakeAccounts) Authenticate(_ context.Context, token string) (domain.Account, error) {
	if token != "good" {
		return domain.Account{}, domain.ErrUnauthorized
	}
	return user, nil
}

func (f *fakeAccounts) Login(_ context.Context, u, p string) (domain.Session, error) {
	if u != "james" || p != "pw" {
		return domain.Session{}, domain.ErrUnauthorized
	}
	return domain.Session{Token: "good", AccountID: "u1", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (f *fakeAccounts) Register(_ context.Context, code, _, _ string) (domain.Session, error) {
	if code != "inv" {
		return domain.Session{}, domain.ErrInvalid
	}
	return domain.Session{Token: "good", AccountID: "u1"}, nil
}

func (f *fakeAccounts) Logout(_ context.Context, token string) error {
	f.logout = token
	return f.err
}

func (f *fakeAccounts) Keys(context.Context, string) ([]domain.KeyInfo, error) {
	return []domain.KeyInfo{{Kind: domain.KeyAnthropic, Set: true, Last4: "WXYZ"}}, f.err
}

func (f *fakeAccounts) SetKey(_ context.Context, _ string, _ domain.KeyKind, secret string) error {
	f.key = secret
	return f.err
}

func (f *fakeAccounts) SetDailyTokenCap(_ context.Context, _ string, c int) error {
	f.cap = c
	return f.err
}

func (f *fakeAccounts) SetAgentModel(_ context.Context, _, model string) error {
	f.model = model
	return f.err
}

func (f *fakeAccounts) AgentModel(context.Context, string) (string, error) {
	return f.model, f.err
}

func (f *fakeAccounts) Username(_ context.Context, id string) (string, error) {
	return "name-of-" + id, f.err
}

func (f *fakeAccounts) Today(context.Context, string) (accounts.Spend, error) {
	return accounts.Spend{Used: 5, Cap: 10}, f.err
}

func (f *fakeAccounts) CreateInvite(_ context.Context, _ string, sponsored bool) (domain.Invite, error) {
	return domain.Invite{Code: "new-invite", Sponsored: sponsored}, f.err
}

func (f *fakeAccounts) Players(context.Context, string) ([]accounts.Player, error) {
	return []accounts.Player{{ID: "u2", Username: "pip", Sponsored: true, SponsorCap: f.sponsor}}, f.err
}

func (f *fakeAccounts) Sponsor(_ context.Context, _, _ string, c int) error {
	f.sponsor = c
	return f.err
}

func (f *fakeAccounts) Invites(context.Context, string) ([]domain.Invite, error) {
	return []domain.Invite{{Code: "inv"}}, f.err
}

type rig struct {
	roster  *fakeRoster
	srv     *httptest.Server
	table   *fakeTable
	creator *fakeCreator
	acct    *fakeAccounts
	market  *fakeMarket
	econ    *fakeEconomy
	board   *fakeBoard
}

func setup(t *testing.T, lib fakeLibrary, mutate func(*web.Server)) rig {
	t.Helper()
	r := rig{table: &fakeTable{err: lib.err}, creator: &fakeCreator{}, acct: &fakeAccounts{}, roster: &fakeRoster{err: lib.err}, market: &fakeMarket{err: lib.err}, econ: &fakeEconomy{err: lib.err}, board: &fakeBoard{err: lib.err}}
	s := web.New(web.Deps{
		Creator: r.creator, Roster: r.roster, Library: lib, Table: r.table, Events: hub.New(), Gallery: fakeGallery(lib),
		Auth: r.acct, Settings: r.acct, Market: r.market, Economy: r.econ, Speech: &fakeSpeech{err: lib.err}, Board: r.board, Roles: []domain.AgentRole{{Name: "dm", Model: "claude-opus-5-5", Prompt: "secret prompt"}},
		Static: fstest.MapFS{"index.html": {Data: []byte("<h1>app</h1>")}},
		Media:  fstest.MapFS{"games/g1/pc1.webp": {Data: []byte("WEBP")}},
		Clock:  clock{}, Log: slog.New(slog.DiscardHandler),
	})
	if mutate != nil {
		mutate(s)
	}
	r.srv = httptest.NewServer(s.Handler())
	t.Cleanup(r.srv.Close)
	return r
}

// do sends a request, signed in unless token is empty.
func do(t *testing.T, method, url, body, token string, headers ...string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(body))
	if token != "" {
		req.AddCookie(&http.Cookie{Name: web.CookieName, Value: token})
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func TestSignedInRoutes(t *testing.T) {
	t.Parallel()
	r := setup(t, fakeLibrary{}, nil)
	cases := []struct {
		method, path, body string
		status             int
		contains           string
	}{
		{"GET", "/api/options", "", 200, "classes"},
		{"GET", "/api/games", "", 200, `"sceneKey":"tavern","updatedAt":"0001-01-01T00:00:00Z","art":"/stock/candlelit/scenes/tavern.webp"},{"yourTurn":false,"id":"g2"`},
		{"POST", "/api/games", `{"hero":{"name":"A"}}`, 201, "g1"},
		{"POST", "/api/games", `{`, 400, "error"},
		{"GET", "/api/games/g1", "", 200, "longsword"},
		{"GET", "/api/games/g1", "", 200, `"log":[]`},
		{"GET", "/api/tavern/seats", "", 200, `"portrait":"/media/characters/pc1.webp"`},
		{"POST", "/api/tavern/seats/take", `{`, 400, ""},
		{"GET", "/api/tavern/notices", "", 200, `"text":"A healer"`},
		{"POST", "/api/tavern/offer", `{`, 400, ""},
		{"POST", "/api/games/g1/wanted", `{`, 400, ""},
		{"POST", "/api/games/g1/ask", `{`, 400, ""},
		{"POST", "/api/games/g1/answer", `{`, 400, ""},
		{"POST", "/api/games/g1/seats", `{`, 400, ""},
		{"POST", "/api/games/g1/give-back", `{`, 400, ""},
		{"DELETE", "/api/games/g1", "", 204, ""},
		{"POST", "/api/games/g1/begin", "", 204, ""},
		{"POST", "/api/games/g1/act", `{"text":"hello"}`, 204, ""},
		{"POST", "/api/games/g1/act", `nope`, 400, ""},
		{"POST", "/api/games/g1/fight", `{"actionId":"longsword"}`, 204, ""},
		{"POST", "/api/games/g1/fight", `[`, 400, ""},
		{"POST", "/api/games/g1/autopilot", `{"characterId":"pc1","on":true}`, 204, ""},
		{"POST", "/api/games/g1/autopilot", `{`, 400, ""},
		{"POST", "/api/games/g1/cover", `{"characterId":"pc1","allow":false}`, 204, ""},
		{"POST", "/api/games/g1/cover", `{`, 400, ""},
		{"POST", "/api/games/g1/recruit", `{"characterId":"nim"}`, 204, ""},
		{"POST", "/api/games/g1/recruit", `{`, 400, ""},
		{"POST", "/api/games/g1/send-home", `{`, 400, ""},
		{"POST", "/api/games/g1/cast", "", 204, ""},
		{"PUT", "/api/games/g1/typing", `{"mode":"loud"}`, 400, "unknown"},
		{"PUT", "/api/games/g1/typing", `{`, 400, ""},
		{"POST", "/api/games/g1/art/pc1", "", 202, ""},
		{"GET", "/media/games/g1/pc1.webp", "", 200, "WEBP"},
		{"GET", "/", "", 200, "<h1>app</h1>"},
		{"GET", "/play/g1", "", 200, "<h1>app</h1>"},
		{"GET", "/assets/index-gone.js", "", 404, "not found"},
		{"GET", "/api/me", "", 200, `"last4":"WXYZ"`},
		{"GET", "/api/me", "", 200, `"backdrop":"/stock/candlelit/scenes/tavern.webp"`},
		{"GET", "/api/me", "", 200, `"models":[{"id":"claude-opus-5-5"`},
		{"GET", "/api/me", "", 200, `"roles":[{"name":"dm","description":"","model":"claude-opus-5-5"`},
		{"PUT", "/api/me/keys/anthropic", `{"key":"sk-ant-x"}`, 204, ""},
		{"PUT", "/api/me/keys/bitcoin", `{"key":"x"}`, 400, "unknown"},
		{"PUT", "/api/me/keys/openai", `{`, 400, ""},
		{"PUT", "/api/me/cap", `{"cap":500}`, 204, ""},
		{"PUT", "/api/me/cap", `{`, 400, ""},
		{"PUT", "/api/me/agent-model", `{"model":"claude-haiku-5-5"}`, 204, ""},
		{"PUT", "/api/me/agent-model", `{`, 400, ""},
		{"GET", "/api/admin/invites", "", 200, "inv"},
		{"POST", "/api/admin/invites", "", 201, "new-invite"},
		{"POST", "/api/admin/invites", `{"sponsored":true}`, 201, `"sponsored":true`},
		{"POST", "/api/admin/invites", `{`, 400, ""},
		{"PUT", "/api/admin/players/u2/sponsor", `{"tokenCap":300}`, 204, ""},
		{"PUT", "/api/admin/players/u2/sponsor", `{`, 400, ""},
		{"GET", "/api/admin/players", "", 200, `"sponsorCap":300`},
		{"GET", "/api/characters", "", 200, `"portrait":"/media/characters/pc1.webp"`},
		{"GET", "/api/tavern", "", 200, `"ownerName":"name-of-friend"`},
		{"GET", "/api/market", "", 200, `"sellerName":"name-of-friend","mine":false`},
		{"GET", "/api/market", "", 200, `"feePercent":12`},
		{"GET", "/api/admin/economy", "", 200, `"supply":777`},
		{"PUT", "/api/admin/economy", `{"lootPercent":50,"marketFeePercent":15,"shopBuyBackPercent":40}`, 204, ""},
		{"PUT", "/api/admin/economy", `{`, 400, ""},
		{"POST", "/api/speech", `{"text":"Hello","voice":"fable","style":"narrator"}`, 200, "ID3 u1 fable"},
		{"POST", "/api/speech", `{`, 400, ""},
		{"POST", "/api/market/listings", `{"characterId":"pc1","itemId":"sword","price":20}`, 201, `"id":"l2"`},
		{"POST", "/api/market/listings", `{`, 400, ""},
		{"POST", "/api/market/listings/l1/buy", `{"characterId":"pc1"}`, 204, ""},
		{"POST", "/api/market/listings/l1/withdraw", `{"characterId":"pc1"}`, 204, ""},
		{"POST", "/api/market/listings/l1/collect", `{"characterId":"pc1"}`, 204, ""},
		{"POST", "/api/market/listings/l1/steal", `{}`, 404, ""},
		{"POST", "/api/market/listings/l1/buy", `{`, 400, ""},
		{"GET", "/api/shop", "", 200, `"buyBackPercent":45`},
		{"POST", "/api/shop/buy", `{"characterId":"pc1","key":"rope","qty":2}`, 204, ""},
		{"POST", "/api/shop/buy", `{`, 400, ""},
		{"POST", "/api/shop/sell", `{"characterId":"pc1","itemId":"sword"}`, 200, `"paid":7`},
		{"POST", "/api/shop/sell", `{`, 400, ""},
		{"POST", "/api/characters", `{"name":"Aria","class":"Fighter"}`, 201, `"id":"ch1"`},
		{"POST", "/api/characters", `{`, 400, ""},
		{"POST", "/api/characters/random", `{"avoid":["Fighter"]}`, 201, `"id":"rnd"`},
		{"POST", "/api/characters/random", `{`, 400, ""},
		{"GET", "/api/characters/pc2", "", 200, `"painting":true`},
		{"GET", "/api/characters/pc2", "", 200, `"xpLevels":[0,300,900,2700,6500]`},
		{"PUT", "/api/characters/pc1/hire", `{"hire":"ask"}`, 204, ""},
		{"PUT", "/api/characters/pc1/hire", `{"hire":"maybe"}`, 400, "unknown"},
		{"PUT", "/api/characters/pc1/hire", `{`, 400, ""},
		{"PUT", "/api/characters/pc1/gender", `{"gender":"f"}`, 204, ""},
		{"PUT", "/api/characters/pc1/gender", `{`, 400, ""},
		{"PUT", "/api/characters/pc1/improve", `{"increases":{"STR":2}}`, 204, ""},
		{"PUT", "/api/characters/pc1/improve", `{`, 400, ""},
		{"PUT", "/api/characters/pc1/equip", `{"itemId":"sword","slot":"main"}`, 204, ""},
		{"PUT", "/api/characters/pc1/equip", `{"itemId":"sword","slot":"head"}`, 400, "unknown"},
		{"PUT", "/api/characters/pc1/equip", `{`, 400, ""},
		{"POST", "/api/games/g1/equip", `{"characterId":"pc1","itemId":"ring","slot":"trinket1"}`, 204, ""},
		{"POST", "/api/games/g1/equip", `{"slot":"head"}`, 400, "unknown"},
		{"DELETE", "/api/characters/pc1", "", 204, ""},
	}
	for _, c := range cases {
		res, body := do(t, c.method, r.srv.URL+c.path, c.body, "good")
		if res.StatusCode != c.status || !strings.Contains(body, c.contains) {
			t.Errorf("%s %s = %d %q", c.method, c.path, res.StatusCode, body)
		}
	}
	if r.roster.got.AccountID != "u1" || r.roster.hire != domain.HireAsk || r.roster.gender != domain.Female {
		t.Errorf("roster calls must act as the signed-in account: %+v", r.roster)
	}
	if r.table.text != "equip pc1 ring trinket1" || r.creator.got.AccountID != "u1" || r.acct.key != "sk-ant-x" || r.acct.cap != 500 || r.acct.model != "claude-haiku-5-5" {
		t.Errorf("handlers must act as the signed-in account: %+v %+v", r.creator.got, r.acct)
	}
	if do(t, "PUT", r.srv.URL+"/api/games/g1/typing", `{"mode":"choices"}`, "good"); r.table.text != "typing choices" {
		t.Errorf("typing passes the mode: %q", r.table.text)
	}
	if do(t, "POST", r.srv.URL+"/api/games/g1/cover", `{"characterId":"pc1","allow":false}`, "good"); r.table.text != "cover pc1 false" {
		t.Errorf("cover passes the character and choice: %q", r.table.text)
	}
	if r.roster.equip != "pc1 sword main" {
		t.Errorf("equip in town passes the item and slot: %q", r.roster.equip)
	}
	if do(t, "POST", r.srv.URL+"/api/games/g1/equip", `{"characterId":"pc1","itemId":"ring","slot":""}`, "good"); r.table.text != "equip pc1 ring " {
		t.Errorf("equip in a game passes the character, item and slot: %q", r.table.text)
	}
	if do(t, "POST", r.srv.URL+"/api/games/g1/seats", `{"characterId":"pc2","open":true}`, "good"); r.table.text != "offer pc2 true" {
		t.Errorf("offer seat: %q", r.table.text)
	}
	if do(t, "POST", r.srv.URL+"/api/games/g1/give-back", `{"characterId":"pc2"}`, "good"); r.table.text != "give back pc2" {
		t.Errorf("give back: %q", r.table.text)
	}
	for body, want := range map[string]string{
		"wanted": `{"text":"A healer"}|wanted A healer`,
		"ask":    `{"characterId":"h1"}|ask h1`,
		"answer": `{"characterId":"h1","yes":true}|answer h1 true`,
	} {
		in, out, _ := strings.Cut(want, "|")
		if do(t, "POST", r.srv.URL+"/api/games/g1/"+body, in, "good"); r.table.text != out {
			t.Errorf("%s: %q", body, r.table.text)
		}
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/tavern/offer", `{"gameId":"g9","characterId":"h1"}`, "good"); res.StatusCode != 204 || r.board.took != "offer u1 g9 h1" {
		t.Errorf("offer hero: %d %q", res.StatusCode, r.board.took)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/tavern/seats/take", `{"gameId":"g9","characterId":"pc1"}`, "good"); res.StatusCode != 204 || r.board.took != "u1 g9 pc1" {
		t.Errorf("take seat: %d %q", res.StatusCode, r.board.took)
	}
	if r.roster.improved != "pc1 STR +2" {
		t.Errorf("improve in town passes the choice: %q", r.roster.improved)
	}
	if do(t, "POST", r.srv.URL+"/api/games/g1/improve", `{"characterId":"pc1","increases":{"DEX":1,"CON":1}}`, "good"); r.table.text != "improve pc1 DEX +1, CON +1" {
		t.Errorf("improve in a game passes the character and choice: %q", r.table.text)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/games/g1/improve", `{`, "good"); res.StatusCode != 400 {
		t.Errorf("bad improve body: %d", res.StatusCode)
	}
	if do(t, "POST", r.srv.URL+"/api/games/g1/recruit", `{"characterId":"nim"}`, "good"); r.table.text != "recruit nim" {
		t.Errorf("recruit passes the character: %q", r.table.text)
	}
	if do(t, "POST", r.srv.URL+"/api/games/g1/send-home", `{"characterId":"pc2"}`, "good"); r.table.text != "send home pc2" {
		t.Errorf("send home passes the character: %q", r.table.text)
	}
	if do(t, "POST", r.srv.URL+"/api/games/g1/autopilot", `{"characterId":"pc1","on":true}`, "good"); r.table.text != "autopilot pc1 true" {
		t.Errorf("autopilot must pass the character and switch: %q", r.table.text)
	}
	if _, me := do(t, "GET", r.srv.URL+"/api/me", "", "good"); !strings.Contains(me, `"agentModel":"claude-haiku-5-5"`) || !strings.Contains(me, `"dailyTokenCap":10`) || strings.Contains(me, "secret prompt") {
		t.Errorf("/api/me reads settings fresh and never sends prompts: %s", me)
	}
	if res, _ := do(t, "GET", r.srv.URL+"/play/g1", "", "good"); res.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("the page must not be cached: %q", res.Header.Get("Cache-Control"))
	}
	_, body := do(t, "GET", r.srv.URL+"/api/games/g1", "", "good")
	if !strings.Contains(body, `"mine":["pc1"]`) || !strings.Contains(body, `"yourTurn":true`) || !strings.Contains(body, `"waitingOn":["Aria","Mae"]`) || !strings.Contains(body, `"paused":false`) ||
		!strings.Contains(body, `"you":"u1"`) || !strings.Contains(body, `"chapterAt":{"dm":60000,"companion":25000}`) ||
		!strings.Contains(body, `"atTavern":true`) || !strings.Contains(body, `"partyLimit":4`) || !strings.Contains(body, `"castStale":false`) || !strings.Contains(body, `"mayType":true`) || !strings.Contains(body, `"maxAction":500`) || !strings.Contains(body, `"choices":["Look around carefully"`) {
		t.Errorf("seat view: %s", body)
	}
	if strings.Contains(body, "secret") || !strings.Contains(body, `"art":{"battlemap":"/stock/candlelit/battlemaps/tavern.webp","npc-snik":"/stock/candlelit/monsters/kobold.webp","pc1":"/media/games/g1/pc1.webp","scene":"/stock/candlelit/scenes/tavern.webp"}`) || !strings.Contains(body, `"painting":["pc2"]`) {
		t.Errorf("game view: %s", body)
	}
	r.table.busy = true
	if _, body := do(t, "GET", r.srv.URL+"/api/games/g1", "", "good"); !strings.Contains(body, `"busy":true`) || !strings.Contains(body, `"awaiting":false`) {
		t.Errorf("a game mid-turn must not invite input: %s", body)
	}
}

func TestAuthFlow(t *testing.T) {
	t.Parallel()
	r := setup(t, fakeLibrary{}, nil)
	for _, path := range []string{"/api/games", "/api/me", "/media/games/g1/pc1.webp", "/api/games/g1/events"} {
		if res, _ := do(t, "GET", r.srv.URL+path, "", ""); res.StatusCode != 401 {
			t.Errorf("%s signed out = %d, want 401", path, res.StatusCode)
		}
	}
	if res, _ := do(t, "GET", r.srv.URL+"/api/games", "", "forged"); res.StatusCode != 401 {
		t.Errorf("bad token = %d", res.StatusCode)
	}
	res, _ := do(t, "POST", r.srv.URL+"/api/auth/login", `{"username":"james","password":"pw"}`, "", "X-Forwarded-Proto", "https")
	c := res.Cookies()
	if res.StatusCode != 204 || len(c) != 1 || c[0].Value != "good" || !c[0].HttpOnly || !c[0].Secure {
		t.Fatalf("login = %d cookies %+v", res.StatusCode, c)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/login", `{"username":"james","password":"no"}`, ""); res.StatusCode != 401 {
		t.Errorf("wrong password = %d", res.StatusCode)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/login", `{`, ""); res.StatusCode != 400 {
		t.Errorf("bad login body = %d", res.StatusCode)
	}
	res, _ = do(t, "POST", r.srv.URL+"/api/auth/register", `{"invite":"inv","username":"pip","password":"pw"}`, "")
	if res.StatusCode != 201 || len(res.Cookies()) != 1 {
		t.Errorf("register = %d", res.StatusCode)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/register", `{"invite":"bad"}`, ""); res.StatusCode != 400 {
		t.Errorf("bad invite = %d", res.StatusCode)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/register", `{`, ""); res.StatusCode != 400 {
		t.Errorf("bad register body = %d", res.StatusCode)
	}
	res, _ = do(t, "POST", r.srv.URL+"/api/auth/logout", "", "good")
	if res.StatusCode != 204 || r.acct.logout != "good" || res.Cookies()[0].MaxAge >= 0 {
		t.Errorf("logout = %d", res.StatusCode)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/logout", "", ""); res.StatusCode != 204 {
		t.Errorf("logout without a session = %d", res.StatusCode)
	}
	r.acct.err = errors.New("db down")
	if res, _ := do(t, "POST", r.srv.URL+"/api/auth/logout", "", "good"); res.StatusCode != 500 {
		t.Errorf("logout failure = %d", res.StatusCode)
	}
}

func TestCrossSiteWritesAreRefused(t *testing.T) {
	t.Parallel()
	r := setup(t, fakeLibrary{}, nil)
	res, _ := do(t, "POST", r.srv.URL+"/api/games/g1/act", `{"text":"x"}`, "good", "Origin", "https://evil.example")
	if res.StatusCode != 403 || r.table.text != "" {
		t.Fatalf("cross-site POST = %d", res.StatusCode)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/games/g1/act", `{"text":"x"}`, "good", "Origin", "::bad"); res.StatusCode != 403 {
		t.Fatalf("malformed origin = %d", res.StatusCode)
	}
	if res, _ := do(t, "POST", r.srv.URL+"/api/games/g1/act", `{"text":"ok"}`, "good", "Origin", r.srv.URL); res.StatusCode != 204 {
		t.Fatalf("same-origin POST = %d", res.StatusCode)
	}
	big := `{"text":"` + strings.Repeat("a", 2<<20) + `"}`
	if res, _ := do(t, "POST", r.srv.URL+"/api/games/g1/act", big, "good"); res.StatusCode != 400 && res.StatusCode != 413 {
		t.Fatalf("oversized body = %d", res.StatusCode)
	}
}

func TestLocalModeSignsInAutomatically(t *testing.T) {
	t.Parallel()
	r := setup(t, fakeLibrary{}, func(s *web.Server) { s.Local, s.Demo = &user, true })
	res, body := do(t, "GET", r.srv.URL+"/api/me", "", "")
	if res.StatusCode != 200 || !strings.Contains(body, `"local":true`) || !strings.Contains(body, `"demo":true`) {
		t.Fatalf("local me = %d %s", res.StatusCode, body)
	}
}

func TestErrorMapping(t *testing.T) {
	t.Parallel()
	cases := map[error]int{
		domain.ErrNotFound: 404, domain.ErrBusy: 409, domain.ErrWrongMode: 400, domain.ErrNotYourTurn: 400,
		domain.ErrForbidden: 403, domain.ErrNoKey: 400, domain.ErrOverBudget: 429, errors.New("boom"): 500,
	}
	for err, want := range cases {
		r := setup(t, fakeLibrary{err: err}, nil)
		r.acct.err = err
		for _, req := range [][2]string{
			{"GET", "/api/games/x"},
			{"POST", "/api/games/x/art/pc1"},
			{"POST", "/api/games/x/begin"},
			{"GET", "/api/games"},
			{"DELETE", "/api/games/x"},
			{"GET", "/api/me"},
			{"GET", "/api/admin/invites"},
			{"POST", "/api/admin/invites"},
			{"GET", "/api/admin/players"},
			{"GET", "/api/characters"},
			{"GET", "/api/tavern"},
			{"GET", "/api/market"},
			{"GET", "/api/shop"},
			{"GET", "/api/tavern/seats"},
			{"GET", "/api/tavern/notices"},
			{"GET", "/api/admin/economy"},
			{"GET", "/api/characters/x"},
			{"DELETE", "/api/characters/x"},
		} {
			if res, _ := do(t, req[0], r.srv.URL+req[1], "", "good"); res.StatusCode != want {
				t.Errorf("%v: %s %s = %d, want %d", err, req[0], req[1], res.StatusCode, want)
			}
		}
		if res, _ := do(t, "POST", r.srv.URL+"/api/characters", `{}`, "good"); res.StatusCode != want {
			t.Errorf("%v: create character = %d, want %d", err, res.StatusCode, want)
		}
		for _, req := range [][2]string{{"/api/market/listings", `{"price":1}`}, {"/api/market/listings/l1/buy", `{}`}, {"/api/shop/buy", `{}`}, {"/api/shop/sell", `{}`}, {"/api/speech", `{}`}} {
			if res, _ := do(t, "POST", r.srv.URL+req[0], req[1], "good"); res.StatusCode != want {
				t.Errorf("%v: POST %s = %d, want %d", err, req[0], res.StatusCode, want)
			}
		}
		for _, req := range [][2]string{{"/api/me/keys/anthropic", `{"key":"k"}`}, {"/api/me/cap", `{"cap":1}`}, {"/api/characters/x/hire", `{"hire":"no"}`}, {"/api/characters/x/equip", `{"itemId":"i","slot":"main"}`}} {
			if res, _ := do(t, "PUT", r.srv.URL+req[0], req[1], "good"); res.StatusCode != want {
				t.Errorf("%v: PUT %s = %d, want %d", err, req[0], res.StatusCode, want)
			}
		}
	}
}

// todayFails answers keys but fails the spend lookup.
type todayFails struct{ fakeAccounts }

func (*todayFails) Today(context.Context, string) (accounts.Spend, error) {
	return accounts.Spend{}, errors.New("usage db down")
}

func TestMeSpendFailure(t *testing.T) {
	t.Parallel()
	bad := &todayFails{}
	srv := web.New(web.Deps{Auth: bad, Settings: bad, Clock: clock{}, Log: slog.New(slog.DiscardHandler)})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	if res, _ := do(t, "GET", ts.URL+"/api/me", "", "good"); res.StatusCode != 500 {
		t.Fatalf("spend failure = %d", res.StatusCode)
	}
}

func TestDebugMode(t *testing.T) {
	t.Parallel()
	off := setup(t, fakeLibrary{}, nil)
	if res, body := do(t, "GET", off.srv.URL+"/api/games/g1/debug", "", "good"); strings.Contains(body, "secret") {
		t.Errorf("transcripts must not be served outside debug mode (%d)", res.StatusCode)
	}
	on := setup(t, fakeLibrary{}, func(s *web.Server) { s.Debug = true })
	if res, body := do(t, "GET", on.srv.URL+"/api/games/g1/debug", "", "good"); res.StatusCode != 200 || !strings.Contains(body, "secret") {
		t.Errorf("debug endpoint = %d %q", res.StatusCode, body)
	}
	missing := setup(t, fakeLibrary{err: domain.ErrNotFound}, func(s *web.Server) { s.Debug = true })
	if res, _ := do(t, "GET", missing.srv.URL+"/api/games/nope/debug", "", "good"); res.StatusCode != 404 {
		t.Errorf("missing game = %d", res.StatusCode)
	}
}

func TestEventStream(t *testing.T) {
	t.Parallel()
	h := hub.New()
	r := setup(t, fakeLibrary{}, func(s *web.Server) { s.Events = h })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", r.srv.URL+"/api/games/g1/events", nil)
	req.AddCookie(&http.Cookie{Name: web.CookieName, Value: "good"})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	rd := bufio.NewReader(res.Body)
	if line, _ := rd.ReadString('\n'); !strings.HasPrefix(line, ": connected") {
		t.Fatalf("first line %q", line)
	}
	h.Publish("g1", domain.Event{Type: domain.EvNarration, Text: "Hello there"})
	for {
		line, err := rd.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "data: ") {
			if !strings.Contains(line, "Hello there") {
				t.Fatalf("event %q", line)
			}
			return
		}
	}
}
