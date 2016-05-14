package rrs

import (
	"sync"

	"golang.org/x/net/context"

	"github.com/teh-cmc/seq"
)

// -----------------------------------------------------------------------------

// RRSeq implements a buffered `Sequencer` backed by a cluster of `RRServer`s: a
// distributed system that guarantees sequential `ID` generation by using RW
// quorums and read-repair conflict resolution strategies.
//
// You can find more information about the ideas behind such a system in the
// `README.md` file at the root of this repository.
type RRSeq struct {
	cp  *rrAPIPool
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

	cp, err := newRRAPIPool(addrs...)
	if err != nil {
		return nil, err
	}

	ids := make(chan seq.ID, bufSize)
	stop := make(chan struct{}, 0)
	wg := &sync.WaitGroup{}
	rrseq := &RRSeq{cp: cp, ids: ids, stop: stop, wg: wg}

	var curRange, nextRange [2]seq.ID
	wg.Add(1)
	go func() { // background buffering routine
		defer wg.Done()
		switched := false
		for {

			// if the current range has already been half (or more) consumed,
			// fetch and store the next available range from the cluster
			if curRange[0] >= (curRange[0]+curRange[1])/2 && !switched {
				nextRange[0], nextRange[1] = rrseq.getNextRange(name, bufSize)
				switched = true
			}
			// if the current range has been entirely consumed,
			// replace the current range by the already stored next range
			if curRange[0] >= curRange[1] {
				curRange = nextRange
				switched = false
			}

			select {
			case <-stop: // stream has been closed, kill routine
				return
			default:
				select {
				case ids <- curRange[0]: // blocks once `seq` is full
					curRange[0]++
				case <-stop: // stream has been closed, kill routine
					return
				}
			}

		}
	}()

	return rrseq, nil
}

// -----------------------------------------------------------------------------

// getNextRange fetches the next available range of `ID`s from the cluster
// of `RRServer`s.
func (ss RRSeq) getNextRange(name string, rangeSize int) (seq.ID, seq.ID) {
	idReply, err := ss.cp.Client().GRPCNextID(
		//          ^^^^^^^^^^^^^^
		//                 ^--- transparent round-robin
		context.TODO(), // TODO: handle timeouts
		&NextIDRequest{Name: name, RangeSize: int64(rangeSize)},
	)
	if err != nil {
		// NOTE: if things go wrong, an empty `ID` range is returned
		// this will be retried on the next iteration, using the next client (round-robin)
		return 0, 0
	}
	return seq.ID(idReply.FromId), seq.ID(idReply.ToId)
}

// GetStream returns a pre-buffered, range-able stream of sequential `ID`s,
// starting at `1`.
func (ss RRSeq) GetStream() seq.IDStream { return ss.ids }

// Close closes the associated `IDStream`.
//
// It returns the first error met.
func (ss *RRSeq) Close() error {
	close(ss.stop)       // kill background routine
	ss.wg.Wait()         // wait for it to be fully stopped
	close(ss.ids)        // close `ID` stream
	return ss.cp.Close() // close gRPC clients
}
