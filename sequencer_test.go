package seq

import (
	"testing"

	"github.com/facebookgo/ensure"
)

// -----------------------------------------------------------------------------

func TestIDStream_Next(t *testing.T) {
	ids := make(chan ID, 1)

	select {
	case <-ids:
		t.Fatal("should not be here")
	default:
	}

	ids <- ID(42)
	ensure.DeepEqual(t, IDStream(ids).Next(), ID(42))

	close(ids)
	ensure.DeepEqual(t, IDStream(ids).Next(), ID(0))
}
