# Seq ![Status](https://img.shields.io/badge/status-stable-green.svg?style=plastic) [![Build Status](http://img.shields.io/travis/teh-cmc/seq.svg?style=plastic)](https://travis-ci.org/teh-cmc/seq) [![GoDoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=plastic)](http://godoc.org/github.com/teh-cmc/seq)

This repository offers a gentle overview of the possible design solutions to the common problem of generating sequential / monotonically increasing IDs in a distributed system.

Specifically, it focuses on maximizing performances and guaranteeing a fair distribution of the workload between the nodes as the size of the cluster increases.

If you're looking for the actual implementation's README, [click here](/rr_seq).

**Table of contents**

- [Seq](#seq)
  - [Organization](#organization)
  - [Common designs](#common-designs)
    - [Consensus protocols (Leader/Followers)](#consensus-protocols-leaderfollowers)
      - [Batching](#batching)
      - [Further reading](#further-reading)
    - [Leaderless consistency](#leaderless-consistency)
      - [Further reading](#further-reading-1)
    - [The Flake model](#the-flake-model)
      - [How?](#how)
      - [Trade-offs](#trade-offs)
      - [Further reading](#further-reading-2)
  - [License](#license-)

## Organization

```
(non-interesting stuff omitted)
.
├── (1) README.md
├── (2) sequencer.go
├── (3) simple_buf_seq
├── (4) rpc
├── (5) rr_seq
└── (6) rr_seq/README.md
```

**(1)**: This [document](/README.md) (succinctly) presents various ways of tackling the problem of distributed sequences. It links to more detailed and particularly interesting readings when necessary.

**(2)**: This [file](/sequencer.go) implements the `ID` type as well as the `Sequencer` interface, both of which the following packages depends on.

**(3)**: [Package `simple_buf_seq`](/simple_buf_seq) implements a simple, non-distributed, buffered `Sequencer` backed by a local, atomic, monotonically increasing 64bits value.  
A `SimpleBufSeq` is not particularly interesting in and of itself; but it provides a performance baseline that can, and will, later be used as a point of comparison for more complex implementations.

**(4)**: [Package `rpc`](/rpc) implements a simple round-robin connection pool for GRPC connections. Not that interesting, but necessary nonetheless.

**(5)**: [Package `rr_seq`](/rr_seq) implements a distributed system that guarantees the generation of sequential `ID`s by using RW quorums and read-repair conflict-resolution strategies.  
It is a direct, heavily documented, tested & benchmarked implementation of the `leaderless consistency` strategy described below.

**(6)**: This [document](/rr_seq/README.md) explains how to start using the `rr_seq` package, and gives some insights about its performances.

## Common designs

*Please note that the following might contain mistakes. Corrections and patches are more than welcome.*

### Consensus protocols (Leader/Followers)

Perhaps the most obvious way of solving this problem is to implement a distributed locking mechanism upon a consensus protocol such as [Raft](https://raft.github.io/).

In fact, several tools like e.g. [Consul](https://www.consul.io/) and [ZooKeeper](https://zookeeper.apache.org/) already exist out there, and provide all the necessary abstractions for emulating atomic integers in a distributed environment; out of the box.

Using these capabilities, it is quite straightforward to expose an atomic `get-and-incr` endpoint for clients to query.

**pros**:

- Consistency & sequentiality guarantees  
  By using a quorum combined with a Leader/Followers model, the system can guarantee the generation of monotonically increasing IDs over time.
- Fault-tolerancy
  The system can and will stay available as long as N/2+1 (i.e. a quorum) nodes are still available.

**cons**:

- Poor performance  
  Since every operation requires communication between nodes, most of the time is spent in costly network I/O.  
  Also, if the system wants to guarantee consistency even in the event of total failure (i.e. all nodes simultaneously go down), it *must* persist every increment to disk as they happen.  
  This obviously leads to even worse performances, as the system now spends another significant part of its time doing disk I/O too.  
- Uneven workload distribution  
  Due to the nature of the Leader/Followers model; a single node, the Leader, is in charge of handling all of the incoming traffic (e.g. serialization/deserialization of RPC requests).  
  This also means that, if the Leader dies, the whole system is unavailable for as long as it takes for a new Leader to be elected.

#### Batching

A simple enhancement to the *consensus protocols* approach above is to batch the fetching of IDs.  
I.e., instead of returning a single ID every time a client queries the service, the system would return a *range* of available IDs.

This has the obvious advantage of greatly reducing the number of network calls necessary to obtain N IDs (depending on the size of the ranges used), thus fixing the performance issues of the basic approach.  
However, it introduces a new issue: what happens if a client who's just fetched a range of IDs crashed? Those IDs will be definitely lost, creating potentially large cluster-wide sequence "gaps" in the process.

There are 3 possible solutions to this new problem, each coming with its own set of trade-offs:  
- Accept the fact that your sequence as a whole might have "gaps" in it  
  The performance boost might be worth the cost if the potential discontinuity of the sequence is not considered an issue at the application level.  
- Keep track, server-side, of the current range associated with each client, and use it when a client comes back from the dead  
  Aside from the evidently added complexity of keeping track of all this information; this could cause scalability issues as the number of *uniquely identified* clients increases.  
  Also, what if a previously dead client never goes back online?  
- Keep track, on each client, of its current range and make sure to persist it to disk
  Aside from the evidently added [complexity inherent to client-side caching](http://martinfowler.com/bliki/TwoHardThings.html); this would create a particularly difficult situation due to the fact that the client might crash *after* having received a new range, but *before* having persisted it to disk.  
  The only way to fix that would be to keep the server on hold until the client has notified it about the success of the disk synchronization... How will this affect performance?  
  Also, what if a client never comes back from the dead?  

Those new trade-offs probably have solutions of their own, which would certainly bring even more trade-offs and some more solutions to the table.  
**TL;DR**: As with everything else in life, this is a perpetually recursive rabbit-hole of trade-offs.

Note that the basic approach is just a special case of the batching approach that comes with a batch size of 1; meaning that every issues that applies to one actually applies to the other.  
I.e. cluster-wide "gaps" are definitely possible even without batches.

#### Further reading

- [thesecretlivesofdata.com](http://thesecretlivesofdata.com/raft/) offers a great visual introduction to the inner workings of the Raft protocol.
- ["In Search of an Understandable Consensus Algorithm"](http://ramcloud.stanford.edu/raft.pdf), the official Raft paper, describes all the nitty gritty implementation details of such a system.

### Leaderless consistency

A variation of the above approach consists of removing the need for a centralized Leader by using conflict-resolution strategies.  
A common strategy is to combined RW quorums with self-repairing reads.

If those concepts seem a bit too distant for you, have a look at the *Further reading* section below; those articles will make everything crystal clear.  
In any case, I'll present a rapid explanation of the basic idea.

Let's define:  
- N: the number of nodes in the cluster
- R: the number of nodes to read from when querying for a new ID
- W: the number of nodes that must acknowledge the value of the next generated ID

If `R + W > N`, then the read set and the write set *always* overlap, meaning that *at least one* of `R` results is the most recent ID in the cluster.  
Using a simple read-repair conflict resolution strategy, we can always keep the highest ID from the read set; thus essentially implementing a leader-less quorum.

This solution offers the same exact pros & cons as the basic *consensus protocols* approach; with the only difference that it completely removes the need for a centralized Leader that has to handle every single incoming query.  
The load is now evenly balanced between the nodes.

Of course, network & disk I/O, distributed lock contention et al. are still the main restraint to overall performance; but you really don't have a choice as long as you need a shared state between your nodes... Unless you go for the [Flake model](#the-flake-model), that is.

Package [`RRSeq`](/rr_seq) is a direct, heavily documented, tested & benchmarked implementation of this leaderless strategy.

#### Further reading

- Werner Vogels' [famous post on consistency models](http://www.allthingsdistributed.com/2008/12/eventually_consistent.html) is certainly a must read when it comes to consistency in distributed systems.
- Riak's [replication properties](http://docs.basho.com/riak/kv/2.1.4/developing/app-guide/replication-properties/) is a great example of using {N,R,W} knobs and conflict resolution strategies to adjust trade-offs between consistency and availability.
- [Peter Bourgon's talk](https://www.youtube.com/watch?v=em9zLzM8O7c) about (mostly) [Soundclound's Roshi](https://github.com/soundcloud/roshi) presents a system that achieves consistency without consensus, using CRDTs and self-repairing reads and writes.

### The Flake model

Although it does not provide sequential IDs per se; the Flake-ish way (named after twitter's [Snowflake](https://github.com/twitter/snowflake/tree/b3f6a3c6ca8e1b6847baa6ff42bf72201e2c2231)) of doing things is such an elegant and performant solution that I *had* to mention it here.  
The Flake model allows for the generation of unique, roughly time-sortable IDs in a distributed, **shared-nothing** architecture; thus guaranteeing horizontal linear scaling.

#### How?

The basic idea is fairly simple: instead of working with simple integers that you increment each time you need a new ID, you define an ID as the result of the bit-packing of various values.  
As an example, the original implementation used to use 64bits integers with the following distribution of bits (extracted from Snowflake's documentation):

```
timestamp - 41 bits (millisecond precision w/ a custom epoch gives us 69 years)
configured machine id - 10 bits - gives us up to 1024 machines
sequence number - 12 bits - rolls over every 4096 per machine (with protection to avoid rollover in the same ms)
```

#### Trade-offs

Although this model offers you great performance and linear horizontal scalability, it comes with some possibly serious trade-offs:  
- Using the above distribution, you cannot:
  - have more than 1024 machines in your cluster
  - handle more than 4096 queries per millisecond per machine
  - given a cluster of N machines, guarantee the ordering of M IDs that were generated within a range of N milliseconds
- The system relies on wall-clock time
  There is a *lot* of literature out there about the dangers of non-logical time in distributed systems (..even with a perfectly configured `ntpd`), so I won't go into details; check the `Further reading` section if you're curious about those things.

#### Further reading

- Justin Sheehy's ["There is No Now"](http://queue.acm.org/detail.cfm?id=2745385) is a great and thorough article regarding time in distributed environments.
- Martin Kleppmann's [post about Redis' Redlock](https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html) is a fantastic analysis of how-so unfortunate timing issues can have serious consequences in distributed systems.
- Mikito Takada (aka. Mixu)'s short book: ["Distributed systems: for fun and profit"](http://book.mixu.net/distsys/single-page.html) is a classic introduction to distributed systems with a section dedicated to the subject of timing and ordering assumptions.

## License ![License](https://img.shields.io/badge/license-MIT-blue.svg?style=plastic)

The MIT License (MIT) - see LICENSE for more details

Copyright (c) 2015	Clement 'cmc' Rey	<cr.rey.clement@gmail.com> [@teh_cmc](https://twitter.com/teh_cmc)
