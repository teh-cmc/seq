package seq

// -----------------------------------------------------------------------------

// SimpleBufSeq implements a simple buffered `Sequencer` backed by a local,
// atomic, monotonically increasing 64bits value.
//
// A `SimpleBufSeq` is not particularly useful in and of itself; but it provides
// a good performance baseline that can be used as a point of comparison for
// more complex implementations.
type SimpleBufSeq chan ID

// NewSimpleBufSeq returns a new `SimpleBufSeq` with the specified buffer size.
//
// `SimpleBufSeq` will constantly try to keep this buffer as full as possible
// using a dedicated background routine.
//
// `bufSize` will default to 0 in case it is < 0.
func NewSimpleBufSeq(bufSize int) SimpleBufSeq {
	if bufSize < 0 {
		bufSize = 0
	}

	seq := make(SimpleBufSeq, bufSize)
	id := ID(1)

	go func() { // background buffering routine
		for {
			select {
			case seq <- id: // blocks once `seq` is full
				id++
			case <-seq: // stream has been closed, kill routine
				return
			}
		}
	}()

	return seq
}

// GetStream returns a pre-buffered, range-able stream of monotonically
// increasing IDs, starting at 1.
func (ss SimpleBufSeq) GetStream() IDStream { return IDStream(chan ID(ss)) }

// Close closes the associated `IDStream`.
// It always returns `nil`.
func (ss SimpleBufSeq) Close() error { close(ss); return nil }
