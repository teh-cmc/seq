package seq

import (
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/ensure"
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

func TestSimpleBufSeq_MultiClient(t *testing.T) {
	seq := NewSimpleBufSeq(1024)
	lastID := ID(0)

	go func() {
		<-time.After(time.Millisecond * 250)
		_ = seq.Close()
	}()

	s1, s2, s3 := seq.GetStream(), seq.GetStream(), seq.GetStream()
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

func TestSimpleBufSeq_ConcurrentClients256(t *testing.T) {
	seq := NewSimpleBufSeq(1024)

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

// -----------------------------------------------------------------------------

// NOTE: run these benchmarks with `go test -bench=. -cpu 1,8,32`

func BenchmarkSimpleBufSeq_BufSize1024_SingleClient(b *testing.B) {
	s := NewSimpleBufSeq(1024).GetStream()
	for i := 0; i < b.N; i++ {
		_ = s.Next()
	}
}
