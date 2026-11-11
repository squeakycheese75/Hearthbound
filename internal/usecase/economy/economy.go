// Package economy lets the realm's owner see and steer its money: how much
// gold exists, where it comes from and goes, what things sell for, and the
// settings that control gold in (loot) and out (fees, the shop).
package economy

import (
	"context"
	"dnd/internal/domain"
	"fmt"
	"slices"
	"time"
)

// Store reads the gold ledger and economy settings.
type Store interface {
	Economy(ctx context.Context) (domain.Economy, error)
	SetEconomy(ctx context.Context, e domain.Economy) error
	GoldSupply(ctx context.Context) (gold, heroes int, err error)
	GoldFlows(ctx context.Context, since time.Time) (map[domain.GoldReason]int, error)
	RecentSales(ctx context.Context, since time.Time) ([]domain.Listing, error)
}

// Admins says whether an account runs the realm.
type Admins interface {
	IsAdmin(ctx context.Context, accountID string) (bool, error)
}

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// UseCase reports on and steers the economy.
type UseCase struct {
	store  Store
	admins Admins
	clock  Clock
}

// New wires the use case.
func New(store Store, admins Admins, clock Clock) *UseCase {
	return &UseCase{store: store, admins: admins, clock: clock}
}

// Flows is gold movement over a period.
type Flows struct {
	In  int                       `json:"in"`
	Out int                       `json:"out"`
	Net int                       `json:"net"`
	By  map[domain.GoldReason]int `json:"by"`
}

// Sale is one recent market sale.
type Sale struct {
	Item   string        `json:"item"`
	Rarity domain.Rarity `json:"rarity"`
	Price  int           `json:"price"`
	At     time.Time     `json:"at"`
}

// Report is the realm's money at a glance.
type Report struct {
	Economy domain.Economy `json:"economy"`
	// Supply is all the gold in the realm; Heroes how many living heroes hold it.
	Supply int    `json:"supply"`
	Heroes int    `json:"heroes"`
	Day    Flows  `json:"day"`
	Week   Flows  `json:"week"`
	Sales  []Sale `json:"sales"`
}

// Report shows the realm's money to an admin.
func (u *UseCase) Report(ctx context.Context, accountID string) (Report, error) {
	if err := u.requireAdmin(ctx, accountID); err != nil {
		return Report{}, err
	}
	econ, err := u.store.Economy(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("load economy: %w", err)
	}
	gold, heroes, err := u.store.GoldSupply(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("gold supply: %w", err)
	}
	now := u.clock.Now()
	day, err := u.flows(ctx, now.Add(-24*time.Hour))
	if err != nil {
		return Report{}, err
	}
	week, err := u.flows(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		return Report{}, err
	}
	sales, err := u.sales(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		return Report{}, err
	}
	return Report{Economy: econ, Supply: gold, Heroes: heroes, Day: day, Week: week, Sales: sales}, nil
}

// SetEconomy changes the realm's economy settings.
func (u *UseCase) SetEconomy(ctx context.Context, accountID string, e domain.Economy) error {
	if err := u.requireAdmin(ctx, accountID); err != nil {
		return err
	}
	if err := e.Validate(); err != nil {
		return err
	}
	if err := u.store.SetEconomy(ctx, e); err != nil {
		return fmt.Errorf("save economy: %w", err)
	}
	return nil
}

func (u *UseCase) flows(ctx context.Context, since time.Time) (Flows, error) {
	by, err := u.store.GoldFlows(ctx, since)
	if err != nil {
		return Flows{}, fmt.Errorf("gold flows: %w", err)
	}
	f := Flows{By: by}
	for reason, n := range by {
		switch {
		case slices.Contains(domain.Faucets, reason):
			f.In += n
		case slices.Contains(domain.Sinks, reason):
			f.Out += n
		}
	}
	f.Net = f.In - f.Out
	return f, nil
}

// maxSales caps how many recent sales the report lists.
const maxSales = 50

func (u *UseCase) sales(ctx context.Context, since time.Time) ([]Sale, error) {
	ls, err := u.store.RecentSales(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("recent sales: %w", err)
	}
	out := make([]Sale, 0, min(len(ls), maxSales))
	for _, l := range ls[:min(len(ls), maxSales)] {
		out = append(out, Sale{Item: l.Item.Name, Rarity: l.Item.Rarity, Price: l.Price, At: l.SoldAt})
	}
	return out, nil
}

func (u *UseCase) requireAdmin(ctx context.Context, accountID string) error {
	ok, err := u.admins.IsAdmin(ctx, accountID)
	if err != nil {
		return fmt.Errorf("check admin: %w", err)
	}
	if !ok {
		return fmt.Errorf("%w: only the realm's admin manages the economy", domain.ErrForbidden)
	}
	return nil
}
