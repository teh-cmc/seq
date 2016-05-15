# Seq

This repository offers a gentle overview of the possible design solutions to the common problem of generating sequential / monotonically increasing IDs in a distributed system.  
Specifically, it focuses on maximizing performances and guaranteeing a fair distribution of the workload between nodes as the size of the cluster increases.

## Organization

```
.
├── (1) README.md
├── (2) sequencer.go
├── (3) simple_buf_seq
├── (4) rpc
└── (5) rr_seq
```

**(1)**: This document (succinctly) presents various ways of tackling the problem of distributed sequences. It links to more detailed related readings when necessary.  
**(2)**: This file implements the `ID` type as well as the `Sequencer` interface, both of which the following packages depends on.  
**(3)**: Package `simple_buf_seq` implements a simple, non-distributed, buffered `Sequencer` backed by a local, atomic, monotonically increasing 64bits value.  
A `SimpleBufSeq` is not particularly interesting in and of itself; but it provides a performance baseline that can, and will, later be used as a point of comparison for more complex implementations.  
**(4)**: Package `rpc` implements a simple round-robin connection pool for GRPC connections. Not that interesting, but necessary nonetheless.  
**(5)**: Package `rr_seq` implements a distributed system that guarantees sequential `ID` generation by using RW quorums and read-repair conflict-resolution strategies.  
It is a direct, heavily documented, tested & benchmarked implementation of the `read-repair + client-side caching` strategy described below.

## Possible designs

### Consensus protocols

Perhaps the most obvious and straightforward way of solving this problem is to implement a distributed locking mechanism upon a consensus protocol such as [Raft](https://raft.github.io/).

In fact, several tools such as [Consul](https://www.consul.io/) or [ZooKeeper](https://zookeeper.apache.org/) already exist out there, and provide all the necessary abstractions for emulating atomic integers across the network; out of the box.

Using these capabilities, it is quite straightforward to expose a `get-and-incr` atomic endpoint for clients to query.

**pros**:

- Strong consistency & sequentiality guarantees  
  Using a quorum, the system can A) guarantee the sequentiality of the IDs returned over time, and B) assure that there is no "holes" or "gaps" in the sequence.
- Good fault-tolerance guarantees  
  The system can and will stay available as long as N/2+1 nodes are still available.

**cons**:

- Poor performance  
  Since every operation requires communication between nodes, most of the time is spent in costly network IO.
- Uneven workload distribution  
  Due to the nature of the Leader/Follower model; a single node, the leader, is in charge of handling all of the incoming traffic (e.g. serialization/deserialization of RPC requests).

**Further reading**:

- [thesecretlivesofdata](http://thesecretlivesofdata.com/raft/) offers a great visual introduction to the inner workings of the Raft protocol.

### Consensus protocols + client-side caching

A simple enhancement to the *consensus protocols* approach is to batch the fetching of IDs: instead of returning a single ID every time a client queries the service, the system will allocate a *range* of available IDs and return this range to the client.

This has the obvious advantage of greatly reducing the number of network calls necessary to obtain N IDs (depending on the size of the ranges used); thus fixing the performance issues of the basic approach.  
However, this requires the client to maintain a local cache for storing its allocated range, which comes with various drawbacks:  
- Adds extra application-logic to the client side
- Can result in "holes" or "gaps" in the sequence if the client loses its cache for any reason (e.g. crash)  
  (Obviously the client could make sure to persist its cache to avoid this issue; but then you're juste adding even more client-side complexity.)

Overall, the performance boost is certainly worth the extra cost in complexity if the potential discontinuity of the sequence is not considered an issue at the application level.

Note that the simple *consensus protocols* approach is juste a special case of the *client-side caching* approach; where `range_size == 1`.

### The read-repair strategy

Let's define:

- N: the number of nodes in the cluster
- R: the number of nodes to read from when querying for a new ID
- W: the number of nodes that must acknowledge the value of the next generated ID

If `R + W > N`, then the read set and the write set *always* overlap, meaning that *at least one* of `R` results is the most recent ID in the cluster.  
Using a simple read-repair conflict resolution strategy, we always keep the highest ID from the read set; thus essentially implementing a master-less quorum.

This solution offers the same exact pros & cons as the *consensus protocols* approach; except there is no uneven workload distribution anymore.  
Indeed, every node in the cluster can now handle its fair share of the incoming traffic, since there is no elected leader.  
Of course, network IO and lock contention within the cluster will still damage overall performance; but you really don't have a choice as long as you need a shared state between your nodes.

**Further reading**:

- Werner Vogels' [famous post on consistency models](http://www.allthingsdistributed.com/2008/12/eventually_consistent.html) is certainly a must read when it comes to consistency in distributed systems.
- Riak's [replication properties](http://docs.basho.com/riak/kv/2.1.4/developing/app-guide/replication-properties/) is a great example of using {N,R,W} and conflict resolution strategies to adjust trade-offs between consistency and availability.

### The read-repair strategy + client-side caching

### The Flake model

Although it does not provide sequential IDs per se; the flake-ish way of doing things is such an elegant and performant solution that I *had* to mention it here.
