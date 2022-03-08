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
	FollowSymlinks bool // Follow symlinks when recursing into subdirectories
	MaxDepth int // Maximum number of subdirectory recursions
	Expr expr.Expr // Matching semantics of the given pattern
	RealPath bool // Return real absolute path to all matches
}

// MatchFixed returns the result of calling Match with the given string pattern
// used to match file names verbatim.
func MatchFixed(option Option, pattern string, path ...string) ([]string, error) {
	option.Expr = expr.Fixed
	return Match(option, pattern, path...)
}

// MatchGlob returns the result of calling Match with the given string pattern
// used to match file names according to path.Match semantics.
func MatchGlob(option Option, pattern string, path ...string) ([]string, error) {
	option.Expr = expr.Glob
	return Match(option, pattern, path...)
}

// MatchRegexp returns the result of calling Match with the given string pattern
// used to match file names according to regexp.Regexp semantics.
func MatchRegexp(option Option, pattern string, path ...string) ([]string, error) {
	option.Expr = expr.Regexp
	return Match(option, pattern, path...)
}

type ErrMaxDepth int

// Error returns a descriptive error string for the receiver ErrMaxDepth e.
func (e ErrMaxDepth) Error() string {
	return "maximum depth (" + strconv.Itoa(int(e)) + ") exceeded"
}

func Match(option Option, pattern string, sub ...string) ([]string, error) {
	found := []string{}
	for _, p := range sub {
		root := path.Clean(p)
		fs.WalkDir(os.DirFS(root), ".",
			func(c string, d fs.DirEntry, err error) error {
				fmt.Printf("path=%s, d=%#v, err=%v\n", c, d, err)
				if err != nil {
					if d == nil {
						// the root path os.DirFS(p) was invalid
						return err
					} else {
						// os.ReadDir(path) failed, skip the directory
						return nil
					}
				}
				depth := len(strings.FieldsFunc(strings.TrimPrefix(path.Join(root, c), root),
					func(r rune) bool { return r == os.PathSeparator }))
				fmt.Printf("depth=%d, path=%s, root=%s\n", depth, path.Join(root, c), root)
				if d.IsDir() && depth >= option.MaxDepth {
					return fs.SkipDir
				}
				if !d.IsDir() {
					if option.FollowSymlinks && d.Type() & fs.ModeSymlink != 0 {
						//dest, oerr := os.Readlink(path)
					} else {
						ok, merr := option.Expr.Match(pattern, c)
						if merr != nil {
							return merr
						} else if ok {
							found = append(found, c)
						}
					}
				}
				return nil
			})
	}
	return found, nil
}
