package nbs

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/ensure"
	"github.com/teh-cmc/seq"
)

// -----------------------------------------------------------------------------

// NOTE: run these tests with `go test -race -cpu 1,8,32`

func TestSimpleBufSeq_New_BufSize(t *testing.T) {
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(-42).ids), 0)
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(0).ids), 0)
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(1).ids), 1)
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(1e6).ids), int(1e6))
}

func TestSimpleBufSeq_FirstID(t *testing.T) {
	ensure.DeepEqual(t, <-NewSimpleBufSeq(1e2).GetStream(), seq.ID(1))
}

// -----------------------------------------------------------------------------

func testSimpleBufSeq_SingleClient(bufSize int, t *testing.T) {
	s := NewSimpleBufSeq(bufSize)
	lastID := seq.ID(0)

	go func() {
		<-time.After(time.Millisecond * 250)
		_ = s.Close()
	}()

	for id := range s.GetStream() {
		ensure.DeepEqual(t, id, lastID+1)
		lastID = id
	}
}

func TestSimpleBufSeq_BufSize0_SingleClient(t *testing.T) {
	testSimpleBufSeq_SingleClient(0, t)
}

func TestSimpleBufSeq_BufSize1_SingleClient(t *testing.T) {
	testSimpleBufSeq_SingleClient(1, t)
}

func TestSimpleBufSeq_BufSize1024_SingleClient(t *testing.T) {
	testSimpleBufSeq_SingleClient(1024, t)
}

// -----------------------------------------------------------------------------

func testSimpleBufSeq_MultiClient(bufSize int, t *testing.T) {
	s := NewSimpleBufSeq(bufSize)
	lastID := seq.ID(0)

	go func() {
		<-time.After(time.Millisecond * 250)
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

func TestSimpleBufSeq_BufSize0_MultiClient(t *testing.T) {
	testSimpleBufSeq_MultiClient(0, t)
}

func TestSimpleBufSeq_BufSize1_MultiClient(t *testing.T) {
	testSimpleBufSeq_MultiClient(1, t)
}

func TestSimpleBufSeq_BufSize1024_MultiClient(t *testing.T) {
	testSimpleBufSeq_MultiClient(1024, t)
}

// -----------------------------------------------------------------------------

func testSimpleBufSeq_ConcurrentClients256(bufSize int, t *testing.T) {
	seq := NewSimpleBufSeq(bufSize)

	go func() {
		<-time.After(time.Millisecond * 250)
		_ = seq.Close()
	}()

	wg := &sync.WaitGroup{}
	for i := 0; i < 256; i++ {
		wg.Add(1)
		go func() {
			for id := range seq.GetStream() {
				_ = id
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestSimpleBufSeq_BufSize0_ConcurrentClients256(t *testing.T) {
	testSimpleBufSeq_ConcurrentClients256(0, t)
}

func TestSimpleBufSeq_BufSize1_ConcurrentClients256(t *testing.T) {
	testSimpleBufSeq_ConcurrentClients256(1, t)
}

func TestSimpleBufSeq_BufSize1024_ConcurrentClients256(t *testing.T) {
	testSimpleBufSeq_ConcurrentClients256(1024, t)
}

// -----------------------------------------------------------------------------

// NOTE: run these benchmarks with `go test -run=none -bench=. -cpu 1,8,32`

func benchmarkSimpleBufSeq_SingleClient(bufSize int, b *testing.B) {
	s := NewSimpleBufSeq(bufSize).GetStream()
	for i := 0; i < b.N; i++ {
		_ = s.Next()
	}
}

func BenchmarkSimpleBufSeq_BufSize0_SingleClient(b *testing.B) {
	benchmarkSimpleBufSeq_SingleClient(0, b)
}

func BenchmarkSimpleBufSeq_BufSize1_SingleClient(b *testing.B) {
	benchmarkSimpleBufSeq_SingleClient(1, b)
}

func BenchmarkSimpleBufSeq_BufSize1024_SingleClient(b *testing.B) {
	benchmarkSimpleBufSeq_SingleClient(1024, b)
}

// -----------------------------------------------------------------------------

func benchmarkSimpleBufSeq_MultiClient(bufSize int, b *testing.B) {
	s := NewSimpleBufSeq(bufSize)
	b.RunParallel(func(pb *testing.PB) {
		ids := s.GetStream()
		for pb.Next() {
			_ = ids.Next()
		}
	})
}

func BenchmarkSimpleBufSeq_BufSize0_MultiClient(b *testing.B) {
	benchmarkSimpleBufSeq_MultiClient(0, b)
}

func BenchmarkSimpleBufSeq_BufSize1_MultiClient(b *testing.B) {
	benchmarkSimpleBufSeq_MultiClient(1, b)
}

func BenchmarkSimpleBufSeq_BufSize1024_MultiClient(b *testing.B) {
	benchmarkSimpleBufSeq_MultiClient(1024, b)
}

// -----------------------------------------------------------------------------

func ExampleSimpleBufSeq() {
	s := NewSimpleBufSeq(2)

	ids := make([]seq.ID, 0)
	for id := range s.GetStream() {
		ids = append(ids, id)
		if id == 10 { // won't stop until 12: 11 & 12 are already buffered
			_ = s.Close()
		}
	}
	fmt.Println(ids)

	// Output: [1 2 3 4 5 6 7 8 9 10 11 12]
}