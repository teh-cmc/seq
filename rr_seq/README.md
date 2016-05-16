# RRSeq

This package implements a distributed system that guarantees the generation of sequential `ID`s by using RW quorums and read-repair conflict-resolution strategies.

It is a direct, heavily documented, tested & benchmarked implementation of the `leaderless consistency` strategy described at the [root of this repository](/).

**Table of contents**

- [RRSeq](#rrseq)
  - [Installing](#installing)
  - [Quickstart](#quickstart)
  - [Performance](#performance)

## Installing

The easiest way is to `go get` all the things:

```
go get -u github.com/teh-cmc/seq/...
```
:warning: This most likely will return a `no buildable Go source files` error. You can safely ignore it.

## Quickstart

This quickstart will show you how to start your own cluster of `RRServer`s and `RRSeq`s.  
You'll then be able to experiment with it at will.

Everything that follows assumes that your current working directory is [`/rr_seq`](/rr_seq).

**I. Running the tests**

First thing first, you should make sure that all the tests pass without any issue.

```
go test -cpu 1,4,8 -run=. -bench=none -cover -v
```
:warning: You should expect some innocuous `grpc` warnings when executing these commands.

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
	if flag.NArg() < 3 {
		fmt.Println("Usage:", usage())
		os.Exit(1)
	}

	var f *os.File
	var err error

	f, err = os.Open(flag.Arg(1))
	if err != nil {
		f, err = os.Create(flag.Arg(1))
		if err != nil {
			log.Fatal(err)
		}
	}
	_ = f.Close()

	s, err := rrs.NewRRServer(flag.Arg(0), flag.Arg(1), flag.Args()[2:]...)
	if err != nil {
		log.Fatal(err)
	}

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
:warning: You should expect some innocuous `grpc` warnings when executing these commands.

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
:warning: You should expect some innocuous `grpc` warnings when executing these commands.

That's it, you now have 2 `RRSeq`s fetching sequential IDs from your cluster of 3 `RRServer`s.

**IV. Experimenting**

Now that you have a running cluster with a bunch of clients connected to it, you can start experimenting however you like. Here are some ideas:

- `SIGINT`/`SIGQUIT`/`SIGKILL` some or all of the servers and see what happens
- `SIGINT`/`SIGQUIT`/`SIGKILL` some or all of the clients and see what happens
- `rm` some or all of the persistent logs and see what happens
- any combination of the above

:warning: Due to `grpc`'s exponential backoff algorithm used for reconnections, it can take quite some time for a `RRSeq` to reconnect to a `RRServer`.

TL;DR: wreak havoc.

## Performance

### Running the benchmarks

```
go test -cpu 1,8,32 -run=none -bench=. -cover -v
```
:warning: You should expect some innocuous `grpc` warnings when executing these commands.

Here are the results using a `DELL XPS 15-9530 (i7-4712HQ@2.30GHz)`:

```
BenchmarkRRSeq_BufSize0_SingleClient                       	    1000	   1026260 ns/op
BenchmarkRRSeq_BufSize0_SingleClient-8                     	    3000	    500635 ns/op
BenchmarkRRSeq_BufSize0_SingleClient-32                    	    3000	    512832 ns/op
BenchmarkRRSeq_BufSize1_SingleClient                       	    1000	   1062353 ns/op
BenchmarkRRSeq_BufSize1_SingleClient-8                     	    3000	    514863 ns/op
BenchmarkRRSeq_BufSize1_SingleClient-32                    	    3000	    518578 ns/op
BenchmarkRRSeq_BufSize2_SingleClient                       	    2000	    551448 ns/op
BenchmarkRRSeq_BufSize2_SingleClient-8                     	    5000	    260458 ns/op
BenchmarkRRSeq_BufSize2_SingleClient-32                    	    5000	    263286 ns/op
BenchmarkRRSeq_BufSize1024_SingleClient                    	 1000000	      1261 ns/op
BenchmarkRRSeq_BufSize1024_SingleClient-8                  	 2000000	       714 ns/op
BenchmarkRRSeq_BufSize1024_SingleClient-32                 	 2000000	       707 ns/op
BenchmarkRRSeq_BufSize0_ConcurrentClients_Local            	    1000	   1121957 ns/op
BenchmarkRRSeq_BufSize0_ConcurrentClients_Local-8          	    3000	    530608 ns/op
BenchmarkRRSeq_BufSize0_ConcurrentClients_Local-32         	    3000	    547488 ns/op
BenchmarkRRSeq_BufSize1_ConcurrentClients_Local            	    2000	   1146762 ns/op
BenchmarkRRSeq_BufSize1_ConcurrentClients_Local-8          	    3000	    538000 ns/op
BenchmarkRRSeq_BufSize1_ConcurrentClients_Local-32         	    3000	    543174 ns/op
BenchmarkRRSeq_BufSize2_ConcurrentClients_Local            	    2000	    572742 ns/op
BenchmarkRRSeq_BufSize2_ConcurrentClients_Local-8          	    5000	    271561 ns/op
BenchmarkRRSeq_BufSize2_ConcurrentClients_Local-32         	    5000	    273733 ns/op
BenchmarkRRSeq_BufSize1024_ConcurrentClients_Local         	 1000000	      1315 ns/op
BenchmarkRRSeq_BufSize1024_ConcurrentClients_Local-8       	 2000000	       879 ns/op
BenchmarkRRSeq_BufSize1024_ConcurrentClients_Local-32      	 1000000	      1048 ns/op
BenchmarkRRSeq_BufSize0_ConcurrentClients_Distributed      	       1	1155801513 ns/op
BenchmarkRRSeq_BufSize0_ConcurrentClients_Distributed-8    	    2000	    907526 ns/op
BenchmarkRRSeq_BufSize0_ConcurrentClients_Distributed-32   	    2000	    955798 ns/op
BenchmarkRRSeq_BufSize1_ConcurrentClients_Distributed      	       1	1543643181 ns/op
BenchmarkRRSeq_BufSize1_ConcurrentClients_Distributed-8    	    1000	   1006642 ns/op
BenchmarkRRSeq_BufSize1_ConcurrentClients_Distributed-32   	    1000	   1042091 ns/op
BenchmarkRRSeq_BufSize2_ConcurrentClients_Distributed      	       1	1622989431 ns/op
BenchmarkRRSeq_BufSize2_ConcurrentClients_Distributed-8    	    3000	    507277 ns/op
BenchmarkRRSeq_BufSize2_ConcurrentClients_Distributed-32   	    2000	    534845 ns/op
BenchmarkRRSeq_BufSize1024_ConcurrentClients_Distributed   	       1	1129368563 ns/op
BenchmarkRRSeq_BufSize1024_ConcurrentClients_Distributed-8 	 2000000	       939 ns/op
BenchmarkRRSeq_BufSize1024_ConcurrentClients_Distributed-32	 1000000	      1065 ns/op
```

And the same results for `SimpleBufSeq` (still using a `DELL XPS 15-9530 (i7-4712HQ@2.30GHz)`):

```
BenchmarkSimpleBufSeq_BufSize0_SingleClient           	 3000000	       426 ns/op
BenchmarkSimpleBufSeq_BufSize0_SingleClient-8         	 3000000	       449 ns/op
BenchmarkSimpleBufSeq_BufSize0_SingleClient-32        	 3000000	       449 ns/op
BenchmarkSimpleBufSeq_BufSize1_SingleClient           	 5000000	       347 ns/op
BenchmarkSimpleBufSeq_BufSize1_SingleClient-8         	 5000000	       374 ns/op
BenchmarkSimpleBufSeq_BufSize1_SingleClient-32        	 5000000	       376 ns/op
BenchmarkSimpleBufSeq_BufSize1024_SingleClient        	10000000	       188 ns/op
BenchmarkSimpleBufSeq_BufSize1024_SingleClient-8      	10000000	       222 ns/op
BenchmarkSimpleBufSeq_BufSize1024_SingleClient-32     	10000000	       220 ns/op
BenchmarkSimpleBufSeq_BufSize0_ConcurrentClients      	 3000000	       421 ns/op
BenchmarkSimpleBufSeq_BufSize0_ConcurrentClients-8    	 3000000	       544 ns/op
BenchmarkSimpleBufSeq_BufSize0_ConcurrentClients-32   	 2000000	       654 ns/op
BenchmarkSimpleBufSeq_BufSize1_ConcurrentClients      	 5000000	       344 ns/op
BenchmarkSimpleBufSeq_BufSize1_ConcurrentClients-8    	 3000000	       510 ns/op
BenchmarkSimpleBufSeq_BufSize1_ConcurrentClients-32   	 2000000	       635 ns/op
BenchmarkSimpleBufSeq_BufSize2_ConcurrentClients      	 5000000	       315 ns/op
BenchmarkSimpleBufSeq_BufSize2_ConcurrentClients-8    	 3000000	       490 ns/op
BenchmarkSimpleBufSeq_BufSize2_ConcurrentClients-32   	 2000000	       630 ns/op
BenchmarkSimpleBufSeq_BufSize1024_ConcurrentClients   	10000000	       189 ns/op
BenchmarkSimpleBufSeq_BufSize1024_ConcurrentClients-8 	 5000000	       370 ns/op
BenchmarkSimpleBufSeq_BufSize1024_ConcurrentClients-32	 3000000	       597 ns/op
```

### Comparing performance with `SimpleBufSeq`

In order compare the performance of `SimpleBufSeq` and `RRSeq/RRServer`; we'll need to focus of 3 situations:

- 32 concurrent clients trying to fetch IDs from a `Sequencer` with a buffering size of `0`

  `SimpleBufSeq`:
  ```
  BenchmarkSimpleBufSeq_BufSize0_ConcurrentClients-32   	    2000000	       654 ns/op
  ```
  `RRSeq`:
  ```
  BenchmarkRRSeq_BufSize0_ConcurrentClients_Distributed-32   	   2000	    955798 ns/op
  ```

- 32 concurrent clients trying to fetch IDs from a `Sequencer` with a buffering size of `2`

  `SimpleBufSeq`:
  ```
  BenchmarkSimpleBufSeq_BufSize2_ConcurrentClients-32   	    2000000	       630 ns/op
  ```
  `RRSeq`:
  ```
  BenchmarkRRSeq_BufSize2_ConcurrentClients_Distributed-32   	   2000	    534845 ns/op
  ```

- 32 concurrent clients trying to fetch IDs from a `Sequencer` with a buffering size of `1024`

  `SimpleBufSeq`:
  ```
  BenchmarkSimpleBufSeq_BufSize1024_ConcurrentClients-32	    3000000	       597 ns/op
  ```
  `RRSeq`:
  ```
  BenchmarkRRSeq_BufSize1024_ConcurrentClients_Distributed-32   1000000	      1065 ns/op
  ```

The results are pretty much self explanatory.

With a small buffer size, `RRSeq` is basically DOS-ing itself out of existence.  
Once the buffer size starts to increase, the disk & network contention decreases proportionally:  
- Starting with an empty buffer, `RRSeq` is 3 orders of magnitude slower than `SimpleBufSeq`, i.e. barely usable;
- Once it reaches a 1024-wide buffer, `RRSeq` becomes less than 2 times slower than a `SimpleBufSeq`
