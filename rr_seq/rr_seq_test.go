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

// TESTS: `go test -race -cpu 1,4,8 -run=. -bench=none -cover`
// BENCHMARKS: `go test -cpu 1,8,32 -run=none -bench=. -cover`

// -----------------------------------------------------------------------------

func TestRRSeq_New_BufSize(t *testing.T) {
	var s *RRSeq
	var err error

	name := fmt.Sprintf("TestRRSeq_New_BufSize(gomaxprocs:%d)", runtime.GOMAXPROCS(0))

	s, err = NewRRSeq(name, -42, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(s.ids), 0)
	_ = s.Close()

	s, err = NewRRSeq(name, 0, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(s.ids), 0)
	_ = s.Close()

	s, err = NewRRSeq(name, 1, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(s.ids), 1)
	_ = s.Close()

	s, err = NewRRSeq(name, 1e6, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, cap(s.ids), int(1e6))
	_ = s.Close()
}

func TestRRSeq_FirstID(t *testing.T) {
	name := fmt.Sprintf("TestRRSeq_FirstID(gomaxprocs:%d)", runtime.GOMAXPROCS(0))
	s, err := NewRRSeq(name, 1e2, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}
	ensure.DeepEqual(t, <-s.Stream(), seq.ID(1))
	_ = s.Close()
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

	for id := range s.Stream() {
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

func TestRRSeq_BufSize2_SingleClient(t *testing.T) {
	testRRSeq_SingleClient(2, t)
}

func TestRRSeq_BufSize1024_SingleClient(t *testing.T) {
	testRRSeq_SingleClient(1024, t)
}

// -----------------------------------------------------------------------------

func testRRSeq_MultiClients_Local(bufSize int, t *testing.T) {
	name := fmt.Sprintf(
		"testRRSeq_MultiClients_Local(bufsz:%d)(gomaxprocs:%d)", bufSize, runtime.GOMAXPROCS(0),
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

func TestRRSeq_BufSize0_MultiClients_Local(t *testing.T) {
	testRRSeq_MultiClients_Local(0, t)
}

func TestRRSeq_BufSize1_MultiClients_Local(t *testing.T) {
	testRRSeq_MultiClients_Local(1, t)
}

func TestRRSeq_BufSize2_MultiClients_Local(t *testing.T) {
	testRRSeq_MultiClients_Local(2, t)
}

func TestRRSeq_BufSize1024_MultiClients_Local(t *testing.T) {
	testRRSeq_MultiClients_Local(1024, t)
}

// -----------------------------------------------------------------------------

func testRRSeq_ConcurrentClients256_Local(bufSize int, t *testing.T) {
	name := fmt.Sprintf(
		"testRRSeq_ConcurrentClients256_Local(bufsz:%d)(gomaxprocs:%d)",
		bufSize, runtime.GOMAXPROCS(0),
	)
	s, err := NewRRSeq(name, bufSize, testingRRServerAddrs...)
	if err != nil {
		t.Fatal(err)
	}

	ids := make(seq.IDSlice, 0, 256*(bufSize+1)*10)
	idsLock := &sync.Mutex{}

	go func() {
		<-time.After(time.Millisecond * 500)
		_ = s.Close()
	}()

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

func TestRRSeq_BufSize0_ConcurrentClients256_Local(t *testing.T) {
	testRRSeq_ConcurrentClients256_Local(0, t)
}

func TestRRSeq_BufSize1_ConcurrentClients256_Local(t *testing.T) {
	testRRSeq_ConcurrentClients256_Local(1, t)
}

func TestRRSeq_BufSize2_ConcurrentClients256_Local(t *testing.T) {
	testRRSeq_ConcurrentClients256_Local(2, t)
}

func TestRRSeq_BufSize1024_ConcurrentClients256_Local(t *testing.T) {
	testRRSeq_ConcurrentClients256_Local(1024, t)
}

// -----------------------------------------------------------------------------

// This is essentially the single most important of the standard (i.e. non-mayhem)
// tests, as it checks that a cluster of `RRServer`s, being bombarded of NextID
// queries on its every nodes, still consistently deliver coherent, sequential `ID`s.
func testRRSeq_ConcurrentClients32_Distributed(bufSize int, t *testing.T) {
	name := fmt.Sprintf(
		"testRRSeq_ConcurrentClients32_Distributed(bufsz:%d)(gomaxprocs:%d)",
		bufSize, runtime.GOMAXPROCS(0),
	)

	ss := make([]*RRSeq, 0, 32)
	for i := 0; i < 32; i++ {
		s, err := NewRRSeq(name, bufSize, testingRRServerAddrs...)
		if err != nil {
			t.Fatal(err)
		}
		ss = append(ss, s)
	}

	ids := make(seq.IDSlice, 0, 32*(bufSize+1)*10)
	idsLock := &sync.Mutex{}

	go func() {
		<-time.After(time.Millisecond * 1500)
		for _, s := range ss {
			_ = s.Close()
		}
	}()

	wg := &sync.WaitGroup{}
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(ii int) {
			s := ss[ii]
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
		}(i)
	}
	wg.Wait()

	// check that the entire set of `ID`s returned contains no duplicates
	// -> gaps are normal and expected
	ids = ids.Sort()
	for i := 0; i < len(ids)-1; i++ {
		ensure.True(t, ids[i] < ids[i+1])
	}
}

func TestRRSeq_BufSize0_ConcurrentClients32_Distributed(t *testing.T) {
	testRRSeq_ConcurrentClients32_Distributed(0, t)
}

func TestRRSeq_BufSize1_ConcurrentClients32_Distributed(t *testing.T) {
	testRRSeq_ConcurrentClients32_Distributed(1, t)
}

func TestRRSeq_BufSize2_ConcurrentClients32_Distributed(t *testing.T) {
	testRRSeq_ConcurrentClients32_Distributed(2, t)
}

func TestRRSeq_BufSize1024_ConcurrentClients32_Distributed(t *testing.T) {
	testRRSeq_ConcurrentClients32_Distributed(1024, t)
}

// -----------------------------------------------------------------------------

func benchmarkRRSeq_SingleClient(bufSize int, b *testing.B) {
	name := fmt.Sprintf(
		"benchmarkRRSeq_SingleClient(bufsz:%d)(gomaxprocs:%d)", bufSize, runtime.GOMAXPROCS(0),
	)
	s, err := NewRRSeq(name, bufSize, testingRRServerAddrs...)
	if err != nil {
		b.Fatal(err)
	}
	ids := s.Stream()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ids.Next()
	}
	b.StopTimer()

	_ = s.Close()
}

func BenchmarkRRSeq_BufSize0_SingleClient(b *testing.B) {
	benchmarkRRSeq_SingleClient(0, b)
}

func BenchmarkRRSeq_BufSize1_SingleClient(b *testing.B) {
	benchmarkRRSeq_SingleClient(1, b)
}

func BenchmarkRRSeq_BufSize2_SingleClient(b *testing.B) {
	benchmarkRRSeq_SingleClient(2, b)
}

func BenchmarkRRSeq_BufSize1024_SingleClient(b *testing.B) {
	benchmarkRRSeq_SingleClient(1024, b)
}

// -----------------------------------------------------------------------------

func benchmarkRRSeq_ConcurrentClients_Local(bufSize int, b *testing.B) {
	name := fmt.Sprintf(
		"benchmarkRRSeq_ConcurrentClients_Local(bufsz:%d)(gomaxprocs:%d)",
		bufSize, runtime.GOMAXPROCS(0),
	)
	s, err := NewRRSeq(name, bufSize, testingRRServerAddrs...)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ids := s.Stream()
		for pb.Next() {
			_ = ids.Next()
		}
	})
	b.StopTimer()

	_ = s.Close()
}

func BenchmarkRRSeq_BufSize0_ConcurrentClients_Local(b *testing.B) {
	benchmarkRRSeq_ConcurrentClients_Local(0, b)
}

func BenchmarkRRSeq_BufSize1_ConcurrentClients_Local(b *testing.B) {
	benchmarkRRSeq_ConcurrentClients_Local(1, b)
}

func BenchmarkRRSeq_BufSize2_ConcurrentClients_Local(b *testing.B) {
	benchmarkRRSeq_ConcurrentClients_Local(2, b)
}

func BenchmarkRRSeq_BufSize1024_ConcurrentClients_Local(b *testing.B) {
	benchmarkRRSeq_ConcurrentClients_Local(1024, b)
}

// -----------------------------------------------------------------------------

func benchmarkRRSeq_ConcurrentClients_Distributed(bufSize int, b *testing.B) {
	name := fmt.Sprintf(
		"benchmarkRRSeq_ConcurrentClients_Distributed(bufsz:%d)(gomaxprocs:%d)",
		bufSize, runtime.GOMAXPROCS(0),
	)

	ss := make([]*RRSeq, 0, 32)
	for i := 0; i < 32; i++ {
		s, err := NewRRSeq(name, bufSize, testingRRServerAddrs...)
		if err != nil {
			b.Fatal(err)
		}
		ss = append(ss, s)
	}

	curSeq := 0
	curLock := &sync.Mutex{}
	nextRRSeq := func() *RRSeq {
		curLock.Lock()
		s := ss[curSeq]
		curSeq++
		if curSeq >= len(ss) {
			curSeq = 0
		}
		curLock.Unlock()
		return s
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ids := nextRRSeq().Stream()
		for pb.Next() {
			_ = ids.Next()
		}
	})
	b.StopTimer()

	for _, s := range ss {
		_ = s.Close()
	}
}

func BenchmarkRRSeq_BufSize0_ConcurrentClients_Distributed(b *testing.B) {
	benchmarkRRSeq_ConcurrentClients_Distributed(0, b)
}

func BenchmarkRRSeq_BufSize1_ConcurrentClients_Distributed(b *testing.B) {
	benchmarkRRSeq_ConcurrentClients_Distributed(1, b)
}

func BenchmarkRRSeq_BufSize2_ConcurrentClients_Distributed(b *testing.B) {
	benchmarkRRSeq_ConcurrentClients_Distributed(2, b)
}

func BenchmarkRRSeq_BufSize1024_ConcurrentClients_Distributed(b *testing.B) {
	benchmarkRRSeq_ConcurrentClients_Distributed(1024, b)
}

// -----------------------------------------------------------------------------

func ExampleRRSeq() {
	s, err := NewRRSeq("myseq", 2, testingRRServerAddrs...)
	if err != nil {
		panic(err)
	}

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
