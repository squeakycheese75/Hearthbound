package domain

import (
	"fmt"
	"time"
)

// ListingStatus is where a market listing stands.
type ListingStatus string

// Listing statuses.
const (
	ListingOpen      ListingStatus = "open"
	ListingSold      ListingStatus = "sold"
	ListingWithdrawn ListingStatus = "withdrawn"
)

// Market limits.
const (
	// MarketFeePercent of every sale leaves the realm (the default; see Economy).
	MarketFeePercent = 10
	// MaxPrice bounds a listing's price, in gold.
	MaxPrice = 100_000
	// MaxOpenListings bounds how many items one account can have for sale.
	MaxOpenListings = 20
	// ShopBuyBackPercent is what the town shop pays for gear it buys (the default).
	ShopBuyBackPercent = 50
)

// Listing is an item for sale at the market. The item is held here (in
// escrow) until it sells or is withdrawn; a sale's proceeds wait here until
// the seller collects them into one of their heroes in town.
type Listing struct {
	ID string `json:"id"`
	// SellerID is the selling account; SellerCharacter the hero who listed it.
	SellerID        string        `json:"sellerId"`
	SellerCharacter string        `json:"sellerCharacter"`
	Item            Item          `json:"item"`
	Price           int           `json:"price"`
	Status          ListingStatus `json:"status"`
	CreatedAt       time.Time     `json:"createdAt"`
	BuyerID         string        `json:"buyerId,omitempty"`
	SoldAt          time.Time     `json:"soldAt,omitzero"`
	// Proceeds is what the seller receives after the fee; Collected is set once
	// they take it.
	Proceeds  int  `json:"proceeds,omitempty"`
	Collected bool `json:"collected,omitempty"`
}

// CheckTradable says whether a character can trade an item: it must exist,
// not be equipped, and not be a keepsake.
func (c *Creature) CheckTradable(itemID string) (*Item, error) {
	it := c.Item(itemID)
	switch {
	case it == nil:
		return nil, fmt.Errorf("%w: %s isn't carrying that", ErrNotFound, c.Name)
	case it.Slot != "":
		return nil, fmt.Errorf("%w: take %s off first", ErrInvalid, it.Name)
	case it.Kind == ItemKeepsake:
		return nil, fmt.Errorf("%w: %s is a keepsake; it can't be sold", ErrInvalid, it.Name)
	}
	return it, nil
}

// TakeItem removes an item from the character and returns it.
func (c *Creature) TakeItem(itemID string) (Item, bool) {
	for i, it := range c.Items {
		if it.ID == itemID {
			c.Items = append(c.Items[:i], c.Items[i+1:]...)
			return it, true
		}
	}
	return Item{}, false
}
