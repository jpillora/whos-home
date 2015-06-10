package main

import (
	"log"
	"os"
	"time"

	"github.com/jpillora/opts"
	"github.com/jpillora/whos-home/daemon"
)

func main() {

	c := &daemon.Config{
		Interval: 30 * time.Second,
	}

	opts.New(c).
		Version("0.2.0").
		PkgRepo().
		Parse()

	log.SetOutput(os.Stderr)
	daemon.Run(*c)
}
