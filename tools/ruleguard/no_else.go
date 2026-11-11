//go:build ruleguard

package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// noElse enforces the house rule: no `else` and no `else if`.
// Use guard clauses, early returns, switch, or a helper function instead.
func noElse(m dsl.Matcher) {
	m.Match(
		`if $_ { $*_ } else { $*_ }`,
		`if $_; $_ { $*_ } else { $*_ }`,
		`if $_ { $*_ } else if $_ { $*_ }`,
		`if $_; $_ { $*_ } else if $_ { $*_ }`,
	).
		Report("no else: use an early return / guard clause, switch, or extract a function")
}
