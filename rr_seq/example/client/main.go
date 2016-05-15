package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/teh-cmc/seq/rr_seq"
)

// -----------------------------------------------------------------------------

var s *rrs.RRSeq

func usage() string { return "./client seq_name buf_size ms_delay addr1 [addrN...]" }

func main() {
	flag.Parse()
	if flag.NArg() < 4 {
		fmt.Println("Usage:", usage())
		os.Exit(1)
	}

	bufSize, err := strconv.ParseInt(flag.Arg(1), 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	delay, err := strconv.ParseInt(flag.Arg(2), 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	s, err = rrs.NewRRSeq(flag.Arg(0), int(bufSize), flag.Args()[3:]...)
	ids := s.Stream()
	log.Println("ready")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

infLoop:
	for {
		select {
		case <-c:
			break infLoop
		case <-time.After(time.Duration(delay) * time.Millisecond):
			log.Println("Got:", ids.Next())
		}
	}

	_ = s.Close()
}
