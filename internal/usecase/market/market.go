// Package market runs the realm's marketplace: fixed-price listings between
// players (the item held in escrow, a fee on each sale) and the town shop.
// Only heroes in town trade: an adventuring hero's sheet belongs to their game.
package market

import (
	"context"
	"dnd/internal/domain"
	"dnd/internal/domain/rules"
	"fmt"
	"time"
)

// Tx is the store inside one transaction: everything read and written
// through it commits together or not at all.
type Tx interface {
	Character(ctx context.Context, id string) (domain.Character, error)
	SaveCharacter(ctx context.Context, ch domain.Character) error
	Listing(ctx context.Context, id string) (domain.Listing, error)
	SaveListing(ctx context.Context, l domain.Listing) error
	// Record appends to the gold ledger.
	Record(ctx context.Context, e domain.LedgerEntry) error
}

// Store reads listings and runs trades atomically.
type Store interface {
	Economy(ctx context.Context) (domain.Economy, error)
	OpenListings(ctx context.Context) ([]domain.Listing, error)
	ListingsBySeller(ctx context.Context, sellerID string) ([]domain.Listing, error)
	Atomically(ctx context.Context, fn func(Tx) error) error
}

// Catalog supplies what the town shop sells.
type Catalog interface {
	Items() []domain.BaseItem
	Item(key string) (domain.BaseItem, error)
}

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// IDGen makes unique IDs.
type IDGen interface {
	New() string
}

// UseCase runs the market.
type UseCase struct {
	store   Store
	catalog Catalog
	clock   Clock
	ids     IDGen
}

// New wires the use case.
func New(store Store, catalog Catalog, clock Clock, ids IDGen) *UseCase {
	return &UseCase{store: store, catalog: catalog, clock: clock, ids: ids}
}

// View is the market as one account sees it.
type View struct {
	// FeePercent is the market's cut of each sale; BuyBackPercent what the shop pays.
	FeePercent     int `json:"feePercent"`
	BuyBackPercent int `json:"buyBackPercent"`
	// Open are the listings for sale (including the viewer's own).
	Open []domain.Listing `json:"open"`
	// Mine are the viewer's listings still open, or sold and not yet collected.
	Mine []domain.Listing `json:"mine"`
}

// Browse returns what's for sale and the account's own listings.
func (u *UseCase) Browse(ctx context.Context, accountID string) (View, error) {
	open, err := u.store.OpenListings(ctx)
	if err != nil {
		return View{}, fmt.Errorf("load market: %w", err)
	}
	all, err := u.store.ListingsBySeller(ctx, accountID)
	if err != nil {
		return View{}, fmt.Errorf("load your listings: %w", err)
	}
	econ, err := u.store.Economy(ctx)
	if err != nil {
		return View{}, fmt.Errorf("load economy: %w", err)
	}
	v := View{Open: open, Mine: []domain.Listing{}, FeePercent: econ.MarketFeePercent, BuyBackPercent: econ.ShopBuyBackPercent}
	for _, l := range all {
		if l.Status == domain.ListingOpen || (l.Status == domain.ListingSold && !l.Collected) {
			v.Mine = append(v.Mine, l)
		}
	}
	return v, nil
}

// ListInput puts one of a hero's items up for sale.
type ListInput struct {
	AccountID   string `json:"-"`
	CharacterID string `json:"characterId"`
	ItemID      string `json:"itemId"`
	Price       int    `json:"price"`
}

// List takes an item from a hero in town and puts it up for sale.
func (u *UseCase) List(ctx context.Context, in ListInput) (domain.Listing, error) {
	if in.Price < 1 || in.Price > domain.MaxPrice {
		return domain.Listing{}, fmt.Errorf("%w: price must be 1 to %d gold", domain.ErrInvalid, domain.MaxPrice)
	}
	mine, err := u.store.ListingsBySeller(ctx, in.AccountID)
	if err != nil {
		return domain.Listing{}, fmt.Errorf("load your listings: %w", err)
	}
	if openCount(mine) >= domain.MaxOpenListings {
		return domain.Listing{}, fmt.Errorf("%w: at most %d items for sale at once", domain.ErrInvalid, domain.MaxOpenListings)
	}
	var l domain.Listing
	err = u.store.Atomically(ctx, func(tx Tx) error {
		ch, err := hero(ctx, tx, in.AccountID, in.CharacterID)
		if err != nil {
			return err
		}
		if _, err := ch.Sheet.CheckTradable(in.ItemID); err != nil {
			return err
		}
		it, _ := ch.Sheet.TakeItem(in.ItemID)
		it.History = append(it.History, fmt.Sprintf("Listed at the market by %s for %d gp (%s)", ch.Sheet.Name, in.Price, u.day()))
		l = domain.Listing{
			ID: u.ids.New(), SellerID: in.AccountID, SellerCharacter: ch.ID, Item: it, Price: in.Price,
			Status: domain.ListingOpen, CreatedAt: u.clock.Now(),
		}
		return save(ctx, tx, l, ch)
	})
	return l, err
}

// Buy pays for a listing from one of the account's heroes in town, who gets
// the item. The fee leaves the realm; the rest waits for the seller.
func (u *UseCase) Buy(ctx context.Context, accountID, listingID, characterID string) error {
	econ, err := u.store.Economy(ctx)
	if err != nil {
		return fmt.Errorf("load economy: %w", err)
	}
	return u.store.Atomically(ctx, func(tx Tx) error {
		l, err := openListing(ctx, tx, listingID)
		if err != nil {
			return err
		}
		if l.SellerID == accountID {
			return fmt.Errorf("%w: that's your own listing; withdraw it instead", domain.ErrInvalid)
		}
		ch, err := hero(ctx, tx, accountID, characterID)
		if err != nil {
			return err
		}
		if err := canReceive(ch.Sheet, l.Price); err != nil {
			return err
		}
		ch.Sheet.Gold -= l.Price
		l.Item.History = append(l.Item.History, fmt.Sprintf("Bought at the market by %s for %d gp (%s)", ch.Sheet.Name, l.Price, u.day()))
		ch.Sheet.Items = append(ch.Sheet.Items, l.Item)
		l.Status, l.BuyerID, l.SoldAt = domain.ListingSold, accountID, u.clock.Now()
		fee := econ.Fee(l.Price)
		l.Proceeds = l.Price - fee
		if err := u.record(ctx, tx, domain.GoldMarketFee, fee, l.SellerID, "", l.Item.Name); err != nil {
			return err
		}
		if err := u.record(ctx, tx, domain.GoldMarketSale, l.Proceeds, accountID, ch.ID, l.Item.Name); err != nil {
			return err
		}
		return save(ctx, tx, l, ch)
	})
}

// Withdraw takes an unsold item back into one of the seller's heroes in town.
func (u *UseCase) Withdraw(ctx context.Context, accountID, listingID, characterID string) error {
	return u.store.Atomically(ctx, func(tx Tx) error {
		l, err := openListing(ctx, tx, listingID)
		if err != nil {
			return err
		}
		if l.SellerID != accountID {
			return fmt.Errorf("%w: listing %s", domain.ErrNotFound, listingID)
		}
		ch, err := hero(ctx, tx, accountID, characterID)
		if err != nil {
			return err
		}
		if err := canReceive(ch.Sheet, 0); err != nil {
			return err
		}
		l.Item.History = append(l.Item.History, fmt.Sprintf("Taken off the market (%s)", u.day()))
		ch.Sheet.Items = append(ch.Sheet.Items, l.Item)
		l.Status = domain.ListingWithdrawn
		return save(ctx, tx, l, ch)
	})
}

// Collect pays a sold listing's proceeds to one of the seller's heroes in town.
func (u *UseCase) Collect(ctx context.Context, accountID, listingID, characterID string) error {
	return u.store.Atomically(ctx, func(tx Tx) error {
		l, err := tx.Listing(ctx, listingID)
		if err != nil {
			return fmt.Errorf("load listing: %w", err)
		}
		if l.SellerID != accountID {
			return fmt.Errorf("%w: listing %s", domain.ErrNotFound, listingID)
		}
		if l.Status != domain.ListingSold || l.Collected {
			return fmt.Errorf("%w: nothing to collect", domain.ErrInvalid)
		}
		ch, err := hero(ctx, tx, accountID, characterID)
		if err != nil {
			return err
		}
		ch.Sheet.Gold += l.Proceeds
		l.Collected = true
		if err := u.record(ctx, tx, domain.GoldCollected, l.Proceeds, accountID, ch.ID, l.Item.Name); err != nil {
			return err
		}
		return save(ctx, tx, l, ch)
	})
}

// ShopItem is something the town shop sells, at its price.
type ShopItem struct {
	domain.BaseItem
	Price int `json:"price"`
}

// Shop lists what the town shop sells: every catalogue item at list price.
func (u *UseCase) Shop() []ShopItem {
	items := u.catalog.Items()
	out := make([]ShopItem, 0, len(items))
	for _, b := range items {
		out = append(out, ShopItem{BaseItem: b, Price: b.Value})
	}
	return out
}

// ShopBuy buys catalogue gear from the town shop for a hero in town. qty
// applies to items sold by number (arrows, rations, potions).
func (u *UseCase) ShopBuy(ctx context.Context, accountID, characterID, key string, qty int) error {
	b, err := u.catalog.Item(key)
	if err != nil {
		return err
	}
	if !b.Stack || qty < 1 {
		qty = 1
	}
	qty = min(qty, 20)
	price := b.Value * qty
	return u.store.Atomically(ctx, func(tx Tx) error {
		ch, err := hero(ctx, tx, accountID, characterID)
		if err != nil {
			return err
		}
		if err := canReceive(ch.Sheet, price); err != nil {
			return err
		}
		it := domain.NewItem(u.ids.New(), b, qty)
		it.History = []string{fmt.Sprintf("Bought from the town shop by %s for %d gp (%s)", ch.Sheet.Name, price, u.day())}
		ch.Sheet.Gold -= price
		ch.Sheet.Items = append(ch.Sheet.Items, it)
		if err := u.record(ctx, tx, domain.GoldShopSale, price, accountID, ch.ID, b.Name); err != nil {
			return err
		}
		return tx.SaveCharacter(ctx, ch)
	})
}

// ShopSell sells a hero's item to the town shop for half its value and
// returns what the shop paid.
func (u *UseCase) ShopSell(ctx context.Context, accountID, characterID, itemID string) (int, error) {
	econ, err := u.store.Economy(ctx)
	if err != nil {
		return 0, fmt.Errorf("load economy: %w", err)
	}
	paid := 0
	err = u.store.Atomically(ctx, func(tx Tx) error {
		ch, err := hero(ctx, tx, accountID, characterID)
		if err != nil {
			return err
		}
		it, err := ch.Sheet.CheckTradable(itemID)
		if err != nil {
			return err
		}
		paid = econ.BuyBack(*it)
		name := it.Name
		ch.Sheet.TakeItem(itemID)
		ch.Sheet.Gold += paid
		if err := u.record(ctx, tx, domain.GoldShopBuysBack, paid, accountID, ch.ID, name); err != nil {
			return err
		}
		return tx.SaveCharacter(ctx, ch)
	})
	return paid, err
}

// record appends a gold movement to the ledger (nothing for zero amounts).
func (u *UseCase) record(ctx context.Context, tx Tx, why domain.GoldReason, amount int, accountID, characterID, note string) error {
	if amount <= 0 {
		return nil
	}
	e := domain.LedgerEntry{At: u.clock.Now(), Reason: why, Amount: amount, AccountID: accountID, CharacterID: characterID, Note: note}
	if err := tx.Record(ctx, e); err != nil {
		return fmt.Errorf("record gold: %w", err)
	}
	return nil
}

// hero loads one of the account's characters who is in town and able to trade.
func hero(ctx context.Context, tx Tx, accountID, characterID string) (domain.Character, error) {
	ch, err := tx.Character(ctx, characterID)
	if err != nil {
		return domain.Character{}, fmt.Errorf("load character: %w", err)
	}
	if ch.OwnerID != accountID || ch.Sheet == nil {
		return domain.Character{}, fmt.Errorf("%w: character %s", domain.ErrNotFound, characterID)
	}
	if ch.Status != domain.CharIdle {
		return domain.Character{}, fmt.Errorf("%w: %s isn't in town (%s); trade when they're back", domain.ErrInvalid, ch.Sheet.Name, ch.Status)
	}
	return ch, nil
}

func openListing(ctx context.Context, tx Tx, id string) (domain.Listing, error) {
	l, err := tx.Listing(ctx, id)
	if err != nil {
		return domain.Listing{}, fmt.Errorf("load listing: %w", err)
	}
	if l.Status != domain.ListingOpen {
		return domain.Listing{}, fmt.Errorf("%w: that item has already gone", domain.ErrInvalid)
	}
	return l, nil
}

// canReceive checks a hero can pay a price and carry one more item.
func canReceive(c *domain.Creature, price int) error {
	if c.Gold < price {
		return fmt.Errorf("%w: %s has %d gold; this costs %d", domain.ErrInvalid, c.Name, c.Gold, price)
	}
	if len(c.Items) >= rules.MaxInventory {
		return fmt.Errorf("%w: %s can't carry any more", domain.ErrInvalid, c.Name)
	}
	return nil
}

func save(ctx context.Context, tx Tx, l domain.Listing, ch domain.Character) error {
	if err := tx.SaveListing(ctx, l); err != nil {
		return fmt.Errorf("save listing: %w", err)
	}
	if err := tx.SaveCharacter(ctx, ch); err != nil {
		return fmt.Errorf("save character: %w", err)
	}
	return nil
}

func openCount(ls []domain.Listing) int {
	n := 0
	for _, l := range ls {
		if l.Status == domain.ListingOpen {
			n++
		}
	}
	return n
}

func (u *UseCase) day() string {
	return u.clock.Now().Format("2 Jan 2006")
}
