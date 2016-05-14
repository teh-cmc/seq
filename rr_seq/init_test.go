package rrs

import "sync"

// -----------------------------------------------------------------------------

const (
	testingRRSeqName = "idseq"

	testingRRServer1Addr = ":16001"
	testingRRServer2Addr = ":16002"
	testingRRServer3Addr = ":16003"
	testingRRServer4Addr = ":16004"
	testingRRServer5Addr = ":16005"
)

var (
	testingRRServerAddrs = []string{
		testingRRServer1Addr,
		testingRRServer2Addr,
		testingRRServer3Addr,
		testingRRServer4Addr,
		testingRRServer5Addr,
	}

	testingRRServer1 *RRServer
	testingRRServer2 *RRServer
	testingRRServer3 *RRServer
	testingRRServer4 *RRServer
	testingRRServer5 *RRServer
)

func init() {
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		testingRRServer1, err = NewRRServer(testingRRServer1Addr,
			testingRRServer2Addr,
			testingRRServer3Addr,
			testingRRServer4Addr,
			testingRRServer5Addr,
		)
		if err != nil {
			panic(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		testingRRServer2, err = NewRRServer(testingRRServer2Addr,
			testingRRServer1Addr,
			testingRRServer3Addr,
			testingRRServer4Addr,
			testingRRServer5Addr,
		)
		if err != nil {
			panic(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		testingRRServer3, err = NewRRServer(testingRRServer3Addr,
			testingRRServer1Addr,
			testingRRServer2Addr,
			testingRRServer4Addr,
			testingRRServer5Addr,
		)
		if err != nil {
			panic(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		testingRRServer4, err = NewRRServer(testingRRServer4Addr,
			testingRRServer1Addr,
			testingRRServer2Addr,
			testingRRServer3Addr,
			testingRRServer5Addr,
		)
		if err != nil {
			panic(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		testingRRServer5, err = NewRRServer(testingRRServer5Addr,
			testingRRServer1Addr,
			testingRRServer2Addr,
			testingRRServer3Addr,
			testingRRServer4Addr,
		)
		if err != nil {
			panic(err)
		}
	}()

	wg.Wait()
}
