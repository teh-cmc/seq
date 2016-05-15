package rrs

import (
	"io/ioutil"
	"os"
	"strconv"
)

// ENV_VARS:
//
// SEQ_SERVERS=<int> -> specifies the number N of nodes in the cluster (default: 5)
// SEQ_FSYNC={0,1}   -> enables or disables disk persistence (default: disabled)

// -----------------------------------------------------------------------------

var testingRRServers []*RRServer
var testingRRServerAddrs []string

func init() {
	var nbServers int64 = 5
	if n, err := strconv.ParseInt(os.Getenv("SEQ_SERVERS"), 10, 64); err == nil {
		nbServers = n
	}

	paths := make([]string, nbServers)
	fsync := false
	if os.Getenv("SEQ_FSYNC") == "1" {
		fsync = true
	}
	if fsync {
		for i := range paths {
			f, err := ioutil.TempFile("", "seq_tests")
			if err != nil {
				panic(err)
			}
			paths[i] = f.Name()
			_ = f.Close()
		}
	}

	testingRRServers = make([]*RRServer, nbServers)

	var err error
	var ppath string
	for i := int64(0); i < nbServers; i++ {
		if len(paths) > 0 {
			ppath = paths[i]
		}
		testingRRServers[i], err = NewRRServer(":0", ppath) // warning "0 peer" expected
		if err != nil {
			panic(err)
		}
	}
	for i, s := range testingRRServers {
		for j := int64(0); j < nbServers; j++ {
			if j != int64(i) {
				// /!\ dynamically adding peers directly to the pool
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
