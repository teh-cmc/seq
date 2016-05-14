package rrs

import (
	"testing"

	"github.com/facebookgo/ensure"
)

// -----------------------------------------------------------------------------

const (
	testingRRSeqName = "idseq"

	testingRRServer1Addr = ":16001"
	testingRRServer2Addr = ":16002"
	testingRRServer3Addr = ":16003"
)

var testingRRServerAddrs = []string{
	testingRRServer1Addr,
	testingRRServer2Addr,
	testingRRServer3Addr,
}

var (
	testingRRServer1 *RRServer
	testingRRServer2 *RRServer
	testingRRServer3 *RRServer
)

func init() {
	var err error

	go func() {
		testingRRServer1, err = NewRRServer(testingRRServer1Addr,
			testingRRServer2Addr,
			testingRRServer3Addr,
		)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		testingRRServer2, err = NewRRServer(testingRRServer2Addr,
			testingRRServer1Addr,
			testingRRServer3Addr,
		)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		testingRRServer3, err = NewRRServer(testingRRServer3Addr,
			testingRRServer1Addr,
			testingRRServer2Addr,
		)
		if err != nil {
			panic(err)
		}
	}()
}

// -----------------------------------------------------------------------------

// NOTE: run these tests with `go test -race -cpu 1,8,32`

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
