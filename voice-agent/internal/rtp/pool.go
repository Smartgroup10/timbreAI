package rtp

import (
	"errors"
	"sync"
)

// Pool hands out UDP ports from a configured range. Returned ports are taken out of rotation
// until Release is called. Designed for the External Media flow: each Asterisk channel needs its
// own port for the RTP socket while the call is alive.
type Pool struct {
	mu     sync.Mutex
	free   []int
	in_use map[int]bool
}

func NewPool(start, end int) *Pool {
	if end < start {
		start, end = end, start
	}
	free := make([]int, 0, end-start+1)
	for p := start; p <= end; p++ {
		free = append(free, p)
	}
	return &Pool{free: free, in_use: map[int]bool{}}
}

var ErrExhausted = errors.New("rtp_port_pool_exhausted")

func (p *Pool) Acquire() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.free) == 0 {
		return 0, ErrExhausted
	}
	port := p.free[0]
	p.free = p.free[1:]
	p.in_use[port] = true
	return port, nil
}

func (p *Pool) Release(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.in_use[port] {
		return
	}
	delete(p.in_use, port)
	p.free = append(p.free, port)
}

func (p *Pool) Stats() (free, inUse int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.free), len(p.in_use)
}
