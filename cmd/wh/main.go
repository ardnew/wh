package main

import (
	"fmt"
	"os"

	"github.com/ardnew/wh"
)

func main() {
	opt := wh.Option{
		FollowSymlinks: false,
		MaxDepth: 1,
		RealPath: true,
	}
	f, err := wh.MatchGlob(opt, os.Args[1], os.Args[2:]...)
	fmt.Printf("found = %#v\n\nerr = %#v\n", f, err)
}
