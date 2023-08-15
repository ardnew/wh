package wh

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/ardnew/wh/expr"
)

// Option defines all search and match options for the exported Match functions.
type Option struct {
	FollowSymlinks bool      // Follow symlinks when recursing into subdirectories
	MaxFollow      int       // Maximum number symlink components to follow
	MaxDepth       int       // Maximum number of subdirectory recursions
	Expr           expr.Expr // Matching semantics of the given pattern
	IgnoreCase     bool      // Ignore case in matching semantics
	WorkingDir     string    // Current working directory
	fromDepth      int       // Depth prior to dereferencing a symlink
	fromFollow     int       // Number of Links resolved
}

// MatchFunc is the signature of each of the exported matching functions.
type MatchFunc func(Option, string, ...string) ([]string, error)

// MatchFixed returns the result of calling Match with the given string pattern
// used to match file names verbatim.
func MatchFixed(option Option, pattern string, sub ...string) ([]string, error) {
	option.Expr = expr.Fixed
	if option.IgnoreCase {
		pattern = strings.ToLower(pattern)
	}
	return Match(option, pattern, sub...)
}

// MatchGlob returns the result of calling Match with the given string pattern
// used to match file names according to path.Match semantics.
func MatchGlob(option Option, pattern string, sub ...string) ([]string, error) {
	option.Expr = expr.Glob
	if option.IgnoreCase {
		pattern = strings.ToLower(pattern)
	}
	return Match(option, pattern, sub...)
}

// MatchRegexp returns the result of calling Match with the given string pattern
// used to match file names according to regexp.Regexp semantics.
func MatchRegexp(option Option, pattern string, sub ...string) ([]string, error) {
	option.Expr = expr.Regexp
	if option.IgnoreCase {
		pattern = "(?i)" + pattern
	}
	return Match(option, pattern, sub...)
}

// ErrMaxDepth represents a condition when walking a file system where the
// number of descendent directories traversed is greater than maximum allowed.
type ErrMaxDepth int

// Error returns a descriptive error string for the receiver ErrMaxDepth e.
func (e ErrMaxDepth) Error() string {
	return "maximum depth (" + strconv.Itoa(int(e)) + ") exceeded"
}

// ErrWalkDir represents a list of errors encountered when calling fs.WalkDir
// on their corresponding subdirectories.
type ErrWalkDir []errWalkDir
type errWalkDir struct {
	dir string
	err error
}

// Error returns a descriptive error string for the receiver ErrWalkDir e.
func (e ErrWalkDir) Error() string {
	t := make([]string, len(e))
	for i, s := range e {
		t[i] = fmt.Sprintf("%q: %q", s.dir, s.err)
	}
	return "{" + strings.Join(t, ", ") + "}"
}

// ErrInvalidPath represents an error for a path with invalid symbols.
type ErrInvalidPath string

// Error returns a descriptive error string for the receiver ErrInvalidPath e.
func (e ErrInvalidPath) Error() string {
	return "invalid path: " + string(e)
}

// ValidPath reports whether the given string s contains invalid symbols for a
// file path.
func ValidPath(s string) error {
	// Silently ignore certain runes that are not valid in a path.
	ignore := string(os.PathSeparator) + "."
	strip := func(r rune) rune {
		if strings.ContainsRune(ignore, r) {
			return rune(-1)
		}
		return r
	}
	// Note this has different semantics than strings.ContainsAny(s, ignore).
	if !fs.ValidPath(strings.Map(strip, s)) {
		return ErrInvalidPath(s)
	}
	return nil
}

type (
	// Chain holds a sequence of Link for a single path component.
	Chain []*Link
	// Link holds a single symlink dereference in a Chain.
	Link struct {
		root string
		name string
		ent  fs.DirEntry
	}
)

// MakeChain creates a new Chain, initialized with the given list of Links.
func MakeChain(link ...*Link) Chain {
	return append(Chain{}, link...)
}

// Add adds the given list of Links to a Chain.
func (c *Chain) Add(link ...*Link) {
	*c = append(*c, link...)
}

// Head returns the first symlink in a Chain.
func (c *Chain) Head() *Link {
	if len(*c) > 0 {
		return (*c)[0]
	}
	return nil
}

// String returns a graphical representation of a Chain.
func (c *Chain) String() string {
	if len(*c) == 0 {
		return ""
	} else if len(*c) == 1 {
		return (*c)[0].Path()
	} else {
		var sb strings.Builder
		for i := 0; i < len(*c); i++ {
			branch := "└┬╼╸"
			if i == 0 {
				branch = "─┬╼╸"
			} else if i == len(*c)-1 {
				branch = "└─╼╸"
			}
			fmt.Fprintf(&sb, "%*s%s %s\n", i, "", branch, (*c)[i].Path())
		}
		return sb.String()
	}
}

// NewLink returns a reference to a new Link, initialized with the given file
// system attributes.
func NewLink(root string, name string, ent fs.DirEntry) *Link {
	return &Link{root: root, name: name, ent: ent}
}

// Path returns the result of joining the Link's file name to its parent
// directory.
func (l *Link) Path() string { return path.Join(l.root, l.name) }

// IsSymlink returns true if and only if the Link has symlink mode bits set.
func (l *Link) IsSymlink() bool { return l.ent.Type()&fs.ModeSymlink != 0 }

// Deref creates and returns a new Link initialized with the destination's
// file system attributes of the receive symlink.
func (l *Link) Deref() (d Link, err error) {
	var dest string
	dest, err = os.Readlink(l.Path())
	if err != nil {
		return // Just ignore the symlink if there is any error.
	}
	if !path.IsAbs(dest) {
		dest = path.Join(l.root, dest)
	}
	var info fs.FileInfo
	info, err = os.Lstat(dest)
	if err != nil {
		return // Just ignore the symlink if there is any error.
	}
	d.root = path.Dir(dest)
	d.name = path.Base(dest)
	d.ent = fs.FileInfoToDirEntry(info)
	return
}

func Match(option Option, pattern string, sub ...string) (found []string, err error) {

	serr := make(ErrWalkDir, 0, len(sub))

	for _, p := range sub {

		// A canonical path is required for accurately computing traversal depth.
		root := path.Clean(p)

		werr := fs.WalkDir(os.DirFS(root), ".",
			func(c string, d fs.DirEntry, err error) error {

				// Check if we have an error on directory entry
				if err != nil {
					if d == nil {
						// The root path os.DirFS(p) was invalid; stop all processing.
						return err
					} else {
						// os.ReadDir(path) failed; skip the directory.
						return nil
					}
				}

				chain := MakeChain(NewLink(root, c, d))

				// Before recursing down a directory, verify we won't exceed MaxDepth
				depth := len(strings.FieldsFunc(strings.TrimPrefix(chain.Head().Path(), root),
					func(r rune) bool { return r == os.PathSeparator })) + option.fromDepth
				//fmt.Printf("[%d] %s // %s\n", depth, root, c)
				if d.IsDir() && depth >= option.MaxDepth {
					// Stop processing this subtree if it exceeds MaxDepth.
					return fs.SkipDir
				}

				// Special processing for symlinks if we should follow them.
				if option.FollowSymlinks && chain.Head().IsSymlink() {

					ptr := chain.Head()

					// Repeatedly dereference the symlink until we have a regular file.
					for {
						dest, err := ptr.Deref()
						if err != nil {
							return nil // Just ignore the symlink if there is any error.
						}
						chain.Add(&dest)
						ptr = &dest
						if !ptr.IsSymlink() {
							break // Dereferenced file is not a symlink; stop dereferencing.
						}
					}

					// At this point, chain.Head() refers to the original symlink, and ptr
					// refers to the regular file/dir to which it linked (directly or
					// indirectly, in the case of nested symlinks).

					// Check if symlink referred to a directory.
					if ptr.ent.IsDir() {
						// Regardless of the number of indirections, we consider it having
						// recursed only 1 level. Verify that it doesn't exceed MaxDepth.
						if depth+1 <= option.MaxDepth {
							// Copy our existing Options, and update traversal counters so
							// that the recursive call to Match can accurately keep track
							// (which can not be computed by simply counting the number
							// of directories between our Walk root and current descendent).
							//
							// This only modifies the copied Options struct;
							//   the Options from the caller's context remain unmodified.
							lopt := option
							lopt.fromDepth = depth
							// Stop following symlinks as soon as we exceed MaxFollow.
							lopt.fromFollow++
							lopt.FollowSymlinks = lopt.fromFollow < lopt.MaxFollow ||
								lopt.MaxFollow < 0 // Negative = unlimited dereferences

							mfound, merr := Match(lopt, pattern, ptr.Path())
							// Just ignore the symlink if there is an error of any sort.
							if merr == nil {
								found = append(found, mfound...)
							}
						}
					}

					// Update our DirEntry and current path to refer to our dereferenced
					// file/directory.
					d = ptr.ent
					c = ptr.Path()
				}

				// Finally, if current file is not a directory, test if it matches the
				// user-provided pattern.
				if !d.IsDir() {
					base := path.Base(chain.Head().name)
					if option.IgnoreCase {
						base = strings.ToLower(base)
					}
					ok, merr := option.Expr.Match(pattern, base)
					if merr != nil {
						// If there was an error with matching, stop processing completely
						// because the pattern is invalid.
						return merr
					} else if ok {
						// No error, add the current chain to our list of matches.
						found = append(found, chain.String())
					}
				}

				// Continue processing.
				return nil
			})

		if werr != nil {
			serr = append(serr, errWalkDir{dir: root, err: werr})
		}
	}

	// Ensure the returned error is nil unless we have added elements to serr.
	if len(serr) > 0 {
		return found, serr
	}
	return found, nil
}
