package rrs

import (
	"os"
	"strconv"
)

// -----------------------------------------------------------------------------

var testingRRServers []*RRServer
var testingRRServerAddrs []string

func init() {
	// use SEQ_SERVERS envar to dynamically specify number of test servers
	var nbServers int64 = 5
	if n, err := strconv.ParseInt(os.Getenv("SEQ_SERVERS"), 10, 64); err == nil {
		nbServers = n
	}

	testingRRServers = make([]*RRServer, nbServers)

	var err error
	for i := int64(0); i < nbServers; i++ {
		testingRRServers[i], err = NewRRServer(":0")
		if err != nil {
			panic(err)
		}
	}
	for i, s := range testingRRServers {
		for j := int64(0); j < nbServers; j++ {
			if j != int64(i) {
				if err := s.cp.Add(testingRRServers[j].Addr().String()); err != nil {
					panic(err)
				}
			}
		}
	}

	testingRRServerAddrs = make([]string, nbServers)
	for i := range testingRRServerAddrs {
		testingRRServerAddrs[i] = testingRRServers[i].Addr().String()
	}
}
