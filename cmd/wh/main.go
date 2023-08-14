package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ardnew/wh"
)

// ErrNotFound represents an error in which the given file name pattern was not
// found in any searched directory.
type ErrNotFound []string

// Error returns a descriptive error string for the receiver ErrNotFound e.
func (e ErrNotFound) Error() string {
	if len(e) == 1 {
		return "not found: " + e[0]
	} else {
		t := make([]string, len(e))
		for i, s := range e {
			t[i] = fmt.Sprintf("%q", s)
		}
		return "not found: [" + strings.Join(t, ", ") + "]"
	}
}

// ErrNoArg represents an error in which no search patterns were provided.
type ErrNoArg bool

// Error returns a descriptive error string for the receiver ErrNoArg e.
func (ErrNoArg) Error() string {
	return "no search pattern"
}

// PathFlag contains each path found in each occurrence of its corresponding
// command-line flag.
type PathFlag []string

// Set implements the flag.Value interface's Set method.
// The given string s may be either a regular file path or a list of file paths,
// delimited by the OS-specific separator (":" on Unix, ";" on Windows).
// Each path from the given list is added to the receiver slice individually, so
// that the receiver contains only regular file paths.
// An error is returned for the first path encountered that contains invalid
// symbols, if any, or otherwise nil.
func (p *PathFlag) Set(s string) error {
	if p == nil {
		p = &PathFlag{}
	}
	for _, f := range strings.Split(s, string(os.PathListSeparator)) {
		if err := wh.ValidPath(f); err != nil {
			return err
		}
		*p = append(*p, f)
	}
	return nil
}

// String returns a descriptive string of the receiver *PathFlag p.
func (p *PathFlag) String() string {
	t := make([]string, len(*p))
	for i, s := range *p {
		t[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(t, ", ") + "]"
}

type flags struct {
	*flag.FlagSet
	dir PathFlag
	opt wh.Option
}

func main() {

	fl := flags{FlagSet: flag.NewFlagSet("wh", flag.ContinueOnError), dir: PathFlag{}}
	fl.Usage = fl.PrintDefaults

	var fixedFlag, globFlag, regexpFlag bool
	var allFlag, nullFlag, quietFlag, warnFlag bool

	fl.BoolVar(&fl.opt.FollowSymlinks, "L", false, "Follow symbolic links")
	fl.IntVar(&fl.opt.MaxFollow, "s", 0, "Dereference up to `count` chains of symbolic links (-1 = unlimited)")
	fl.IntVar(&fl.opt.MaxDepth, "d", 1, "Limit directory traversal to `depth` levels")
	fl.BoolVar(&fixedFlag, "F", true, "Use fixed string matching")
	fl.BoolVar(&globFlag, "g", false, "Use glob pattern matching")
	fl.BoolVar(&regexpFlag, "e", false, "Use regular expression pattern matching")
	fl.BoolVar(&fl.opt.IgnoreCase, "i", false, "Use case-insensitive matching")
	fl.BoolVar(&allFlag, "a", false, "Report all matching files")
	fl.BoolVar(&nullFlag, "0", false, "Delimit output with null ('\\0') instead of newline ('\\n')")
	fl.BoolVar(&quietFlag, "q", false, "Print nothing; status indicates match found")
	fl.BoolVar(&warnFlag, "w", false, "Print warning and diagnostic messages")
	fl.Var(&fl.dir, "p", "Search only in `path-list` (can be specified multiple times)")

	var errWriter, outWriter io.Writer = os.Stderr, os.Stdout

	if err := fl.Parse(os.Args[1:]); err != nil {
		halt(errWriter, err)
	}

	if quietFlag {
		errWriter = io.Discard
		outWriter = io.Discard
	}
	fl.SetOutput(outWriter)

	eol := "\n"
	if nullFlag {
		eol = "\x00"
	}

	if len(fl.Args()) == 0 {
		halt(errWriter, ErrNoArg(true), fl.PrintDefaults)
	}

	fn := wh.MatchFixed
	if regexpFlag {
		fn = wh.MatchRegexp
	} else if globFlag {
		fn = wh.MatchGlob
	}

	fl.opt.WorkingDir = "."
	if w, err := os.Getwd(); err == nil {
		fl.opt.WorkingDir = w
	}

	if len(fl.dir) == 0 {
		var err error
		if p, ok := os.LookupEnv("PATH"); ok {
			err = fl.dir.Set(p)
		} else {
			err = fl.dir.Set(fl.opt.WorkingDir)
		}
		if err != nil {
			halt(errWriter, err)
		}
	}

	found := []string{}
	warns := []error{}
	for _, a := range fl.Args() {
		f, err := fn(fl.opt, a, fl.dir...)
		if err != nil {
			warn := fmt.Errorf("warning: %w", err)
			if warnFlag {
				fmt.Fprintln(errWriter, warn)
			} else {
				warns = append(warns, warn)
			}
		}
		if !allFlag && len(f) > 0 {
			found = f[0:1]
			break
		}
		found = append(found, f...)
	}

	if len(found) == 0 {
		if !warnFlag {
			for _, w := range warns {
				fmt.Fprintln(errWriter, w)
			}
		}
		halt(errWriter, ErrNotFound(fl.Args()))
	}

	for _, f := range found {
		fmt.Fprintf(outWriter, "%s%s", f, eol)
	}
}

func halt(w io.Writer, err error, final ...func()) {
	if err != nil {
		if len(final) > 0 {
			for _, f := range final {
				f()
			}
		} else {
			fmt.Fprint(w, "error: ")
			fmt.Fprintln(w, err)
		}
		switch err.(type) {
		case ErrNotFound:
			os.Exit(1)
		case ErrNoArg:
			os.Exit(2)
		case wh.ErrWalkDir:
			os.Exit(3)
		case wh.ErrInvalidPath:
			os.Exit(4)
		default:
			if err == flag.ErrHelp {
				os.Exit(0)
			}
			os.Exit(127)
		}
	}
}
