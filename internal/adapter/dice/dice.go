// Package dice provides real random dice for the rules engine.
package dice

import (
	"crypto/rand"
	"math/big"
)

// Roller rolls fair dice using crypto/rand.
type Roller struct{}

// New returns a Roller.
func New() *Roller { return &Roller{} }

// Roll returns a uniform value in 1..sides.
func (*Roller) Roll(sides int) int {
	if sides < 2 {
		return 1
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return int(n.Int64()) + 1
}
