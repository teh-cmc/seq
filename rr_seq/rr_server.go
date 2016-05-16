// Copyright © 2015 Clement 'cmc' Rey <cr.rey.clement@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package rrs

import (
	"encoding/gob"
	"log"
	"net"
	"os"
	"sync"

	"google.golang.org/grpc"

	"github.com/teh-cmc/seq"
	"github.com/teh-cmc/seq/rpc"
	"github.com/teh-cmc/seq/rr_seq/pb"

	"golang.org/x/net/context"
)

// -----------------------------------------------------------------------------

// lockedID wraps a `seq.ID` in a mutex.
type lockedID struct {
	*sync.RWMutex
	id seq.ID
}

// lockedIDMap wraps a map of `lockedID`s in a mutex.
type lockedIDMap struct {
	*sync.RWMutex
	ids map[string]*lockedID
}

// Load atomically loads the content of the map from the specified file `f`.
func (m *lockedIDMap) Load(f *os.File) error {
	m.Lock()
	defer m.Unlock()

	stat, err := f.Stat()
	if err != nil {
		return err
	}
	if stat.Size() <= 0 { // empty file, do nothing
		return nil
	}

	loaded := make(map[string]seq.ID, len(m.ids))
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if err := gob.NewDecoder(f).Decode(&loaded); err != nil {
		return err
	}
	for name, id := range loaded {
		m.ids[name] = &lockedID{RWMutex: &sync.RWMutex{}, id: id}
	}

	return nil
}

// Dump atomically dumps the content of the map to the specified file `f`.
func (m *lockedIDMap) Dump(f *os.File) error {
	m.Lock()
	defer m.Unlock()

	dumped := make(map[string]seq.ID, len(m.ids))
	for name, lid := range m.ids {
		dumped[name] = lid.id
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	return gob.NewEncoder(f).Encode(dumped)
}

// -----------------------------------------------------------------------------

// RRServer implements a single node of an RRCluster: a distributed system
// that guarantees the generation of sequential `ID`s by using RW quorums and
// read-repair conflict-resolution strategies.
//
// You can find more information about the ideas behind such a system in the
// `README.md` file at the root of this repository.
type RRServer struct {
	*grpc.Server
	addr net.Addr

	cp  *rpc.Pool
	ids *lockedIDMap

	f *os.File
}

// NewRRServer returns a new `RRServer` that forms a cluster with the specified
// peers.
//
// It is *not* `RRServer`'s job to do gossiping, service discovery and/or
// cluster topology management, et al.
// Please use the appropriate tools if you need such features.
// `RRServer` simply assumes that the list of peers you give it is exhaustive
// and correct. No more, no less.
// NOTE: RRServer currently doesn't support adding or removing peers while running.
//
// Set `addr` to "<host>:0" if you want to be assigned a port automatically,
// you can then retrieve the address of the server with `RRServer.Addr()`.
//
// Passing a non-empty `path` enables persistence to disk: the server will always
// write the new `ID` to disk before returning it to the client.
// Although this can severely impact performance, it allows the cluster to stay
// consistent even in the face of total failures (i.e. all nodes have
// simultaneously crashed).
// With disk synchronization disabled, an in-memory cluster of `RRServer`s can
// only sustain up to N-(N/2+1) (i.e. N-QUORUM) simultaneous node failures before
// losing its consistency guarantees.
//
// NOTE: you should always have an odd number of at least 3 nodes in your
// cluster to guarantee the availability of the system in case of a
// "half & (almost) half" netsplit situation.
func NewRRServer(addr string, path string, peerAddrs ...string) (*RRServer, error) {
	serv := &RRServer{
		ids: &lockedIDMap{
			RWMutex: &sync.RWMutex{},
			ids:     make(map[string]*lockedID),
		},
	}

	if len(path) > 0 { // enable disk persistence
		f, err := os.OpenFile(path, os.O_RDWR, 0600)
		if err != nil {
			return nil, err
		}
		serv.f = f
		if err := serv.ids.Load(f); err != nil { // load map from disk
			return nil, err
		}
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	serv.Server = grpc.NewServer()
	pb.RegisterRRAPIServer(serv.Server, serv)
	serv.addr = ln.Addr()
	go serv.Serve(ln)

	if len(peerAddrs) < 2 {
		log.Printf("warning: only %d peers specified\n", len(peerAddrs))
	}
	if len(peerAddrs)%2 == 1 {
		log.Printf("warning: uneven number of nodes in the cluster\n")
	}

	cp, err := rpc.NewPool(peerAddrs...)
	if err != nil {
		return nil, err
	}
	serv.cp = cp

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
// In case of recoverable failure, NextID will retry for as long as the
// level-1 context's deadline has not been exceeded and/or cancelled.
// NOTE: the `ctxL1` parameter is variable only because it is optional: only
// the first context passed will be used.
//
// `name` is the name of the sequence; a `RRServer` can hold as many sequences
// as you deem necessary.
//
// `rangeSize` will default to 1 in case it is < 1.
func (s *RRServer) NextID(
	name string, rangeSize int64, ctxL1 ...context.Context,
) (seq.ID, seq.ID) {
	if rangeSize < 1 {
		rangeSize = 1
	}

	for { // retry for as long as the client's deadline permits it

		if len(ctxL1) > 0 {
			select {
			case <-ctxL1[0].Done(): // deadline is either exceeded or cancelled..
				return 0, 0 // ..either way, we're outta here
			default:
			}
		}

		nbQuorum := (1+s.cp.Size())/2 + 1
		peerConns := s.cp.Conns()        // get all healthy peer-connections from the pool
		if len(peerConns)+1 < nbQuorum { // cannot reach a quorum, might as well give up now
			return 0, 0 // stop right now, it's not worth retrying
		}

		highestID := s.getHighestID(name, nbQuorum, peerConns, ctxL1...)
		if highestID == 0 {
			continue
		}

		fromID, toID := s.setHighestID(
			name, highestID, rangeSize, nbQuorum, peerConns, ctxL1...,
		)
		if fromID == 0 || toID == 0 { // just emphasizing the fact that this can return [0,0)
			continue
		}

		return fromID, toID
	}
}

func (s *RRServer) GRPCNextID(ctx context.Context, in *pb.NextIDRequest) (*pb.NextIDReply, error) {
	fromID, toID := s.NextID(in.Name, in.RangeSize, ctx)
	return &pb.NextIDReply{FromId: uint64(fromID), ToId: uint64(toID)}, nil
}

// getHighestID returns the highest `ID` cluster-wide.
//
// It concurrently fetches the current `ID` from `nbQuorum` nodes (including ourself),
// then returns the highest `ID` within this set.
//
// NOTE: the `ctxL1` parameter is variable only because it is optional: only
// the first context passed will be used.
//
// getHighestID returns `ID(0)` on failures (e.g. no quorum reached).
func (s *RRServer) getHighestID(
	name string, nbQuorum int, peerConns []*grpc.ClientConn,
	ctxL1 ...context.Context,
) seq.ID {
	var ctxL2 context.Context
	var canceller context.CancelFunc
	if len(ctxL1) > 0 { // inherit from the original client-side deadline context
		ctxL2, canceller = context.WithCancel(ctxL1[0])
	}

	wg := &sync.WaitGroup{}
	ids := make(chan seq.ID, len(peerConns)+1)
	ids <- s.getID(name)
	for _, pc := range peerConns {
		wg.Add(1)
		go s.getPeerID(pb.NewRRAPIClient(pc), name, ids, wg, ctxL2)
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

	canceller() // we got what we came for, stop everything downstream
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
// NOTE: the `ctxL1` parameter is variable only because it is optional: only
// the first context passed will be used.
//
// setHighestID returns a [0,0) range on failures (e.g. no quorum reached).
func (s *RRServer) setHighestID(
	name string, highestID seq.ID, rangeSize int64,
	nbQuorum int, peerConns []*grpc.ClientConn,
	ctxL1 ...context.Context,
) (seq.ID, seq.ID) {
	var ctxL2 context.Context
	var canceller context.CancelFunc
	if len(ctxL1) > 0 { // inherit from the original client-side deadline context
		ctxL2, canceller = context.WithCancel(ctxL1[0])
	}

	wg := &sync.WaitGroup{}
	newID := highestID + seq.ID(rangeSize)
	successes := make(chan bool, len(peerConns)+1)
	successes <- s.setID(name, newID)
	for _, pc := range peerConns {
		wg.Add(1)
		go s.setPeerID(pb.NewRRAPIClient(pc), name, newID, successes, wg, ctxL2)
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

	canceller() // we did what we came for, stop everything downstream
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
//
// NOTE: the `ctxL2` parameter is variable only because it is optional: only
// the first context passed will be used.
func (s *RRServer) getPeerID(
	peer pb.RRAPIClient,
	name string,
	ret chan<- seq.ID, wg *sync.WaitGroup,
	ctxL2 ...context.Context,
) error {
	defer wg.Done()

	var ctxL3 context.Context = context.Background()
	if len(ctxL2) > 0 {
		ctxL3 = ctxL2[0]
	}

	idReply, err := peer.GRPCCurID(ctxL3, &pb.CurIDRequest{Name: name})
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
func (s *RRServer) GRPCCurID(ctx context.Context, in *pb.CurIDRequest) (*pb.CurIDReply, error) {
	idChan := make(chan seq.ID)
	go func() { idChan <- s.CurID(in.Name) }()

	var id seq.ID
	select {
	case <-ctx.Done():
	case id = <-idChan:
	}

	return &pb.CurIDReply{CurId: uint64(id)}, nil
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
	var prev seq.ID
	if lockID.id < id { // current `ID` found, check that it's < newID
		prev, lockID.id = lockID.id, id

		if s.f != nil { // disk persistence enabled
			if s.ids.Dump(s.f) != nil {
				lockID.id = prev // persisting failed, rollback ID
				return false
			}
		}

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
	peer pb.RRAPIClient,
	name string, id seq.ID,
	ret chan<- bool, wg *sync.WaitGroup,
	ctxL2 ...context.Context,
) error {
	defer wg.Done()

	var ctxL3 context.Context = context.Background()
	if len(ctxL2) > 0 {
		ctxL3 = ctxL2[0]
	}

	idReply, err := peer.GRPCSetID(ctxL3, &pb.SetIDRequest{Name: name, NewId: uint64(id)})
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
func (s *RRServer) GRPCSetID(ctx context.Context, in *pb.SetIDRequest) (*pb.SetIDReply, error) {
	successChan := make(chan bool)
	go func() { successChan <- s.SetID(in.Name, seq.ID(in.NewId)) }()

	var success bool
	select {
	case <-ctx.Done():
	case success = <-successChan:
	}

	return &pb.SetIDReply{Success: success}, nil
}
