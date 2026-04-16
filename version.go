package main

import (
	"fmt"
	"io"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func runVersion(w io.Writer) {
	fmt.Fprintf(w, "ody %s (%s, built %s)\n", version, commit, date)
}
