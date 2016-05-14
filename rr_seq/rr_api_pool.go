package rrs

import (
	"math/rand"
	"sync"

	"google.golang.org/grpc"
)

// -----------------------------------------------------------------------------

// rrAPIPool implements a pool of gRPC clients for communicating
// with `RRServer`s.
type rrAPIPool struct {
	clients []RRAPIClient

	current     int
	currentLock *sync.RWMutex
}

// newRRAPIPool returns a new `rrAPIPool` with one client for each specified
// addresses.
//
// newRRAPIPool will fail if any of the client fails to connect.
func newRRAPIPool(addrs ...string) (*rrAPIPool, error) {
	p := &rrAPIPool{
		clients:     make([]RRAPIClient, 0, len(addrs)),
		currentLock: &sync.RWMutex{},
	}

	for _, addr := range addrs {
		if err := p.Add(addr); err != nil {
			return nil, err
		}
	}

	if nbClients := len(p.clients); nbClients > 0 {
		p.current = rand.Intn(nbClients) // start round-robin at random position
	}
	return p, nil
}

func (p *rrAPIPool) Add(addr string) error {
	conn, err := grpc.Dial(addr,
		grpc.WithInsecure(), // no transport security
		grpc.WithBlock(),    // blocks until connection is first established
		grpc.WithUserAgent("RRSeq"),
	)
	if err != nil {
		return err
	}

	p.currentLock.Lock()
	p.clients = append(p.clients, NewRRAPIClient(conn))
	p.currentLock.Unlock()

	return nil
}

// -----------------------------------------------------------------------------

// Size returns the size of the pool.
func (p *rrAPIPool) Size() int {
	p.currentLock.RLock()
	defer p.currentLock.RUnlock()
	return len(p.clients)
}

// ClientRoundRobin returns the next `RRAPIClient` in the pool by applying a
// simple round-robin algorithm.
func (p *rrAPIPool) ClientRoundRobin() RRAPIClient {
	p.currentLock.Lock()
	p.current++
	if p.current >= len(p.clients) {
		p.current = 0
	}
	c := p.clients[p.current]
	p.currentLock.Unlock()
	return c
}

// Close terminates all the clients present in the pool.
//
// It returns the first error met.
func (p *rrAPIPool) Close() (err error) {
	p.currentLock.Lock()
	defer p.currentLock.Unlock()

	for _, c := range p.clients {
		if e := c.(*rRAPIClient).cc.Close(); e != nil && err == nil {
			err = e
		}
	}

	p.clients = p.clients[0:0] // clear client list
	return
}
