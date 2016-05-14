package rrs

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/ensure"
	"github.com/teh-cmc/seq"
)

// -----------------------------------------------------------------------------

// NOTE: run these tests with `go test -race -cpu 1,4,8`

func TestRRSeq_New_BufSize(t *testing.T) {
	var rrseq *RRSeq
	var err error

	name := fmt.Sprintf("TestRRSeq_New_BufSize(gomaxprocs:%d)", runtime.GOMAXPROCS(0))

	rrseq, err = NewRRSeq(name, -42, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(rrseq.ids), 0)

	rrseq, err = NewRRSeq(name, 0, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(rrseq.ids), 0)

	rrseq, err = NewRRSeq(name, 1, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(rrseq.ids), 1)

	rrseq, err = NewRRSeq(name, 1e6, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(rrseq.ids), int(1e6))
}

func TestRRSeq_FirstID(t *testing.T) {
	name := fmt.Sprintf("TestRRSeq_FirstID(gomaxprocs:%d)", runtime.GOMAXPROCS(0))
	rrseq, err := NewRRSeq(name, 1e2, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, <-rrseq.GetStream(), seq.ID(1))
}

// -----------------------------------------------------------------------------

func testRRSeq_SingleClient(bufSize int, t *testing.T) {
	name := fmt.Sprintf(
		"testRRSeq_SingleClient(bufsz:%d)(gomaxprocs:%d)", bufSize, runtime.GOMAXPROCS(0),
	)
	s, err := NewRRSeq(name, bufSize, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	lastID := seq.ID(0)

	go func() {
		<-time.After(time.Millisecond * 500)
		_ = s.Close()
	}()

	for id := range s.GetStream() {
		ensure.DeepEqual(t, id, lastID+1)
		lastID = id
	}
}

func TestRRSeq_BufSize0_SingleClient(t *testing.T) {
	testRRSeq_SingleClient(0, t)
}

func TestRRSeq_BufSize1_SingleClient(t *testing.T) {
	testRRSeq_SingleClient(1, t)
}

func TestRRSeq_BufSize1024_SingleClient(t *testing.T) {
	testRRSeq_SingleClient(1024, t)
}

// -----------------------------------------------------------------------------

func testRRSeq_MultiClient_Local(bufSize int, t *testing.T) {
	name := fmt.Sprintf(
		"testRRSeq_MultiClient_Local(bufsz:%d)(gomaxprocs:%d)", bufSize, runtime.GOMAXPROCS(0),
	)
	s, err := NewRRSeq(name, bufSize, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	lastID := seq.ID(0)

	go func() {
		<-time.After(time.Millisecond * 500)
		_ = s.Close()
	}()

	s1, s2, s3 := s.GetStream(), s.GetStream(), s.GetStream()
	for {
		id1 := s1.Next()
		if id1 == 0 {
			break
		}
		ensure.DeepEqual(t, id1, lastID+1)
		lastID++
		id2 := s2.Next()
		if id2 == 0 {
			break
		}
		ensure.DeepEqual(t, id2, id1+1)
		lastID++
		id3 := s3.Next()
		if id3 == 0 {
			break
		}
		ensure.DeepEqual(t, id3, id2+1)
		lastID++
	}
}

func TestRRSeq_BufSize0_MultiClient_Local(t *testing.T) {
	testRRSeq_MultiClient_Local(0, t)
}

func TestRRSeq_BufSize1_MultiClient_Local(t *testing.T) {
	testRRSeq_MultiClient_Local(1, t)
}

func TestRRSeq_BufSize1024_MultiClient_Local(t *testing.T) {
	testRRSeq_MultiClient_Local(1024, t)
}

// -----------------------------------------------------------------------------

func testRRSeq_ConcurrentClients256_Local(bufSize int, t *testing.T) {
	name := fmt.Sprintf(
		"testRRSeq_ConcurrentClients256_Local(bufsz:%d)(gomaxprocs:%d)", bufSize, runtime.GOMAXPROCS(0),
	)
	s, err := NewRRSeq(name, bufSize, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		<-time.After(time.Millisecond * 500)
		_ = s.Close()
	}()

	wg := &sync.WaitGroup{}
	for i := 0; i < 256; i++ {
		wg.Add(1)
		go func() {
			for id := range s.GetStream() {
				_ = id
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestRRSeq_BufSize0_ConcurrentClients256_Local(t *testing.T) {
	testRRSeq_ConcurrentClients256_Local(0, t)
}

func TestRRSeq_BufSize1_ConcurrentClients256_Local(t *testing.T) {
	testRRSeq_ConcurrentClients256_Local(1, t)
}

func TestRRSeq_BufSize1024_ConcurrentClients256_Local(t *testing.T) {
	testRRSeq_ConcurrentClients256_Local(1024, t)
}

// -----------------------------------------------------------------------------

// NOTE: run these benchmarks with `go test -run=none -bench=. -cpu 1,8,32`

func benchmarkRRSeq_SingleClient(bufSize int, b *testing.B) {
	name := fmt.Sprintf(
		"benchmarkRRSeq_SingleClient(bufsz:%d)(gomaxprocs:%d)", bufSize, runtime.GOMAXPROCS(0),
	)
	s, err := NewRRSeq(name, bufSize, testingRRServerAddrs...)
	if err != nil {
		b.Fatal(err)
	}
	ids := s.GetStream()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ids.Next()
	}
}

func BenchmarkRRSeq_BufSize0_SingleClient(b *testing.B) {
	benchmarkRRSeq_SingleClient(0, b)
}

func BenchmarkRRSeq_BufSize1_SingleClient(b *testing.B) {
	benchmarkRRSeq_SingleClient(1, b)
}

func BenchmarkRRSeq_BufSize1024_SingleClient(b *testing.B) {
	benchmarkRRSeq_SingleClient(1024, b)
}

// -----------------------------------------------------------------------------

func benchmarkRRSeq_MultiClient_Local(bufSize int, b *testing.B) {
	name := fmt.Sprintf(
		"benchmarkRRSeq_MultiClient_Local(bufsz:%d)(gomaxprocs:%d)", bufSize, runtime.GOMAXPROCS(0),
	)
	s, err := NewRRSeq(name, bufSize, testingRRServerAddrs...)
	if err != nil {
		b.Fatal(err)
	}
	b.RunParallel(func(pb *testing.PB) {
		ids := s.GetStream()
		for pb.Next() {
			_ = ids.Next()
		}
	})
}

func BenchmarkRRSeq_BufSize0_MultiClient_Local(b *testing.B) {
	benchmarkRRSeq_MultiClient_Local(0, b)
}

func BenchmarkRRSeq_BufSize1_MultiClient_Local(b *testing.B) {
	benchmarkRRSeq_MultiClient_Local(1, b)
}

func BenchmarkRRSeq_BufSize1024_MultiClient_Local(b *testing.B) {
	benchmarkRRSeq_MultiClient_Local(1024, b)
}
