package rrs

import (
	"testing"

	"github.com/facebookgo/ensure"
	"github.com/teh-cmc/seq"
)

// -----------------------------------------------------------------------------

// NOTE: run these tests with `go test -race -cpu 1,8,32`

func TestRRSeq_FirstID(t *testing.T) {
	rrseq, err := NewRRSeq(testingRRSeqName, 1e2, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, <-rrseq.GetStream(), seq.ID(1))
}

func TestRRSeq_New_BufSize(t *testing.T) {
	var rrseq *RRSeq
	var err error

	rrseq, err = NewRRSeq(testingRRSeqName, -42, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(rrseq.ids), 0)

	rrseq, err = NewRRSeq(testingRRSeqName, 0, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(rrseq.ids), 0)

	rrseq, err = NewRRSeq(testingRRSeqName, 1, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(rrseq.ids), 1)

	rrseq, err = NewRRSeq(testingRRSeqName, 1e6, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(rrseq.ids), int(1e6))
}
