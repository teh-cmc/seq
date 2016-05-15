package rrs

import (
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"

	"github.com/teh-cmc/seq"
	"github.com/teh-cmc/seq/rpc"

	"golang.org/x/net/context"
)

// -----------------------------------------------------------------------------

// lockedID wraps a `seq.ID` in a mutex.
//
// NOTE: this lock is used whenever an `ID` is read from or written to.
type lockedID struct {
	*sync.RWMutex
	id seq.ID
}

// lockedIDMap wraps a map of `lockedID`s in a mutex.
//
// NOTE: this lock is used whenever a `lockedID` has to be added or retrieved
// from the map.
type lockedIDMap struct {
	*sync.RWMutex
	ids map[string]*lockedID
}

// RRServer implements a single node of an RRCluster: a distributed system
// that guarantees sequential `ID` generation by using RW quorums and
// read-repair conflict-resolution strategies.
//
// You can find more information about the ideas behind such a system in the
// `README.md` file at the root of this repository.
type RRServer struct {
	*grpc.Server
	addr net.Addr

	cp  *rpc.Pool
	ids *lockedIDMap
}

// NewRRServer returns a new `RRServer` that forms a cluster with the specified
// peers.
//
// It is *not* `RRServer`'s job to do gossiping, service discovery and/or
// cluster topology management, et al.
// Please use the appropriate tools if you need such features.
// `RRServer` simply assumes that the list of peers you give it is exhaustive
// and correct. No more, no less.
//
// Set `addr` to "<host>:0" if you want to be assigned a port automatically,
// you can then retrieve the address of the server with `RRServer.Addr()`.
//
// NOTE: you should always have an odd number of at least 3 nodes in your
// cluster to guarantee the availability of the system in case of a
// "half & (almost) half" netsplit situation.
//
// TODO: fsync support.
// TODO: AddPeer support.
func NewRRServer(addr string, peerAddrs ...string) (*RRServer, error) {
	serv := &RRServer{}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	serv.Server = grpc.NewServer()
	RegisterRRAPIServer(serv.Server, serv)
	serv.addr = ln.Addr()
	go serv.Serve(ln)

	if len(peerAddrs) < 2 {
		log.Printf("warning: only %d peers specified\n", len(peerAddrs))
	}
	if len(peerAddrs)%2 == 1 {
		log.Printf("warning: uneven number of nodes in the cluster\n")
	}

	cp, err := rpc.NewPool(peerAddrs...) // blocks until all connections are established
	if err != nil {
		return nil, err
	}
	serv.cp = cp

	ids := &lockedIDMap{
		RWMutex: &sync.RWMutex{},
		ids:     make(map[string]*lockedID),
	}
	serv.ids = ids

	return serv, nil
}

// Addr returns the address of the listening socket.
func (s RRServer) Addr() net.Addr { return s.addr }

// Close stops the `RRServer` and its associated opened gRPC connections; and
// closes all connections with its peers (pool).
//
// It returns the first error met.
//
// Once closed, a `RRServer` is not reusable.
func (s *RRServer) Close() error {
	s.Server.Stop()
	return s.cp.Close()
}

// -----------------------------------------------------------------------------

// NextID orchestrates with the cluster to return the next available range of
// sequential `ID`s.
//
// NextID uses RW quorums and read-repair conflict-resolution strategies to
// maintain consistency within the cluster of `RRServer`s.
//
// The returned range of `ID`s is of the form [from:to).
// An empty range such as [0,0) indicates failure.
//
// `name` is the name of the sequence; a `RRServer` can hold as many sequences
// as you deem necessary.
// `rangeSize` will default to 1 in case it is < 1.
//
// TODO: server-side retries (note: can induce gaps)
func (s *RRServer) NextID(name string, rangeSize int64) (seq.ID, seq.ID) {
	if rangeSize < 1 {
		rangeSize = 1
	}

	nbQuorum := (1+s.cp.Size())/2 + 1
	peerConns := s.cp.Conns()        // get all healthy peer-connections from the pool
	if len(peerConns)+1 < nbQuorum { // cannot reach a quorum, might as well give up now
		return 0, 0
	}

	highestID := s.getHighestID(name, nbQuorum, peerConns)
	if highestID == 0 {
		return 0, 0
	}

	fromID, toID := s.setHighestID(name, highestID, rangeSize, nbQuorum, peerConns)
	if fromID == 0 || toID == 0 { // just emphasizing the fast that this can return [0,0)
		return 0, 0
	}

	return fromID, toID
}
func (s *RRServer) GRPCNextID(ctx context.Context, in *NextIDRequest) (*NextIDReply, error) {
	fromID, toID := s.NextID(in.Name, in.RangeSize)
	return &NextIDReply{FromId: uint64(fromID), ToId: uint64(toID)}, nil
}

// getHighestID returns the highest `ID` cluster-wide.
//
// It concurrently fetches the current `ID` from `nbQuorum` nodes (including ourself),
// then returns the highest `ID` within this set.
//
// getHighestID returns `ID(0)` on failures (e.g. no quorum reached).
func (s *RRServer) getHighestID(
	name string, nbQuorum int, peerConns []*grpc.ClientConn,
) seq.ID {
	wg := &sync.WaitGroup{}
	ids := make(chan seq.ID, len(peerConns)+1)
	ids <- s.getID(name)
	for _, pc := range peerConns {
		wg.Add(1)
		go s.getPeerID(NewRRAPIClient(pc), name, ids, wg)
	}

	highestID := seq.ID(1)
	nok, ok := 0, 1 // already successfully fetched local `ID`
	for id := range ids {
		if id > highestID {
			highestID = id
		}
		if id > seq.ID(0) {
			ok++
		} else {
			nok++
		}
		if ok >= nbQuorum { // got enough `ID`s
			break
		}
		if nok+ok >= len(peerConns)+1 { // too many failures
			highestID = 0
			break
		}
	}

	// TODO: cancel the unnecessary requests
	wg.Wait()
	close(ids)

	return highestID
}

// setHighestID sets the new highest `ID` cluster-wide and returns a new range
// accordingly.
//
// It concurrently sets the new `ID` (i.e. `highestID+rangeSize` on `nbQuorum` nodes
// (including ourself).
//
// setHighestID returns a [0,0) range on failures (e.g. no quorum reached).
func (s *RRServer) setHighestID(
	name string, highestID seq.ID, rangeSize int64,
	nbQuorum int, peerConns []*grpc.ClientConn,
) (seq.ID, seq.ID) {
	wg := &sync.WaitGroup{}
	newID := highestID + seq.ID(rangeSize)
	successes := make(chan bool, len(peerConns)+1)
	successes <- s.setID(name, newID)
	for _, pc := range peerConns {
		wg.Add(1)
		go s.setPeerID(NewRRAPIClient(pc), name, newID, successes, wg)
	}

	var fromID, toID seq.ID
	nok, ok := 0, 0
	for success := range successes {
		if success {
			ok++
		} else {
			nok++
		}
		if ok >= nbQuorum { // set enough `ID`s
			fromID, toID = highestID, newID
			break
		}
		if nok+ok >= len(peerConns)+1 { // too many failures
			break
		}
	}

	// TODO: cancel the unnecessary requests
	wg.Wait()
	close(successes)

	return fromID, toID
}

// -----------------------------------------------------------------------------

// getID returns the current `ID` associated with the specified `name`, or `ID(1)` if
// it doesn't exist.
func (s *RRServer) getID(name string) seq.ID {
	s.ids.RLock()
	lockedID, ok := s.ids.ids[name]
	s.ids.RUnlock()
	if !ok {
		return 1
	}

	lockedID.RLock()
	id := lockedID.id
	lockedID.RUnlock()

	return id
}

// getPeerID pushes in `ret` the current `ID` associated with the specified
// `name` for the given `peer`; or `ID(1)` if no such `ID` exists.
//
// It pushes `ID(0)` in `ret` on failures (e.g. network error).
func (s *RRServer) getPeerID(
	peer RRAPIClient,
	name string,
	ret chan<- seq.ID, wg *sync.WaitGroup,
) error {
	defer wg.Done()

	idReply, err := peer.GRPCCurID(context.TODO(), &CurIDRequest{Name: name})
	//                             ^^^^^^^^^^^^^^
	// TODO: handle cancellations & timeouts --^
	if err != nil {
		ret <- seq.ID(0)
		return err
	}
	ret <- seq.ID(idReply.CurId)
	return nil
}

// CurID returns the current `ID` associated with the specified name, or `ID(1)` if
// it doesn't exist.
//
// It returns `ID(0)` on failures.
func (s *RRServer) CurID(name string) seq.ID { return s.getID(name) }
func (s *RRServer) GRPCCurID(ctx context.Context, in *CurIDRequest) (*CurIDReply, error) {
	return &CurIDReply{CurId: uint64(s.CurID(in.Name))}, nil
}

// -----------------------------------------------------------------------------

// setID atomically sets the current `ID` to `id` iff curID < newID.
//
// It returns `true` on success; or `false` otherwise (e.g. curID >= newID).
func (s *RRServer) setID(name string, id seq.ID) bool {
	s.ids.Lock()
	lockID, ok := s.ids.ids[name]
	if !ok { // no sequence for the given name: atomically build a new one
		//      and add it to the map
		s.ids.ids[name] = &lockedID{RWMutex: &sync.RWMutex{}, id: id}
		s.ids.Unlock()
		return true
	}
	s.ids.Unlock()

	lockID.Lock()
	if lockID.id < id { // current `ID` found, check that it's < newID
		lockID.id = id
		lockID.Unlock()
		return true
	}
	lockID.Unlock()

	return false
}

// setPeerID atomically sets the current `ID` to `id` on the given `peer`,
// iff curID < newID.
//
// It pushes `true` in `ret` on successes; or `false` otherwise (e.g. curID >= newID).
func (s *RRServer) setPeerID(
	peer RRAPIClient,
	name string, id seq.ID,
	ret chan<- bool, wg *sync.WaitGroup,
) error {
	defer wg.Done()

	idReply, err := peer.GRPCSetID(
		context.TODO(), &SetIDRequest{Name: name, NewId: uint64(id)},
	//  ^^^^^^^^^^^^^^
	//        ^--- TODO: handle cancellations & timeouts
	)
	if err != nil {
		ret <- false
		return err
	}
	ret <- idReply.Success
	return nil
}

// SetID atomically sets the current `ID` to `id` iff curID < newID.
//
// It returns `true` on success; or `false` otherwise (e.g. curID >= newID).
func (s *RRServer) SetID(name string, newID seq.ID) bool { return s.setID(name, newID) }
func (s *RRServer) GRPCSetID(ctx context.Context, in *SetIDRequest) (*SetIDReply, error) {
	success := s.SetID(in.Name, seq.ID(in.NewId))
	return &SetIDReply{Success: success}, nil
}
