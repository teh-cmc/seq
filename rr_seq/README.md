# RRSeq

This package implements a distributed system that guarantees the generation of sequential `ID`s by using RW quorums and read-repair conflict-resolution strategies.

It is a direct, heavily documented, tested & benchmarked implementation of the `leaderless consistency` strategy described at the [root of this repository](/).

**Table of contents**

- [RRSeq](#rrseq)
  - [Quickstart](#quickstart)
  - [Performance](#performance)

## Quickstart

This quickstart will show you how to start your own cluster of `RRServer`s and `RRSeq`s.  
You'll then be able to experiment with it at will.

Everything that follows assumes that your current working directory is [`/rr_seq`](/rr_seq).

**I. Running the tests**

First thing first, you should make sure that all the tests pass without any issue.

```
go test -cpu 1,4,8 -run=. -bench=none -cover -v
```
(!) You should expect some innocuous `grpc` warnings when executing these commands.

**II. Starting some servers**

[`example/server/main.go`](example/server/main.go) implements a simple CLI program that starts a `RRServer` with the given parameters.

This code is pasted below for convenience:

```Go
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
	if flag.NArg() < 2 {
		fmt.Println("Usage:", usage())
		os.Exit(1)
	}

	s, err = rrs.NewRRServer(flag.Arg(0), flag.Arg(1), flag.Args()[2:]...)
	log.Println("ready")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	_ = s.Close()
}
```

Starting a server is done as the following:

```
./server addr path peer1_addr [peerN_addr...]
```

Let's start a cluster of 3 servers, listening on ports `19001`, `19002` and `19003`, respectively:

```
go run example/server/main.go :19001 /tmp/serv1_persist :19002 :19003
go run example/server/main.go :19002 /tmp/serv2_persist :19001 :19003
go run example/server/main.go :19003 /tmp/serv3_persist :19001 :19002
```
(!) You should expect some innocuous `grpc` warnings when executing these commands.

That's it, you're now running a cluster composed 3 `RRServer`s.

**III. Starting some clients**

[`example/client/main.go`](example/client/main.go) implements a simple CLI program that starts a `RRSeq` with the given parameters.

This code is pasted below for convenience:

```Go
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
			select {
			case id := <-ids:
				log.Println("Got:", id)
			default:
			}
		}
	}

	_ = s.Close()
}
```

Starting a client is done as the following:

```
./client seq_name buf_size ms_delay addr1 [addrN...]
```

Let's start 2 clients, both using the sequence named `myseq`.  
This first client will have a buffer size of `1000` and will try to fetch a new ID every `10ms`; while the second client will have a buffer size of `1250` and will try to fetch a new ID every `5ms`:


```
go run example/client/main.go myseq 1000 10 :19001 :19002 :19003
go run example/client/main.go myseq 1250 5  :19001 :19002 :19003
```
(!) You should expect some innocuous `grpc` warnings when executing these commands.

That's it, you now have 2 `RRSeq`s fetching sequential IDs from your cluster of 3 `RRServer`s.

**IV. Experimenting**

Now that you have a running cluster with a bunch of clients connected to it, you can start experimenting however you like. Here are some ideas:

- `SIGINT`/`SIGQUIT`/`SIGKILL` some or all of the servers and see what happens
- `SIGINT`/`SIGQUIT`/`SIGKILL` some or all of the clients and see what happens
- `rm` some or all of the persistent logs and see what happens
- any combination of the above

TL;DR: wreak havoc.

## Performance
