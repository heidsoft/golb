package roundrobin

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Peer represents a backend server
type Peer struct {
	addr             string
	weight           int
	effective_weight int
	current_weight   int
	sync.RWMutex
}

// Pool is a group of Peers, one Peer can not belong to multiple Pool
type Pool struct {
	peers   []*Peer
	current uint64
	sync.RWMutex
}

func (p *Peer) String() string {
	p.RLock()
	defer p.RUnlock()
	return fmt.Sprintf("%s: (w=%d, ew=%d, cw=%d)",
		p.addr, p.weight, p.effective_weight, p.current_weight)
}

func (p *Pool) String() string {
	p.RLock()
	defer p.RUnlock()
	return fmt.Sprintf("%v", p.peers)
}

func (p *Pool) Size() int {
	return len(p.peers)
}

func (p *Pool) Add(peer *Peer) {
	if peer == nil {
		return
	}
	p.Lock()
	defer p.Unlock()

	p.peers = append(p.peers, peer)
}

func (p *Pool) Remove(peer *Peer) {
	if peer == nil {
		return
	}
	p.Lock()
	defer p.Unlock()

	indexOfPeer := func() int {
		for i, pr := range p.peers {
			if pr.addr == peer.addr {
				return i
			}
		}
		return -1
	}

	idx := indexOfPeer()
	if idx >= 0 && idx < p.Size() {
		p.peers = append(p.peers[:idx], p.peers[idx+1:]...)
	}
}

// GetPeer return peer in smooth weighted roundrobin method
func (p *Pool) Get() *Peer {
	p.RLock()
	defer p.RUnlock()

	var best *Peer = nil
	total := 0
	for _, peer := range p.peers {
		peer.Lock()

		total += peer.effective_weight
		peer.current_weight += peer.effective_weight

		if peer.effective_weight < peer.weight {
			peer.effective_weight += 1
		}

		if best == nil || best.current_weight < peer.current_weight {
			best = peer
		}
		peer.Unlock()
	}
	if best != nil {
		best.Lock()
		best.current_weight -= total
		best.Unlock()
	}
	return best
}

// EqualGetPeer get peer by turn, without considering weight
func (p *Pool) EqualGet() *Peer {
	p.RLock()
	defer p.RUnlock()

	if p.Size() <= 0 {
		return nil
	}

	old := atomic.AddUint64(&p.current, 1) - 1
	idx := old % uint64(p.Size())

	return p.peers[idx]
}

func CreatePeer(addr string, weight int) *Peer {
	return &Peer{
		addr:             addr,
		weight:           weight,
		effective_weight: weight,
		current_weight:   0,
	}
}

func CreatePool(peers []*Peer) *Pool {
	return &Pool{
		peers:   peers,
		current: 0,
	}
}