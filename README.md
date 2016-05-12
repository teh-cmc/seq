# Seq

This document gives a gentle overview of the possible design solutions to the common problem of generating sequential / monotonically increasing IDs in a distributed system.  
Specifically, it focuses on maximizing performances and guaranteeing a fair distribution of the workload between nodes, as the size of the cluster increases.

## Possible designs

### Consensus protocols

Perhaps the most obvious and straightforward way of solving this problem is to implement a distributed locking mechanism upon a consensus protocol such as [Raft](https://raft.github.io/).

In fact, several tools such as [Consul](https://www.consul.io/) or [ZooKeeper](https://zookeeper.apache.org/) already exist out there, and provide all the necessary abstractions for emulating atomic integers across the network; out of the box.

Using these capabilities, it is quite straightforward to expose a `get-and-incr` atomic endpoint for clients to query.

**pros**:

- Strong consistency & sequentiality guarantees  
  Using a quorum, the system can A) guarantee the sequentiality of the IDs returned over time, and B) assure that there is no "holes" in the sequence.
- Good fault-tolerancy guarantees  
  The system can and will stay available as long as 2N+1 nodes are still available.

**cons**:

- Poor performance  
  Since every operation requires communication between nodes, most of the time is spent in costly network IO.
- Uneven workload distribution  
  Due to the nature of the Leader/Follower model; a single node, the leader, is charged of handling all of the incoming traffic (e.g. serialization/deserialization of RPC requests).

### Consensus protocols + client-side caching

A simple enhancement to the 
