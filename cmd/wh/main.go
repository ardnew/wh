package main

import (
	"os"

	"github.com/ardnew/wh"
)


func main() {
	opt := wh.Options{
		FollowSymlinks: false,
		MaxDepth: 1,
		RealPath: true,
	}
	wh.MatchGlob(opt, os.Args[1], os.Args[2:])
}
