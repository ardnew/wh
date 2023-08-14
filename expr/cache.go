package expr

import (
	"regexp"
	"sync"
)

// Cache defines a memoized data structure that associates regular expression
// patterns with their compiled regexp.Regexp representations.
// It contains synchronization primitives for safely accessing elements
// concurrently from multiple goroutines.
//
// From a (Expr).Match context, it enables reuse of regexp.Regexp objects across
// multiple calls without having to recompile the pattern string each time.
type Cache struct {
	*sync.RWMutex
	re map[string]*regexp.Regexp
}

// Get returns a compiled regexp.Regexp object for the given regular expression
// string pattern. The pattern will be compiled and added to the receiver Cache
// if it is not present. This method is safe to call from multiple goroutines
// concurrently.
func (c *Cache) Get(pattern string) (*regexp.Regexp, error) {
	c.RLock()
	r, ok := c.re[pattern]
	c.RUnlock()
	if !ok {
		var err error
		if r, err = regexp.Compile(pattern); err != nil {
			return nil, err
		}
		c.Lock()
		c.re[pattern] = r
		c.Unlock()
	}
	return r, nil
}
