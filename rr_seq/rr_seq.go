// Copyright © 2015 Clement 'cmc' Rey <cr.rey.clement@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package rrs

import (
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/teh-cmc/seq"
	"github.com/teh-cmc/seq/rpc"
	"github.com/teh-cmc/seq/rr_seq/pb"
)

// -----------------------------------------------------------------------------

// RRSeq implements a buffered `Sequencer` backed by a cluster of `RRServer`s: a
// distributed system that guarantees sequential `ID` generation by using RW
// quorums and read-repair conflict resolution strategies.
//
// You can find more information about the ideas behind such a system in the
// `README.md` file at the root of this repository.
type RRSeq struct {
	cp  *rpc.Pool
	ids chan seq.ID

	stop chan struct{}
	wg   *sync.WaitGroup
}

// NewRRSeq returns a new `RRSeq` with the specified buffer size.
//
// `RRSeq` will constantly try to keep this buffer as full as possible
// using a dedicated background routine.
//
// `bufSize` will default to 0 in case it is < 0.
func NewRRSeq(name string, bufSize int, addrs ...string) (*RRSeq, error) {
	if bufSize < 0 {
		bufSize = 0
	}

	cp, err := rpc.NewPool(addrs...) // blocks until all connections are established
	if err != nil {
		return nil, err
	}

	ids := make(chan seq.ID, bufSize)
	stop := make(chan struct{}, 0)
	wg := &sync.WaitGroup{}
	rrseq := &RRSeq{cp: cp, ids: ids, stop: stop, wg: wg}

	var curRange, nextRange [2]seq.ID // empty ranges
	wg.Add(1)
	go func() { // background buffering routine
		defer wg.Done()
		switched := false
		for {

			// if the current range has already been half (or more) consumed,
			// or if it is the empty range [0,0) (e.g. due to a previous server error),
			// fetch and store the next available range from the cluster
			if curRange[0] >= (curRange[0]+curRange[1])/2 && !switched {
				nextRange[0], nextRange[1] = rrseq.getNextRange(name, bufSize)
				switched = true
			}
			// if the current range has been entirely consumed,
			// or if it is the empty range [0,0) (e.g. due to a previous server error),
			// replace the current range by the already stored next range
			if curRange[0] >= curRange[1] {
				// NOTE: it is possible for `nextRange` to be [0,0) here,
				// this happens when the server is not able to return a new range
				// for whatever reason
				curRange = nextRange
				switched = false
			}

			select {
			case <-stop: // stream has been closed, kill routine
				return
			default:
				// make sure that `curRange` is not an empty range such as [0,0),
				// otherwise, just continue to the next iteration
				if curRange[0] < curRange[1] {
					select {
					case ids <- curRange[0]: // blocks once `seq` is full
						curRange[0]++
					case <-stop: // stream has been closed, kill routine
						return
					}
				}
			}

		}
	}()

	return rrseq, nil
}

// -----------------------------------------------------------------------------

// getNextRange fetches the next available range of `ID`s from the cluster
// of `RRServer`s.
//
// getNextRange returns the empty [0;0) range on failures; which means that the
// buffering routine will be retrying soon, using another peer (because round-robin).
//
// A deadline is set so that queries and reconnection attemps cannot last
// forever.
// For queries, this deadline is propagated through the various calls made
// within the cluster of `RRServer`s. This means that the time spent in
// intra-cluster RPC calls is included in this deadline.
//
// An exceeded deadline behaves as an error, and as such it returns the empty
// [0;0) range.
//
// NOTE: the deadline is hardcoded for now.
func (ss RRSeq) getNextRange(name string, rangeSize int) (seq.ID, seq.ID) {
	conn := ss.cp.ConnRoundRobin()
	if conn == nil { // no healthy connection available
		return 0, 0 // return empty range, this will toggle the retry machinery
	}

	deadline := time.Second * 4 // TODO(low-prio): make this configurable
	ctx, canceller := context.WithTimeout(context.Background(), deadline)

	idReply, err := pb.NewRRAPIClient(conn).GRPCNextID(
		ctx, // request will be cancelled if it lasts more than `deadline`,
		//      or if we've been trying to reconnect for more than `deadline`;
		//      in any case, this results in `err != nil`
		&pb.NextIDRequest{Name: name, RangeSize: int64(rangeSize)},
	)
	canceller() // we got what we came for, stop everything downstream
	if err != nil {
		return 0, 0 // return empty range, this will toggle the retry machinery
	}

	return seq.ID(idReply.FromId), seq.ID(idReply.ToId)
}

// Stream returns a pre-buffered, range-able stream of sequential `ID`s
// backed by a cluster of `RRServer`s, starting at `ID(1)`.
func (ss RRSeq) Stream() seq.IDStream { return ss.ids }

// Close closes the associated `IDStream` and stops the background buffering
// routine.
//
// It returns the first error met.
//
// Once closed, a `SimpleBufSeq` is not reusable.
func (ss *RRSeq) Close() error {
	close(ss.stop)       // kill background buffering routine..
	ss.wg.Wait()         // ..and wait for it to be fully stopped
	close(ss.ids)        // close `ID` stream
	return ss.cp.Close() // close gRPC connections
}
