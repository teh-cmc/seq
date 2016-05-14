package rrs

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
