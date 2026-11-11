package sqlite_test

import (
	"context"
	"dnd/internal/adapter/sqlite"
	"dnd/internal/domain"
	"errors"
	"testing"
	"time"
)

func TestListingsAndTransactions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t)
	_ = s.CreateCharacter(ctx, hero("a", "Aria"))
	l := domain.Listing{
		ID: "l1", SellerID: "me", SellerCharacter: "a", Price: 20, Status: domain.ListingOpen, CreatedAt: time.UnixMilli(1000),
		Item: domain.Item{ID: "sword", Name: "Sword", Kind: domain.ItemWeapon},
	}
	err := s.InTx(ctx, func(tx *sqlite.Tx) error {
		ch, err := tx.Character(ctx, "a")
		if err != nil {
			return err
		}
		ch.Sheet.Gold = 99
		if err := tx.SaveCharacter(ctx, ch); err != nil {
			return err
		}
		return tx.SaveListing(ctx, l)
	})
	if err != nil {
		t.Fatal(err)
	}
	if open, _ := s.OpenListings(ctx); len(open) != 1 || open[0].Item.Name != "Sword" {
		t.Fatalf("open listings: %+v", open)
	}
	if got, _ := s.Character(ctx, "a"); got.Sheet.Gold != 99 {
		t.Fatal("the character change committed with the listing")
	}
	boom := errors.New("boom")
	err = s.InTx(ctx, func(tx *sqlite.Tx) error {
		sold, _ := tx.Listing(ctx, "l1")
		sold.Status = domain.ListingSold
		_ = tx.SaveListing(ctx, sold)
		ch, _ := tx.Character(ctx, "a")
		ch.Sheet.Gold = 0
		_ = tx.SaveCharacter(ctx, ch)
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("the error comes back: %v", err)
	}
	if open, _ := s.OpenListings(ctx); len(open) != 1 {
		t.Fatal("a failed transaction rolls back the listing")
	}
	if got, _ := s.Character(ctx, "a"); got.Sheet.Gold != 99 {
		t.Fatal("and the character")
	}
	_ = s.InTx(ctx, func(tx *sqlite.Tx) error {
		sold, _ := tx.Listing(ctx, "l1")
		sold.Status = domain.ListingSold
		return tx.SaveListing(ctx, sold)
	})
	if open, _ := s.OpenListings(ctx); len(open) != 0 {
		t.Fatal("sold listings leave the market")
	}
	if mine, _ := s.ListingsBySeller(ctx, "me"); len(mine) != 1 || mine[0].Status != domain.ListingSold {
		t.Fatalf("but stay on the seller's list: %+v", mine)
	}
	_ = s.InTx(ctx, func(tx *sqlite.Tx) error {
		if _, err := tx.Listing(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("unknown listing: %v", err)
		}
		return nil
	})
}

func TestGoldLedgerAndEconomy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := open(t)
	if e, err := s.Economy(ctx); err != nil || e != domain.DefaultEconomy {
		t.Fatalf("defaults until set: %+v %v", e, err)
	}
	tight := domain.Economy{LootPercent: 50, MarketFeePercent: 20, ShopBuyBackPercent: 30}
	if err := s.SetEconomy(ctx, tight); err != nil {
		t.Fatal(err)
	}
	if e, _ := s.Economy(ctx); e != tight {
		t.Fatalf("saved: %+v", e)
	}
	day := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	_ = s.RecordGold(ctx, domain.LedgerEntry{At: day, Reason: domain.GoldLoot, Amount: 40})
	_ = s.RecordGold(ctx, domain.LedgerEntry{At: day, Reason: domain.GoldLoot, Amount: 10})
	_ = s.RecordGold(ctx, domain.LedgerEntry{At: day.Add(-48 * time.Hour), Reason: domain.GoldMarketFee, Amount: 7})
	_ = s.InTx(ctx, func(tx *sqlite.Tx) error {
		return tx.Record(ctx, domain.LedgerEntry{At: day, Reason: domain.GoldShopSale, Amount: 3})
	})
	flows, err := s.GoldFlows(ctx, day.Add(-time.Hour))
	if err != nil || flows[domain.GoldLoot] != 50 || flows[domain.GoldShopSale] != 3 || flows[domain.GoldMarketFee] != 0 {
		t.Fatalf("flows since an hour ago: %v %v", flows, err)
	}
	g, party := adventure("g1", hero("a", "Aria"))
	_ = s.CreateCharacter(ctx, party[0])
	g.GoldMoves = []domain.LedgerEntry{{At: day, Reason: domain.GoldLoot, Amount: 5, GameID: "g1"}}
	_ = s.CreateAdventure(ctx, g, party)
	_ = s.Save(ctx, g)
	if flows, _ := s.GoldFlows(ctx, day.Add(-time.Hour)); flows[domain.GoldLoot] != 55 || g.GoldMoves != nil {
		t.Fatalf("a game save records its gold moves once: %v", flows)
	}
	_ = s.CreateCharacter(ctx, hero("b", "Bram"))
	sold := domain.Listing{ID: "l1", SellerID: "me", Status: domain.ListingSold, Price: 20, Proceeds: 18, SoldAt: day, CreatedAt: day}
	_ = s.InTx(ctx, func(tx *sqlite.Tx) error { return tx.SaveListing(ctx, sold) })
	gold, heroes, err := s.GoldSupply(ctx)
	if err != nil || heroes != 2 || gold != 0+0+18 {
		t.Fatalf("supply counts purses and uncollected proceeds: %d gold, %d heroes, %v", gold, heroes, err)
	}
	if sales, _ := s.RecentSales(ctx, day.Add(-time.Hour)); len(sales) != 1 {
		t.Fatalf("recent sales: %d", len(sales))
	}
	if sales, _ := s.RecentSales(ctx, day.Add(time.Hour)); len(sales) != 0 {
		t.Fatal("older sales are left out")
	}
}
