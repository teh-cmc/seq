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
	currentLock *sync.Mutex
}

// newRRAPIPool returns a new `rrAPIPool` with one client for each specified
// addresses.
//
// newRRAPIPool will fail if any of the client fails to connect.
func newRRAPIPool(addrs ...string) (*rrAPIPool, error) {
	clients := make([]RRAPIClient, len(addrs))
	for i, addr := range addrs {
		conn, err := grpc.Dial(addr,
			grpc.WithInsecure(), // no transport security
			grpc.WithBlock(),    // blocks until connection is first established
			grpc.WithUserAgent("RRSeq"),
		)
		if err != nil {
			return nil, err
		}
		clients[i] = NewRRAPIClient(conn)
	}

	return &rrAPIPool{
		clients:     clients,
		current:     rand.Intn(len(clients)), // start round-robin at random position
		currentLock: &sync.Mutex{},
	}, nil
}

// Size returns the size of the pool.
func (p *rrAPIPool) Size() int { return len(p.clients) }

// Client returns the next `RRAPIClient` in the pool by applying a simple
// round-robin algorithm.
func (p *rrAPIPool) Client() RRAPIClient {
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
