package nbs

import (
	"sync"

	"github.com/teh-cmc/seq"
)

// -----------------------------------------------------------------------------

// SimpleBufSeq implements a simple buffered `Sequencer` backed by a local,
// atomic, monotonically increasing 64bits value.
//
// A `SimpleBufSeq` is not particularly useful in and of itself; but it provides
// a good performance baseline that can be used as a point of comparison for
// more complex implementations.
type SimpleBufSeq struct {
	ids chan seq.ID

	stop chan struct{}
	wg   *sync.WaitGroup
}

// NewSimpleBufSeq returns a new `SimpleBufSeq` with the specified buffer size.
//
// `SimpleBufSeq` will constantly try to keep this buffer as full as possible
// using a dedicated background routine.
//
// `bufSize` will default to 0 in case it is < 0.
func NewSimpleBufSeq(bufSize int) *SimpleBufSeq {
	if bufSize < 0 {
		bufSize = 0
	}

	ids := make(chan seq.ID, bufSize)
	stop := make(chan struct{}, 0)
	id := seq.ID(1)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() { // background buffering routine
		defer wg.Done()
		for {
			select {
			case ids <- id: // blocks once `seq` is full
				id++
			case <-stop: // stream has been closed, kill routine
				return
			}
		}
	}()

	return &SimpleBufSeq{ids: ids, stop: stop, wg: wg}
}

// GetStream returns a pre-buffered, range-able stream of monotonically
// increasing IDs, starting at 1.
func (ss SimpleBufSeq) GetStream() seq.IDStream { return ss.ids }

// Close closes the associated `IDStream`.
// It always returns `nil`.
func (ss *SimpleBufSeq) Close() error {
	close(ss.stop) // kill background routine
	ss.wg.Wait()
	close(ss.ids) // close `ID` stream
	return nil
}
