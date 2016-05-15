package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/teh-cmc/seq/rr_seq"
)

// -----------------------------------------------------------------------------

var s *rrs.RRServer

func usage() string { return "./server addr path peer1_addr [peerN_addr...]" }

func main() {
	flag.Parse()
	if flag.NArg() < 3 {
		fmt.Println("Usage:", usage())
		os.Exit(1)
	}

	s, err := rrs.NewRRServer(flag.Arg(0), flag.Arg(1), flag.Args()[2:]...)
	log.Println("ready")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	_ = s.Close()
}
