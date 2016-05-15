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
// a performance baseline that can later be used as a point of comparison
// for more complex implementations.
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
			case <-stop: // stream has been closed, kill routine
				return
			default:
				select {
				case ids <- id: // feed as much `ID`s as the `bufSize` allows
					id++
				case <-stop: // stream has been closed, kill routine
					return
				}
			}

		}
	}()

	return &SimpleBufSeq{ids: ids, stop: stop, wg: wg}
}

// Stream returns a pre-buffered, range-able stream of monotonically
// increasing IDs backed by a local atomic 64bits value, starting at `ID(1)`.
func (ss SimpleBufSeq) Stream() seq.IDStream { return ss.ids }

// Close closes the associated `IDStream` and stops the background buffering
// routine.
//
// Close always returns `nil`.
//
// Once closed, a `SimpleBufSeq` is not reusable.
func (ss *SimpleBufSeq) Close() error {
	close(ss.stop) // kill background buffering routine..
	ss.wg.Wait()   // ..and wait for it to be fully stopped
	close(ss.ids)  // close `ID` stream
	return nil     // always return `nil`
}
