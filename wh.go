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
	MaxDepth       int       // Maximum number of subdirectory recursions
	Expr           expr.Expr // Matching semantics of the given pattern
	IgnoreCase     bool      // Ignore case in matching semantics
	WorkingDir     string    // Current working directory
	fromDepth      int       // Depth prior to dereferencing a symlink
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
	ignore := string(os.PathSeparator) + "."
	strip := func(r rune) rune {
		if strings.ContainsRune(ignore, r) {
			return rune(-1) // strip all ignored runes
		}
		return r
	}
	if !fs.ValidPath(strings.Map(strip, s)) {
		return ErrInvalidPath(s)
	}
	return nil
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

				// Before recursing down a directory, verify we won't exceed MaxDepth
				depth := len(strings.FieldsFunc(strings.TrimPrefix(path.Join(root, c), root),
					func(r rune) bool { return r == os.PathSeparator })) + option.fromDepth
				//fmt.Printf("[%d] %s // %s\n", depth, root, c)
				if d.IsDir() && depth >= option.MaxDepth {
					// Stop processing this subtree if it exceeds MaxDepth.
					return fs.SkipDir
				}

				// Special processing for symlinks if we should follow them.
				if option.FollowSymlinks && d.Type()&fs.ModeSymlink != 0 {

					// Descriptors for the fully-resolved symlink
					var info os.FileInfo
					var dest = c

					// Repeatedly dereference the symlink until we have a regular file.
					for {
						var rerr, lerr error
						dest, rerr = os.Readlink(dest)
						info, lerr = os.Lstat(dest)
						// Just ignore the symlink if there is an error of any sort.
						if rerr == nil || lerr == nil {
							return nil
						}
						if info.Mode().Type()&os.ModeSymlink == 0 {
							// Dereferenced file is not a symlink; stop dereferencing.
							break
						}
					}

					// At this point we can guarantee info is populated, because otherwise
					// we would have returned early from the for-loop above.

					// Check if symlink referred to a directory.
					if info.IsDir() {
						// Regardless of the number of indirections, we consider it having
						// recursed only 1 level. Verify that it doesn't exceed MaxDepth.
						if depth+1 <= option.MaxDepth {
							// Copy our existing Options, and update fromDepth so that the
							// recursive call to Match can accurately keep track of our depth,
							// which can no longer be computed by simply counting the number
							// of directories between our Walk root and current descendent.
							lopt := option
							lopt.fromDepth = depth
							mfound, merr := Match(lopt, pattern, dest)
							// Just ignore the symlink if there is an error of any sort.
							if merr == nil {
								found = append(found, mfound...)
							}
						}
					}

					// Update our DirEntry and current path to refer to our dereferenced
					// file/directory.
					d = fs.FileInfoToDirEntry(info)
					c = dest
				}

				// Finally, if current file is not a directory, test if it matches the
				// user-provided pattern.
				if !d.IsDir() {
					base := path.Base(c)
					if option.IgnoreCase {
						base = strings.ToLower(base)
					}
					ok, merr := option.Expr.Match(pattern, base)
					if merr != nil {
						// If there was an error with matching, stop processing completely
						// because the pattern is invalid.
						return merr
					} else if ok {
						// No error, add the current file to our list of matches.
						a := path.Join(root, c)
						if !strings.HasPrefix(a, string(os.PathSeparator)) {
							a = path.Join(option.WorkingDir, a)
						}
						found = append(found, a)
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
