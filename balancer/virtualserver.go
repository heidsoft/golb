package balancer

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/onestraw/golb/chash"
	"github.com/onestraw/golb/config"
	"github.com/onestraw/golb/roundrobin"
)

const (
	LB_ROUNDROBIN    = "round-robin"
	LB_COSISTENTHASH = "consistent-hash"
	PROTO_HTTP       = "http"
	PROTO_GRPC       = "grpc"
)

type Pooler interface {
	String() string
	Size() int
	Get(args ...interface{}) string
	Add(addr string, args ...interface{})
	Remove(addr string)
}

type VirtualServer struct {
	sync.RWMutex
	Name         string
	Address      string
	ServerName   string
	Protocol     string
	LBMethod     string
	Pool         Pooler
	rp_lock      sync.RWMutex
	ReverseProxy map[string]*httputil.ReverseProxy
}

type VirtualServerOption func(*VirtualServer) error

func NameOpt(name string) VirtualServerOption {
	return func(vs *VirtualServer) error {
		vs.Name = name
		return nil
	}
}

func AddressOpt(addr string) VirtualServerOption {
	return func(vs *VirtualServer) error {
		vs.Address = addr
		return nil
	}
}

func ServerNameOpt(serverName string) VirtualServerOption {
	return func(vs *VirtualServer) error {
		if serverName == "" {
			serverName = "localhost"
		}
		vs.ServerName = serverName
		return nil
	}
}

func ProtocolOpt(proto string) VirtualServerOption {
	return func(vs *VirtualServer) error {
		if proto == "" {
			proto = PROTO_HTTP
		}
		vs.Protocol = proto
		return nil
	}
}

func LBMethodOpt(method string) VirtualServerOption {
	return func(vs *VirtualServer) error {
		if method != LB_ROUNDROBIN && method != LB_COSISTENTHASH {
			return ErrNotSupportedMethod
		}
		vs.LBMethod = method
		return nil
	}
}

func PoolOpt(method string, peers []config.Server) VirtualServerOption {
	return func(vs *VirtualServer) error {
		if method == "" {
			method = LB_ROUNDROBIN
		}
		if method == LB_ROUNDROBIN {
			pairs := make(map[string]int)
			for _, peer := range peers {
				pairs[peer.Address] = peer.Weight
			}
			vs.Pool = roundrobin.CreatePool(pairs)
		} else if method == LB_COSISTENTHASH {
			addrs := make([]string, len(peers))
			for i, peer := range peers {
				addrs[i] = peer.Address
			}
			vs.Pool = chash.CreatePool(addrs)
		} else {
			return ErrNotSupportedMethod
		}
		return nil
	}
}

func NewVirtualServer(opts ...VirtualServerOption) *VirtualServer {
	vs := &VirtualServer{
		ReverseProxy: make(map[string]*httputil.ReverseProxy),
	}
	for _, opt := range opts {
		if err := opt(vs); err != nil {
			panic(err)
		}
	}
	return vs
}

// ServeHTTP dispatch the request between backend servers
// TODO:
// 	1. buffer the request, check the response code
//  2. disable peer if code is 5xx, then retry with another peer
func (s *VirtualServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.RLock()
	defer s.RUnlock()

	if r.Host != s.ServerName {
		log.Errorf("Host not match, host=%s", r.Host)
		WriteError(w, ErrHostNotMatch)
		return
	}
	// use client's address as hash key if using consistent-hash method
	peer := s.Pool.Get(r.RemoteAddr)
	if peer == "" {
		log.Errorf("Peer not found")
		WriteError(w, ErrPeerNotFound)
		return
	}

	s.rp_lock.RLock()
	rp, ok := s.ReverseProxy[peer]
	s.rp_lock.RUnlock()
	if !ok {
		target, err := url.Parse("http://" + peer)
		if err != nil {
			log.Errorf("url.Parse peer=%s, error=%v", peer, err)
			WriteError(w, ErrInternalBalancer)
			return
		}
		log.Infof("%v", target)
		s.rp_lock.Lock()
		defer s.rp_lock.Unlock()
		// double check to avoid that the proxy is created while applying the lock
		if rp, ok = s.ReverseProxy[peer]; !ok {
			rp = httputil.NewSingleHostReverseProxy(target)
			s.ReverseProxy[peer] = rp
		}
	}
	rp.ServeHTTP(w, r)
}

func (s *VirtualServer) Run() {
	if s.Protocol == PROTO_HTTP {
		http.ListenAndServe(s.Address, s)
	} else {
		panic(ErrNotSupportedProto)
	}
}