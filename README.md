[docimg]:https://godoc.org/github.com/ardnew/wh?status.svg
[docurl]:https://godoc.org/github.com/ardnew/wh
[repimg]:https://goreportcard.com/badge/github.com/ardnew/wh
[repurl]:https://goreportcard.com/report/github.com/ardnew/wh

# wh
#### A `which` alternative with more options

[![GoDoc][docimg]][docurl] [![Go Report Card][repimg]][repurl]

The core functionality of `wh` is implemented as a Go module. An [executable of the same name](cmd/wh) is also provided for command-line usage.

## Usage

> TODO
```txt
  -0	Delimit output with null ('\0') instead of newline ('\n')
  -F	Use fixed string matching (default true)
  -L	Follow symbolic links
  -a	Report all matching files
  -d depth
    	Limit directory traversal to depth levels (default 1)
  -e	Use regular expression pattern matching
  -g	Use glob pattern matching
  -i	Use case-insensitive matching
  -p path-list
    	Search only in path-list (can be specified multiple times)
  -q	Print nothing; status indicates match found
  -s count
    	Dereference up to count chains of symbolic links (-1 = unlimited)
  -w	Print warning and diagnostic messages
```

## Installation

> TODO

