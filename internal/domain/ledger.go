package domain

import (
	"fmt"
	"time"
)

// GoldReason says why gold moved. Faucets bring gold into the realm, sinks
// take it out, transfers move it between players without changing the total.
type GoldReason string

// Gold movements.
const (
	GoldLoot         GoldReason = "loot"          // faucet: the DM hands out gold
	GoldStarting     GoldReason = "starting"      // faucet: a new hero's purse
	GoldShopBuysBack GoldReason = "shop_buyback"  // faucet: the shop buys an item
	GoldDMCharge     GoldReason = "dm_charge"     // sink: an in-story purchase
	GoldShopSale     GoldReason = "shop_sale"     // sink: the shop sells an item
	GoldMarketFee    GoldReason = "market_fee"    // sink: the cut of a player sale
	GoldMarketSale   GoldReason = "market_sale"   // transfer: buyer to seller
	GoldCollected    GoldReason = "market_payout" // transfer: proceeds to a hero
)

// Faucets bring gold into the realm; sinks take it out.
var (
	Faucets = []GoldReason{GoldLoot, GoldStarting, GoldShopBuysBack}
	Sinks   = []GoldReason{GoldDMCharge, GoldShopSale, GoldMarketFee}
)

// LedgerEntry records one movement of gold. Amount is always positive; the
// reason says which way it went. Entries are append-only.
type LedgerEntry struct {
	ID          string     `json:"id"`
	At          time.Time  `json:"at"`
	Reason      GoldReason `json:"reason"`
	Amount      int        `json:"amount"`
	AccountID   string     `json:"accountId,omitempty"`
	CharacterID string     `json:"characterId,omitempty"`
	GameID      string     `json:"gameId,omitempty"`
	// Note is a short description: the item, or what the DM said it was for.
	Note string `json:"note,omitempty"`
}

// Economy is the realm owner's control over money in and out.
type Economy struct {
	// LootPercent scales how much gold the DM may hand out per turn (100 = the
	// standard rules.TurnGoldCap).
	LootPercent int `json:"lootPercent"`
	// MarketFeePercent of each sale between players leaves the realm.
	MarketFeePercent int `json:"marketFeePercent"`
	// ShopBuyBackPercent of an item's value is what the town shop pays.
	ShopBuyBackPercent int `json:"shopBuyBackPercent"`
}

// DefaultEconomy is a new realm's economy.
var DefaultEconomy = Economy{LootPercent: 100, MarketFeePercent: MarketFeePercent, ShopBuyBackPercent: ShopBuyBackPercent}

// Validate checks an economy's settings are in range.
func (e Economy) Validate() error {
	switch {
	case e.LootPercent < 0 || e.LootPercent > 500:
		return fmt.Errorf("%w: loot rate must be 0-500%%", ErrInvalid)
	case e.MarketFeePercent < 0 || e.MarketFeePercent > 50:
		return fmt.Errorf("%w: market fee must be 0-50%%", ErrInvalid)
	case e.ShopBuyBackPercent < 0 || e.ShopBuyBackPercent > 100:
		return fmt.Errorf("%w: shop buy-back must be 0-100%%", ErrInvalid)
	}
	return nil
}

// Fee is the market's cut of a sale at a price, rounded up.
func (e Economy) Fee(price int) int {
	return (price*e.MarketFeePercent + 99) / 100
}

// BuyBack is what the town shop pays for an item.
func (e Economy) BuyBack(it Item) int {
	return it.Value * max(it.Qty, 1) * e.ShopBuyBackPercent / 100
}
