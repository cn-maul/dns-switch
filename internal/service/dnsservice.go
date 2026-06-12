// Package service provides the Wails v3 Service binding for DNS operations.
package service

import (
	"fmt"
	"net"
	"sync"

	"dns-switch/internal/bench"
	"dns-switch/internal/config"
	"dns-switch/internal/dns"
)

// ── Response types (reused from internal/server) ──

// StatusResponse is the response for Status and Test calls.
type StatusResponse struct {
	Servers    map[string]string        `json:"servers"`
	Results    map[string]ResultEntry   `json:"results,omitempty"`
	BestServer string                   `json:"bestServer,omitempty"`
	BestRtt    float64                  `json:"bestRtt,omitempty"`
	CurrentDns string                   `json:"currentDns,omitempty"`
	Ok         bool                     `json:"ok,omitempty"`
	Error      string                   `json:"error,omitempty"`
}

// ResultEntry holds a single DNS server's benchmark result.
type ResultEntry struct {
	AvgRtt float64 `json:"avgRtt,omitempty"`
	Err    bool    `json:"err"`
	ErrMsg string  `json:"errMsg,omitempty"`
	Best   bool    `json:"best"`
}

// SetResponse is the response for the Set call.
type SetResponse struct {
	Ok         bool   `json:"ok"`
	CurrentDns string `json:"currentDns,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SimpleResponse is used for operations that only return ok/error.
type SimpleResponse struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ── DNSService ──

// DNSService provides DNS management operations to the Wails frontend.
type DNSService struct {
	cfgStore   config.Store
	dnsMgr     *dns.Manager
	mu         sync.RWMutex
	results    []bench.Result
	bestIdx    int
	currentDns string
}

// NewDNSService creates a new DNSService with the given dependencies.
func NewDNSService(cfgStore config.Store, dnsMgr *dns.Manager) *DNSService {
	svc := &DNSService{
		cfgStore: cfgStore,
		dnsMgr:   dnsMgr,
	}

	// Initialize currentDNS from config backup state
	if cfg, err := cfgStore.Read(); err == nil && cfg.Backup != nil {
		svc.currentDns = "已设置（查看备份记录）"
	} else {
		svc.currentDns = "DHCP"
	}

	return svc
}

// Status returns the current server list, cached benchmark results, and current DNS.
func (s *DNSService) Status() *StatusResponse {
	cfg, err := s.cfgStore.Read()
	if err != nil {
		return &StatusResponse{Servers: map[string]string{}, Error: err.Error()}
	}

	results, bestIdx := s.getResults()
	resp := &StatusResponse{
		Servers:    cfg.Servers,
		CurrentDns: s.getCurrentDNS(),
		Results:    buildResultMap(results, bestIdx),
	}
	if bestIdx >= 0 && bestIdx < len(results) {
		b := results[bestIdx]
		resp.BestServer = b.Name
		resp.BestRtt = b.AvgRTT
	}
	return resp
}

// Test runs a benchmark of all configured DNS servers and returns the results.
func (s *DNSService) Test() *StatusResponse {
	cfg, err := s.cfgStore.Read()
	if err != nil {
		return &StatusResponse{Error: err.Error()}
	}
	if len(cfg.Servers) == 0 {
		return &StatusResponse{Error: "没有配置 DNS 服务器"}
	}

	bench.Run(cfg.Servers, func(results []bench.Result, idx int) {
		s.setResults(results, idx)
	})

	results, bestIdx := s.getResults()
	resp := &StatusResponse{Servers: cfg.Servers, Results: buildResultMap(results, bestIdx)}
	if bestIdx >= 0 && bestIdx < len(results) {
		b := results[bestIdx]
		resp.BestServer = b.Name
		resp.BestRtt = b.AvgRTT
		if err := s.cfgStore.SaveLastTest(b.Name, b.AvgRTT); err != nil {
			fmt.Printf("ERR save last test: %v\n", err)
		}
	}
	return resp
}

// Set switches the system DNS to the server identified by name (from config).
func (s *DNSService) Set(name string) *SetResponse {
	if name == "" {
		return &SetResponse{Error: "缺少 name 参数"}
	}

	cfg, err := s.cfgStore.Read()
	if err != nil {
		return &SetResponse{Error: err.Error()}
	}

	ip, found := s.cfgStore.LookupServer(cfg.Servers, name)
	if !found {
		if net.ParseIP(name) == nil {
			return &SetResponse{Error: fmt.Sprintf("未知名称 %q", name)}
		}
		ip = name
	}

	if err := s.dnsMgr.Set([]string{ip}); err != nil {
		return &SetResponse{Error: err.Error()}
	}

	s.setCurrentDNS(name)
	return &SetResponse{Ok: true, CurrentDns: name}
}

// Restore restores the system DNS to DHCP.
func (s *DNSService) Restore() *SimpleResponse {
	if err := s.dnsMgr.Restore(); err != nil {
		return &SimpleResponse{Error: err.Error()}
	}
	s.setCurrentDNS("DHCP")
	return &SimpleResponse{Ok: true}
}

// ── Internal state management ──

func (s *DNSService) getResults() ([]bench.Result, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.results, s.bestIdx
}

func (s *DNSService) setResults(results []bench.Result, idx int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = results
	s.bestIdx = idx
}

func (s *DNSService) getCurrentDNS() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentDns
}

func (s *DNSService) setCurrentDNS(dns string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentDns = dns
}

// ── Helper ──

// buildResultMap converts bench results into a name-keyed map for JSON responses.
func buildResultMap(results []bench.Result, bestIdx int) map[string]ResultEntry {
	if len(results) == 0 {
		return nil
	}
	m := make(map[string]ResultEntry, len(results))
	for i, r := range results {
		e := ResultEntry{Err: r.Err, ErrMsg: r.ErrMsg, Best: i == bestIdx}
		if !r.Err {
			e.AvgRtt = r.AvgRTT
		}
		m[r.Name] = e
	}
	return m
}
