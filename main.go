package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	url "net/url"
	sync "sync"
	atom "sync/atomic"
)

var loadBalancer = newLoadBalancer()

func newLoadBalancer() *LoadBalancer {
	var bs []*Backend
	lb := LoadBalancer{
		backends: bs,
		current:  0,
	}

	return &lb
}

func AddBackendHandler(w http.ResponseWriter, r *http.Request) {
	var req AddBackendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
		return
	}

	loadBalancer.AppendUrlString(req.path)
}

func main() {
	fmt.Println("Starting...")

	mux := http.NewServeMux()
	mux.HandleFunc("/add", AddBackendHandler)
	mux.Handle("/api/", loadBalancer)
}

type AddBackendRequest struct {
	path string
}

type LoadBalancer struct {
	backends []*Backend
	m        sync.Mutex
	current  uint32
}

type Backend struct {
	URL *url.URL
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	backend, err := lb.GetNextBackend()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(backend.URL)
	r.Host = backend.URL.Host
	proxy.ServeHTTP(w, r)
}

func (lb *LoadBalancer) AppendUrlString(s string) error {
	lb.m.Lock()
	defer lb.m.Unlock()

	u, err := url.Parse(s)

	if err != nil {
		return err
	}

	bs := Backend{
		URL: u,
	}

	lb.backends = append(lb.backends, &bs)

	return nil
}

func (lb *LoadBalancer) GetNextBackend() (*Backend, error) {
	l := uint32(len(lb.backends))
	if l == 0 {
		return nil, fmt.Errorf("No Backends configured")
	}

	next := atom.AddUint32(&lb.current, 1)

	return lb.backends[next%l], nil
}
