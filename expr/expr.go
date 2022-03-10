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
		if r, err = matchCache.get(pattern); err == nil {
			matched = r.MatchString(s)
		}
	default:
		matched, err = false, ErrInvalidExpr(e)
	}
	return
}

// cache defines a memoized data structure that associates regular expression
// patterns with their compiled regexp.Regexp representations.
// It contains synchronization primitives for safely accessing elements
// concurrently from multiple goroutines.
//
// From a (Expr).Match context, it enables reuse of regexp.Regexp objects across
// multiple calls without having to recompile the pattern string each time.
type cache struct {
	pattern map[string]*regexp.Regexp
	lock    sync.RWMutex
}

// matchCache is a package-global cache for use with (Expr).Match.
// See godoc comments on type cache for details.
var matchCache = cache{pattern: map[string]*regexp.Regexp{}}

// get returns a compiled regexp.Regexp object for the given regular expression
// string pattern. The pattern will be compiled and added to the receiver cache
// if it is not present. This method is safe to call from multiple goroutines
// concurrently.
func (c *cache) get(pattern string) (*regexp.Regexp, error) {
	c.lock.RLock()
	r, ok := c.pattern[pattern]
	c.lock.RUnlock()
	if !ok {
		var err error
		if r, err = regexp.Compile(pattern); err != nil {
			return nil, err
		}
		c.lock.Lock()
		c.pattern[pattern] = r
		c.lock.Unlock()
	}
	return r, nil
}
