package market_test

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/usecase/market"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"
)

var ctx = context.Background()

var errBoom = errors.New("boom")

// memStore keeps JSON copies; Atomically works on a copy and only keeps it if
// the function succeeds, like a real transaction.
type memStore struct {
	chars    map[string][]byte
	listings map[string][]byte
	ledger   []domain.LedgerEntry
	fail     string
}

func newStore() *memStore {
	return &memStore{chars: map[string][]byte{}, listings: map[string][]byte{}}
}

func (m *memStore) put(ch domain.Character) {
	b, _ := json.Marshal(ch)
	m.chars[ch.ID] = b
}

func (m *memStore) char(t *testing.T, id string) domain.Character {
	t.Helper()
	var ch domain.Character
	if err := json.Unmarshal(m.chars[id], &ch); err != nil {
		t.Fatal(err)
	}
	return ch
}

func (m *memStore) all(seller string) []domain.Listing {
	var out []domain.Listing
	for _, b := range m.listings {
		var l domain.Listing
		_ = json.Unmarshal(b, &l)
		if seller == "" && l.Status == domain.ListingOpen || seller != "" && l.SellerID == seller {
			out = append(out, l)
		}
	}
	return out
}

func (m *memStore) Economy(context.Context) (domain.Economy, error) {
	if m.fail == "economy" {
		return domain.Economy{}, errBoom
	}
	return domain.DefaultEconomy, nil
}

func (m *memStore) OpenListings(context.Context) ([]domain.Listing, error) {
	if m.fail == "open" {
		return nil, errBoom
	}
	return m.all(""), nil
}

func (m *memStore) ListingsBySeller(_ context.Context, seller string) ([]domain.Listing, error) {
	if m.fail == "mine" {
		return nil, errBoom
	}
	return m.all(seller), nil
}

func (m *memStore) Atomically(_ context.Context, fn func(market.Tx) error) error {
	tx := &memTx{chars: maps.Clone(m.chars), listings: maps.Clone(m.listings), ledger: append([]domain.LedgerEntry(nil), m.ledger...), fail: m.fail}
	if err := fn(tx); err != nil {
		return err
	}
	m.chars, m.listings, m.ledger = tx.chars, tx.listings, tx.ledger
	return nil
}

type memTx struct {
	chars, listings map[string][]byte
	ledger          []domain.LedgerEntry
	fail            string
	records         int
}

func (t *memTx) Character(_ context.Context, id string) (domain.Character, error) {
	b, ok := t.chars[id]
	if !ok {
		return domain.Character{}, domain.ErrNotFound
	}
	var ch domain.Character
	_ = json.Unmarshal(b, &ch)
	return ch, nil
}

func (t *memTx) SaveCharacter(_ context.Context, ch domain.Character) error {
	if t.fail == "savechar" {
		return errBoom
	}
	b, _ := json.Marshal(ch)
	t.chars[ch.ID] = b
	return nil
}

func (t *memTx) Record(_ context.Context, e domain.LedgerEntry) error {
	t.records++
	if t.fail == "record" || (t.fail == "record2" && t.records == 2) {
		return errBoom
	}
	t.ledger = append(t.ledger, e)
	return nil
}

func (t *memTx) Listing(_ context.Context, id string) (domain.Listing, error) {
	b, ok := t.listings[id]
	if !ok {
		return domain.Listing{}, domain.ErrNotFound
	}
	var l domain.Listing
	_ = json.Unmarshal(b, &l)
	return l, nil
}

func (t *memTx) SaveListing(_ context.Context, l domain.Listing) error {
	if t.fail == "savelisting" {
		return errBoom
	}
	b, _ := json.Marshal(l)
	t.listings[l.ID] = b
	return nil
}

type catalog struct{}

var (
	rope   = domain.BaseItem{Key: "rope", Name: "Rope", Kind: domain.ItemGear, Value: 1}
	arrows = domain.BaseItem{Key: "arrows", Name: "Arrows", Kind: domain.ItemGear, Value: 1, Stack: true}
)

func (catalog) Items() []domain.BaseItem { return []domain.BaseItem{rope, arrows} }
func (catalog) Item(key string) (domain.BaseItem, error) {
	for _, b := range []domain.BaseItem{rope, arrows} {
		if b.Key == key {
			return b, nil
		}
	}
	return domain.BaseItem{}, domain.ErrUnknown
}

type clock struct{}

func (clock) Now() time.Time { return time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC) }

type ids struct{ n int }

func (i *ids) New() string { i.n++; return fmt.Sprintf("id%d", i.n) }

func hero(id, owner string, gold int, items ...domain.Item) domain.Character {
	return domain.Character{
		ID: id, OwnerID: owner, Status: domain.CharIdle,
		Sheet: &domain.Creature{ID: id, Name: strings.ToUpper(id[:1]) + id[1:], Gold: gold, Items: items},
	}
}

var sword = domain.Item{ID: "sword", Base: "longsword", Name: "Keen Longsword", Kind: domain.ItemWeapon, Rarity: domain.Uncommon, Value: 75}

func setup() (*market.UseCase, *memStore) {
	st := newStore()
	st.put(hero("aria", "me", 10, sword, domain.Keepsake("key", "a bone key"), domain.Item{ID: "worn", Name: "Mail", Kind: domain.ItemArmour, Slot: domain.SlotArmour, Value: 75}))
	st.put(hero("pip", "friend", 500))
	st.put(hero("bram", "me", 5))
	return market.New(st, catalog{}, clock{}, &ids{}), st
}

func TestListBuyAndCollect(t *testing.T) {
	t.Parallel()
	uc, st := setup()
	l, err := uc.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 200})
	if err != nil {
		t.Fatal(err)
	}
	if aria := st.char(t, "aria"); aria.Sheet.Item("sword") != nil || l.Item.ID != "sword" || l.Status != domain.ListingOpen {
		t.Fatal("the item moves into the listing (escrow)")
	}
	v, _ := uc.Browse(ctx, "friend")
	if len(v.Open) != 1 || len(v.Mine) != 0 {
		t.Fatalf("pip sees it for sale: %+v", v)
	}
	if err := uc.Buy(ctx, "me", l.ID, "bram"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("you can't buy your own listing: %v", err)
	}
	if err := uc.Buy(ctx, "friend", l.ID, "pip"); err != nil {
		t.Fatal(err)
	}
	pip := st.char(t, "pip")
	got := pip.Sheet.Item("sword")
	if pip.Sheet.Gold != 300 || got == nil || !strings.HasPrefix(got.History[len(got.History)-1], "Bought at the market by Pip for 200 gp") {
		t.Fatalf("pip pays and gets the sword with its history: %d %+v", pip.Sheet.Gold, got)
	}
	if err := uc.Buy(ctx, "friend", l.ID, "pip"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("it can only sell once: %v", err)
	}
	v, _ = uc.Browse(ctx, "me")
	if len(v.Open) != 0 || len(v.Mine) != 1 || v.Mine[0].Proceeds != 180 {
		t.Fatalf("the seller sees proceeds after the 10%% fee: %+v", v.Mine)
	}
	if err := uc.Collect(ctx, "friend", l.ID, "pip"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("only the seller collects: %v", err)
	}
	if err := uc.Collect(ctx, "me", l.ID, "bram"); err != nil {
		t.Fatal(err)
	}
	if bram := st.char(t, "bram"); bram.Sheet.Gold != 185 {
		t.Fatalf("collected into any hero in town: %d", bram.Sheet.Gold)
	}
	if err := uc.Collect(ctx, "me", l.ID, "bram"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("collect once: %v", err)
	}
	if v, _ := uc.Browse(ctx, "me"); len(v.Mine) != 0 {
		t.Fatal("collected sales leave your list")
	}
}

func TestWithdrawAndRules(t *testing.T) {
	t.Parallel()
	uc, st := setup()
	for _, c := range []struct {
		in   market.ListInput
		want error
	}{
		{market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 0}, domain.ErrInvalid},
		{market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "key", Price: 5}, domain.ErrInvalid},
		{market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "worn", Price: 5}, domain.ErrInvalid},
		{market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "ghost", Price: 5}, domain.ErrNotFound},
		{market.ListInput{AccountID: "me", CharacterID: "pip", ItemID: "sword", Price: 5}, domain.ErrNotFound},
		{market.ListInput{AccountID: "me", CharacterID: "nobody", ItemID: "sword", Price: 5}, domain.ErrNotFound},
	} {
		if _, err := uc.List(ctx, c.in); !errors.Is(err, c.want) {
			t.Errorf("%+v: %v", c.in, err)
		}
	}
	l, _ := uc.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 9999})
	if err := uc.Buy(ctx, "friend", l.ID, "pip"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("pip can't afford it: %v", err)
	}
	if err := uc.Withdraw(ctx, "friend", l.ID, "pip"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("only the seller withdraws: %v", err)
	}
	away := st.char(t, "bram")
	away.Status = domain.CharAdventuring
	st.put(away)
	if err := uc.Withdraw(ctx, "me", l.ID, "bram"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("heroes away on an adventure can't trade: %v", err)
	}
	if err := uc.Withdraw(ctx, "me", l.ID, "aria"); err != nil {
		t.Fatal(err)
	}
	if aria := st.char(t, "aria"); aria.Sheet.Item("sword") == nil {
		t.Fatal("the sword comes back")
	}
	if err := uc.Withdraw(ctx, "me", l.ID, "aria"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("withdraw once: %v", err)
	}
	if err := uc.Buy(ctx, "friend", "nope", "pip"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown listing: %v", err)
	}
	if err := uc.Collect(ctx, "me", "nope", "aria"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown listing: %v", err)
	}
	full := st.char(t, "pip")
	full.Sheet.Items = make([]domain.Item, 40)
	st.put(full)
	l2, _ := uc.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 5})
	if err := uc.Buy(ctx, "friend", l2.ID, "pip"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("no room in the pack: %v", err)
	}
	if err := uc.Collect(ctx, "me", l2.ID, "aria"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("nothing to collect on an unsold listing: %v", err)
	}
	if err := uc.Collect(ctx, "me", l2.ID, "bram"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("check the listing before the hero: %v", err)
	}
}

func TestListingLimitsAndFailures(t *testing.T) {
	t.Parallel()
	uc, st := setup()
	aria := st.char(t, "aria")
	for i := range domain.MaxOpenListings {
		aria.Sheet.Items = append(aria.Sheet.Items, domain.Item{ID: fmt.Sprintf("r%d", i), Name: "Rope", Kind: domain.ItemGear, Value: 1})
	}
	st.put(aria)
	for i := range domain.MaxOpenListings {
		if _, err := uc.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: fmt.Sprintf("r%d", i), Price: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := uc.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 1}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("at most %d open listings: %v", domain.MaxOpenListings, err)
	}
	for op, call := range map[string]func() error{
		"open": func() error { _, err := uc.Browse(ctx, "me"); return err },
		"mine": func() error { _, err := uc.Browse(ctx, "me"); return err },
	} {
		st.fail = op
		if err := call(); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", op, err)
		}
	}
	st.fail = "mine"
	if _, err := uc.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 1}); !errors.Is(err, errBoom) {
		t.Fatalf("listing count failure: %v", err)
	}

	uc2, st2 := setup()
	st2.fail = "savelisting"
	if _, err := uc2.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 5}); !errors.Is(err, errBoom) {
		t.Fatalf("save failure: %v", err)
	}
	if st2.char(t, "aria").Sheet.Item("sword") == nil {
		t.Fatal("a failed listing changes nothing")
	}
	st2.fail = "savechar"
	if _, err := uc2.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 5}); !errors.Is(err, errBoom) {
		t.Fatalf("save failure: %v", err)
	}
}

func TestTownShop(t *testing.T) {
	t.Parallel()
	uc, st := setup()
	rich := st.char(t, "aria")
	rich.Sheet.Gold = 50
	st.put(rich)
	if shop := uc.Shop(); len(shop) != 2 || shop[0].Price != 1 {
		t.Fatalf("the shop sells the catalogue at list price: %+v", shop)
	}
	if err := uc.ShopBuy(ctx, "me", "aria", "arrows", 99); err != nil {
		t.Fatal(err)
	}
	if err := uc.ShopBuy(ctx, "me", "aria", "rope", 7); err != nil {
		t.Fatal(err)
	}
	aria := st.char(t, "aria")
	n := len(aria.Sheet.Items)
	bought := aria.Sheet.Items[n-2]
	if bought.Qty != 20 || aria.Sheet.Items[n-1].Qty != 0 || aria.Sheet.Gold != 50-20-1 || !strings.HasPrefix(bought.History[0], "Bought from the town shop by Aria for 20 gp") {
		t.Fatalf("quantities cap at 20 and only stackables stack: %+v gold %d", aria.Sheet.Items[n-2:], aria.Sheet.Gold)
	}
	if err := uc.ShopBuy(ctx, "me", "bram", "arrows", 10); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bram has 5 gold: %v", err)
	}
	if err := uc.ShopBuy(ctx, "me", "aria", "vorpal", 1); !errors.Is(err, domain.ErrUnknown) {
		t.Fatalf("unknown item: %v", err)
	}
	if err := uc.ShopBuy(ctx, "me", "nobody", "rope", 0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown hero: %v", err)
	}
	paid, err := uc.ShopSell(ctx, "me", "aria", "sword")
	if err != nil || paid != 37 {
		t.Fatalf("the shop pays half value: %d %v", paid, err)
	}
	if aria := st.char(t, "aria"); aria.Sheet.Item("sword") != nil || aria.Sheet.Gold != 29+37 {
		t.Fatalf("sold: gold %d", aria.Sheet.Gold)
	}
	if _, err := uc.ShopSell(ctx, "me", "aria", "key"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("keepsakes can't be sold: %v", err)
	}
	if _, err := uc.ShopSell(ctx, "me", "nobody", "key"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown hero: %v", err)
	}
}

func TestTradeEdgeCases(t *testing.T) {
	t.Parallel()
	uc, st := setup()
	l, _ := uc.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 10})
	if err := uc.Buy(ctx, "friend", l.ID, "nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("buying with an unknown hero: %v", err)
	}
	full := st.char(t, "aria")
	full.Sheet.Items = make([]domain.Item, 40)
	st.put(full)
	if err := uc.Withdraw(ctx, "me", l.ID, "aria"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("withdrawing into a full pack: %v", err)
	}
	if err := uc.Buy(ctx, "friend", l.ID, "pip"); err != nil {
		t.Fatal(err)
	}
	away := st.char(t, "bram")
	away.Status = domain.CharAdventuring
	st.put(away)
	if err := uc.Collect(ctx, "me", l.ID, "bram"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("collecting into a hero who's away: %v", err)
	}
}

func TestTheLedgerRecordsEveryCoin(t *testing.T) {
	t.Parallel()
	uc, st := setup()
	rich := st.char(t, "aria")
	rich.Sheet.Gold = 100
	st.put(rich)
	l, _ := uc.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 50})
	_ = uc.Buy(ctx, "friend", l.ID, "pip")
	_ = uc.Collect(ctx, "me", l.ID, "aria")
	_ = uc.ShopBuy(ctx, "me", "aria", "rope", 1)
	_, _ = uc.ShopSell(ctx, "friend", "pip", "sword")
	got := map[domain.GoldReason]int{}
	for _, e := range st.ledger {
		got[e.Reason] += e.Amount
	}
	want := map[domain.GoldReason]int{domain.GoldMarketFee: 5, domain.GoldMarketSale: 45, domain.GoldCollected: 45, domain.GoldShopSale: 1, domain.GoldShopBuysBack: 37}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %d, want %d (%v)", k, got[k], v, got)
		}
	}
}

func TestLedgerFailuresRollBack(t *testing.T) {
	t.Parallel()
	for _, fail := range []string{"economy", "record", "record2"} {
		uc, st := setup()
		l, _ := uc.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 20})
		st.fail = fail
		if err := uc.Buy(ctx, "friend", l.ID, "pip"); !errors.Is(err, errBoom) {
			t.Errorf("%s: buy %v", fail, err)
		}
		if st.char(t, "pip").Sheet.Gold != 500 || len(st.ledger) != 0 {
			t.Errorf("%s: a failed purchase changes nothing", fail)
		}
	}
	uc, st := setup()
	l, _ := uc.List(ctx, market.ListInput{AccountID: "me", CharacterID: "aria", ItemID: "sword", Price: 20})
	_ = uc.Buy(ctx, "friend", l.ID, "pip")
	for op, call := range map[string]func() error{
		"economy": func() error { _, err := uc.Browse(ctx, "me"); return err },
		"record":  func() error { return uc.Collect(ctx, "me", l.ID, "aria") },
	} {
		st.fail = op
		if err := call(); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", op, err)
		}
	}
	st.fail = "record"
	if err := uc.ShopBuy(ctx, "friend", "pip", "rope", 1); !errors.Is(err, errBoom) {
		t.Errorf("shop buy: %v", err)
	}
	if _, err := uc.ShopSell(ctx, "friend", "pip", "sword"); !errors.Is(err, errBoom) {
		t.Errorf("shop sell: %v", err)
	}
	st.fail = "economy"
	if _, err := uc.ShopSell(ctx, "friend", "pip", "sword"); !errors.Is(err, errBoom) {
		t.Errorf("shop sell economy: %v", err)
	}
	st.fail = ""
	l1, _ := uc.List(ctx, market.ListInput{AccountID: "friend", CharacterID: "pip", ItemID: "sword", Price: 1})
	before := len(st.ledger)
	if err := uc.Buy(ctx, "me", l1.ID, "aria"); err != nil {
		t.Fatal(err)
	}
	if n := len(st.ledger) - before; n != 1 {
		t.Fatalf("a 1 gp sale is all fee: only the fee is recorded (%d entries)", n)
	}
}
