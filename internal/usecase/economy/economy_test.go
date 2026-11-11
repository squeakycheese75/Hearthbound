package economy_test

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/usecase/economy"
	"errors"
	"fmt"
	"testing"
	"time"
)

var ctx = context.Background()

var errBoom = errors.New("boom")

var now = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

type store struct {
	econ  domain.Economy
	fail  string
	sales []domain.Listing
}

func (s *store) err(op string) error {
	if s.fail == op {
		return errBoom
	}
	return nil
}

func (s *store) Economy(context.Context) (domain.Economy, error) { return s.econ, s.err("economy") }

func (s *store) SetEconomy(_ context.Context, e domain.Economy) error {
	if err := s.err("set"); err != nil {
		return err
	}
	s.econ = e
	return nil
}

func (s *store) GoldSupply(context.Context) (int, int, error) { return 1234, 3, s.err("supply") }

func (s *store) GoldFlows(_ context.Context, since time.Time) (map[domain.GoldReason]int, error) {
	if s.fail == "week" && since.Before(now.Add(-48*time.Hour)) {
		return nil, errBoom
	}
	if since.After(now.Add(-25 * time.Hour)) { // the last day
		return map[domain.GoldReason]int{domain.GoldLoot: 100, domain.GoldMarketFee: 10, domain.GoldMarketSale: 90}, s.err("day")
	}
	return map[domain.GoldReason]int{domain.GoldLoot: 400, domain.GoldStarting: 50, domain.GoldShopSale: 120, domain.GoldMarketFee: 30}, nil
}

func (s *store) RecentSales(context.Context, time.Time) ([]domain.Listing, error) {
	return s.sales, s.err("sales")
}

type admins map[string]bool

func (a admins) IsAdmin(_ context.Context, id string) (bool, error) {
	if id == "broken" {
		return false, errBoom
	}
	return a[id], nil
}

type clock struct{}

func (clock) Now() time.Time { return now }

func setup() (*economy.UseCase, *store) {
	st := &store{econ: domain.DefaultEconomy}
	for i := range 60 {
		st.sales = append(st.sales, domain.Listing{Price: i + 1, SoldAt: now, Item: domain.Item{Name: fmt.Sprint("item", i), Rarity: domain.Rare}})
	}
	return economy.New(st, admins{"admin": true}, clock{}), st
}

func TestReport(t *testing.T) {
	t.Parallel()
	uc, _ := setup()
	r, err := uc.Report(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if r.Supply != 1234 || r.Heroes != 3 || r.Economy.MarketFeePercent != 10 {
		t.Fatalf("supply and settings: %+v", r)
	}
	if r.Day.In != 100 || r.Day.Out != 10 || r.Day.Net != 90 || r.Day.By[domain.GoldMarketSale] != 90 {
		t.Fatalf("a day's flows; transfers count in neither column: %+v", r.Day)
	}
	if r.Week.In != 450 || r.Week.Out != 150 || r.Week.Net != 300 {
		t.Fatalf("a week's flows: %+v", r.Week)
	}
	if len(r.Sales) != 50 || r.Sales[0].Item != "item0" || r.Sales[0].Rarity != domain.Rare {
		t.Fatalf("the 50 most recent sales: %d", len(r.Sales))
	}
}

func TestOnlyAdminsAndFailures(t *testing.T) {
	t.Parallel()
	uc, st := setup()
	if _, err := uc.Report(ctx, "player"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("players can't see the report: %v", err)
	}
	if err := uc.SetEconomy(ctx, "player", domain.DefaultEconomy); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("or change the economy: %v", err)
	}
	if _, err := uc.Report(ctx, "broken"); !errors.Is(err, errBoom) {
		t.Fatalf("admin check failure: %v", err)
	}
	for _, op := range []string{"economy", "supply", "day", "week", "sales"} {
		st.fail = op
		if _, err := uc.Report(ctx, "admin"); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", op, err)
		}
	}
	st.fail = ""
	for _, bad := range []domain.Economy{{LootPercent: -1, MarketFeePercent: 10, ShopBuyBackPercent: 50}, {LootPercent: 100, MarketFeePercent: 60, ShopBuyBackPercent: 50}, {LootPercent: 100, MarketFeePercent: 10, ShopBuyBackPercent: 101}} {
		if err := uc.SetEconomy(ctx, "admin", bad); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	tighter := domain.Economy{LootPercent: 50, MarketFeePercent: 15, ShopBuyBackPercent: 40}
	if err := uc.SetEconomy(ctx, "admin", tighter); err != nil || st.econ != tighter {
		t.Fatalf("the admin tightens the economy: %v", err)
	}
	st.fail = "set"
	if err := uc.SetEconomy(ctx, "admin", tighter); !errors.Is(err, errBoom) {
		t.Fatalf("save failure: %v", err)
	}
}
