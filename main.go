package main

import (
	"log"
	"os"
	"time"

	"github.com/jpillora/opts"
	"github.com/jpillora/whos-home/daemon"
)

var VERSION = "0.0.0" //set via ldflags

func main() {

	c := &daemon.Config{
		Interval: 30 * time.Second,
	}

	opts.New(c).
		Version(VERSION).
		Repo("github.com/jpillora/whos-home").
		Parse()

	log.SetOutput(os.Stderr)
	daemon.Run(*c)
}
