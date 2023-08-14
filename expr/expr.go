// Package expr defines the supported types of match expressions.
package expr

import (
	"path"
	"regexp"
	"strconv"
	"sync"
)

// Error types specific to package expr that may be returned by one of its
// exported functions or methods. Use type assertion to determine the type of
// error and the interface func Error() for a descriptive error message.
type (
	ErrInvalidExpr Expr
)

// Error returns a descriptive error string for the receiver ErrInvalidExpr e.
func (e ErrInvalidExpr) Error() string {
	return "invalid Expr: int(" + strconv.Itoa(int(e)) + ")"
}

// Expr enumerates all supported types of match expressions.
type Expr int

// Enumerated constants of type Expr.
const (
	Fixed  Expr = iota // Match entire file names verbatim
	Glob               // Match using standard Go path.Match semantics
	Regexp             // Match using standard Go regexp.Regexp semantics
	numExpr
)

// String returns a string representation of the receiver Expr e.
func (e Expr) String() string {
	if u := uint(e); u < uint(numExpr) {
		return [numExpr]string{"fixed", "glob", "regexp"}[u]
	}
	return ErrInvalidExpr(e).Error()
}

// matchCache is a package-global Cache for use with (Expr).Match.
var matchCache = Cache{&sync.RWMutex{}, map[string]*regexp.Regexp{}}

// Match reports whether the given string s matches the given string pattern
// according to the semantics of the receiver Expr e.
// Match is safe to call from multiple goroutines concurrently.
func (e Expr) Match(pattern string, s string) (matched bool, err error) {
	switch e {
	case Fixed:
		matched, err = pattern == s, nil
	case Glob:
		matched, err = path.Match(pattern, s)
	case Regexp:
		var r *regexp.Regexp
		if r, err = matchCache.Get(pattern); err == nil {
			matched = r.MatchString(s)
		}
	default:
		matched, err = false, ErrInvalidExpr(e)
	}
	return
}
