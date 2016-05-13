package seq

import (
	"testing"

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
