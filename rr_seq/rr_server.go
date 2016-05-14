package rrs

import (
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"

	"github.com/teh-cmc/seq"

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

// lockedIDMap wraps a map of `lockedID` in a mutex.
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

	cp  *rrAPIPool
	ids *lockedIDMap
}

// NewRRServer returns a new `RRServer` that forms a cluster with the specified
// peers.
//
// It is *not* `RRServer`'s job to do gossiping, service discovery and/or
// cluster topology management.
// Please use the appropriate tools if you need such features.
// `RRServer` simply assumes that the list of peers you give it is exhaustive
// and correct. No more, no less.
//
// NOTE: you should always have an odd number of at least 3 nodes in your
// cluster to guarantee the availability of the system in case of a
// "half & (almost) half" netsplit situation.
//
// TODO: fsync support.
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

	cp, err := newRRAPIPool(peerAddrs...)
	//         ^^^^^^^^^^^^^^^^^^^^^^^^^^
	// will wait indefinitely until every specified peers are up & running
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

// Addr returns the current listening address.
func (s RRServer) Addr() net.Addr { return s.addr }

// Close stops the `RRServer` and its associated gRPC connections; and closes
// all the clients in the `rrAPIPool`.
//
// It returns the first error met.
func (s *RRServer) Close() error {
	s.Server.Stop()
	return s.cp.Close()
}

// -----------------------------------------------------------------------------

// NextID orchestrates with the cluster to return the next available range of `ID`s.
//
// NextID uses RW quorums an read-repair conflict-resolution strategies to
// maintain consistency within the RRCluster.
//
// The returned range of `ID`s is of the form [from:to).
//
// `rangeSize` will default to 1 in case it is < 1.
func (s *RRServer) NextID(name string, rangeSize int64) (seq.ID, seq.ID) {
	if rangeSize < 1 {
		rangeSize = 1
	}

	nbPeers, nbQuorum := 1+s.cp.Size(), (1+s.cp.Size())/2+1
	wg := &sync.WaitGroup{}

	// STEP I:
	// Fetch the current `ID` from `quorum` nodes (including ourself)
	// Find the highest `ID` within this set

	ids := make(chan seq.ID, nbPeers)
	ids <- s.getID(name)
	for i := 1; i < nbPeers; i++ {
		wg.Add(1)
		go s.getPeerID(s.cp.ClientRoundRobin(), name, ids, wg)
	}

	highestID := seq.ID(0)
	fetched := 1 // already fetched local current `ID`
	for id := range ids {
		if id > highestID {
			highestID = id
		}
		fetched++
		if fetched >= nbQuorum {
			break
		}
	}

	// TODO: cancel the (N-(N/2+1)) unnecessary requests
	wg.Wait()
	close(ids)

	// STEP II:
	// Sets the current `ID` to `highestID+rangeSize` on `quorum` nodes (including ourself)

	newID := highestID + seq.ID(rangeSize)
	successes := make(chan bool, nbPeers)
	successes <- s.setID(name, newID)
	for i := 1; i < nbPeers; i++ {
		wg.Add(1)
		go s.setPeerID(s.cp.ClientRoundRobin(), name, newID, successes, wg)
	}

	var fromID, toID seq.ID
	nbSuccesses := 0
	for s := range successes {
		if s {
			nbSuccesses++
		}
		if nbSuccesses >= nbQuorum {
			fromID, toID = highestID, newID
			break
		}
	}

	// TODO: cancel the (N-(N/2+1)) unnecessary requests
	wg.Wait()
	close(successes)

	return fromID, toID
}
func (s *RRServer) GRPCNextID(ctx context.Context, in *NextIDRequest) (*NextIDReply, error) {
	fromID, toID := s.NextID(in.Name, in.RangeSize)
	return &NextIDReply{FromId: uint64(fromID), ToId: uint64(toID)}, nil
}

// -----------------------------------------------------------------------------

// getID returns the current `ID` associated with the specified `name`, or `1` if
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
// name for the given peer; or `1` if no such `ID` exists.
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
		return err
	}
	ret <- seq.ID(idReply.CurId)
	return nil
}

// CurID returns the current `ID` associated with the specified name, or `1` if
// it doesn't exist.
func (s *RRServer) CurID(name string) seq.ID { return s.getID(name) }
func (s *RRServer) GRPCCurID(ctx context.Context, in *CurIDRequest) (*CurIDReply, error) {
	curID := uint64(s.CurID(in.Name))
	return &CurIDReply{CurId: curID}, nil
}

// -----------------------------------------------------------------------------

// setID sets the current `ID` to `id` iff curID < newID.
//
// It returns `true` on success; or `false` otherwise.
func (s *RRServer) setID(name string, id seq.ID) bool {
	s.ids.Lock()
	lockID, ok := s.ids.ids[name]
	if !ok { // no current `ID` with the given name => atomically build one
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

// setPeerID sets the specified `ID` on the given peer.
//
// It pushes `true` in `ret` in case of success; or `false` otherwise.
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

func (s *RRServer) SetID(name string, newID seq.ID) bool { return s.setID(name, newID) }
func (s *RRServer) GRPCSetID(ctx context.Context, in *SetIDRequest) (*SetIDReply, error) {
	success := s.SetID(in.Name, seq.ID(in.NewId))
	return &SetIDReply{Success: success}, nil
}
