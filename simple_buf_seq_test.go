package seq

import (
	"testing"
	"time"

	"github.com/facebookgo/ensure"
)

// -----------------------------------------------------------------------------

func TestSimpleBufSeq_New_BufSize(t *testing.T) {
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(-42)), 0)
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(0)), 0)
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(1)), 1)
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(1e6)), int(1e6))
}

func TestSimpleBufSeq_FirstID(t *testing.T) {
	ensure.DeepEqual(t, <-NewSimpleBufSeq(1e2).GetStream(), ID(1))
}

func TestSimpleBufSeq_SingleClient(t *testing.T) {
	seq := NewSimpleBufSeq(1024)
	lastID := ID(0)

	go func() {
		<-time.After(time.Millisecond * 250)
		_ = seq.Close()
	}()

	for id := range seq.GetStream() {
		ensure.DeepEqual(t, id, lastID+1)
		lastID = id
	}
}
