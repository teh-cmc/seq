package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"

	"github.com/teh-cmc/seq/rr_seq"
)

// -----------------------------------------------------------------------------

var s *rrs.RRServer

func usage() string { return "./server addr peer1_addr [peerN_addr...]" }

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Println("Usage:", usage())
		os.Exit(1)
	}

	f, err := ioutil.TempFile("", "server_example")
	if err != nil {
		log.Fatal(err)
	}
	path := f.Name()
	_ = f.Close()

	s, err = rrs.NewRRServer(flag.Arg(0), path, flag.Args()[1:]...)
	log.Println("ready")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	_ = s.Close()
}
