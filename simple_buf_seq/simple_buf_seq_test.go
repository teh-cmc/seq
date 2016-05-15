// Copyright © 2015 Clement 'cmc' Rey <cr.rey.clement@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package sbs

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/ensure"
	"github.com/teh-cmc/seq"
)

// TESTS: `go test -race -cpu 1,4,8 -run=. -bench=none -cover`
// BENCHMARKS: `go test -cpu 1,8,32 -run=none -bench=. -cover`

// -----------------------------------------------------------------------------

func TestSimpleBufSeq_New_BufSize(t *testing.T) {
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(-42).ids), 0)
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(0).ids), 0)
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(1).ids), 1)
	ensure.DeepEqual(t, cap(NewSimpleBufSeq(1e6).ids), int(1e6))
}

func TestSimpleBufSeq_FirstID(t *testing.T) {
	ensure.DeepEqual(t, <-NewSimpleBufSeq(1e2).Stream(), seq.ID(1))
}

// -----------------------------------------------------------------------------

func testSimpleBufSeq_SingleClient(bufSize int, t *testing.T) {
	s := NewSimpleBufSeq(bufSize)
	lastID := seq.ID(0)

	go func() {
		<-time.After(time.Millisecond * 250)
		_ = s.Close()
	}()

	for id := range s.Stream() {
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

func testSimpleBufSeq_MultiClients(bufSize int, t *testing.T) {
	s := NewSimpleBufSeq(bufSize)
	lastID := seq.ID(0)

	go func() {
		<-time.After(time.Millisecond * 250)
		_ = s.Close()
	}()

	s1, s2, s3 := s.Stream(), s.Stream(), s.Stream()
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

func TestSimpleBufSeq_BufSize0_MultiClients(t *testing.T) {
	testSimpleBufSeq_MultiClients(0, t)
}

func TestSimpleBufSeq_BufSize1_MultiClients(t *testing.T) {
	testSimpleBufSeq_MultiClients(1, t)
}

func TestSimpleBufSeq_BufSize1024_MultiClients(t *testing.T) {
	testSimpleBufSeq_MultiClients(1024, t)
}

// -----------------------------------------------------------------------------

func testSimpleBufSeq_ConcurrentClients256(bufSize int, t *testing.T) {
	s := NewSimpleBufSeq(bufSize)

	go func() {
		<-time.After(time.Millisecond * 500)
		_ = s.Close()
	}()

	ids := make(seq.IDSlice, 0, 256*(bufSize+1)*10)
	idsLock := &sync.Mutex{}

	wg := &sync.WaitGroup{}
	for i := 0; i < 256; i++ {
		wg.Add(1)
		go func() {
			allIDs := make(seq.IDSlice, 0, (bufSize+1)*10)
			lastID := seq.ID(0)
			for id := range s.Stream() {
				ensure.True(t, id > lastID)
				lastID = id
				allIDs = append(allIDs, id)
			}

			idsLock.Lock()
			ids = append(ids, allIDs...)
			idsLock.Unlock()

			wg.Done()
		}()
	}
	wg.Wait()

	// check that the entire set of `ID`s returned is sequential and
	// monotonically increasing as a whole
	ids = ids.Sort()
	for i := 0; i < len(ids)-1; i++ {
		ensure.True(t, ids[i]+1 == ids[i+1])
	}
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

func benchmarkSimpleBufSeq_SingleClient(bufSize int, b *testing.B) {
	s := NewSimpleBufSeq(bufSize).Stream()
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

func benchmarkSimpleBufSeq_ConcurrentClients(bufSize int, b *testing.B) {
	s := NewSimpleBufSeq(bufSize)
	b.RunParallel(func(pb *testing.PB) {
		ids := s.Stream()
		for pb.Next() {
			_ = ids.Next()
		}
	})
}

func BenchmarkSimpleBufSeq_BufSize0_ConcurrentClients(b *testing.B) {
	benchmarkSimpleBufSeq_ConcurrentClients(0, b)
}

func BenchmarkSimpleBufSeq_BufSize1_ConcurrentClients(b *testing.B) {
	benchmarkSimpleBufSeq_ConcurrentClients(1, b)
}

func BenchmarkSimpleBufSeq_BufSize2_ConcurrentClients(b *testing.B) {
	benchmarkSimpleBufSeq_ConcurrentClients(2, b)
}

func BenchmarkSimpleBufSeq_BufSize1024_ConcurrentClients(b *testing.B) {
	benchmarkSimpleBufSeq_ConcurrentClients(1024, b)
}

// -----------------------------------------------------------------------------

func ExampleSimpleBufSeq() {
	s := NewSimpleBufSeq(2)

	ids := make([]seq.ID, 0)
	for id := range s.Stream() {
		ids = append(ids, id)
		if id == 10 {
			_ = s.Close()
			break
		}
	}
	fmt.Println(ids)

	// Output: [1 2 3 4 5 6 7 8 9 10]
}
