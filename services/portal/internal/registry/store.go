// Package registry holds the portal's in-memory service registry and the
// health poller that keeps each service's status fresh.
package registry

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	shared "github.com/aaron-david-hughes/homehub/shared/registry"
)

// Status of a registered service as shown on the dashboard.
type Status string

const (
	StatusUnknown Status = "unknown" // registered, not yet checked
	StatusHealthy Status = "healthy"
	StatusStale   Status = "stale" // missed StaleAfter consecutive checks
	StatusDemo    Status = "demo"  // placeholder card, never polled
)

// StaleAfter is how many consecutive missed health checks mark a service stale.
const StaleAfter = 3

// Entry is a registered service plus its runtime state.
type Entry struct {
	shared.ServiceRegistration
	Status       Status    `json:"status"`
	Misses       int       `json:"misses"`
	RegisteredAt time.Time `json:"registeredAt"`
	LastChecked  time.Time `json:"lastChecked,omitzero"`
}

// Store is a concurrency-safe in-memory registry.
type Store struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	now     func() time.Time
}

func NewStore() *Store {
	return &Store{entries: map[string]*Entry{}, now: time.Now}
}

// Upsert adds or refreshes a registration. A refresh keeps the existing
// health state so periodic re-registration does not reset a service to unknown.
func (s *Store) Upsert(reg shared.ServiceRegistration) Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.entries[reg.Name]; ok && e.Status != StatusDemo {
		e.ServiceRegistration = reg
		return *e
	}
	e := &Entry{ServiceRegistration: reg, Status: StatusUnknown, RegisteredAt: s.now()}
	s.entries[reg.Name] = e
	return *e
}

// AddDemo adds a placeholder card that is never polled.
func (s *Store) AddDemo(reg shared.ServiceRegistration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.entries[reg.Name]; ok {
		return
	}
	s.entries[reg.Name] = &Entry{ServiceRegistration: reg, Status: StatusDemo, RegisteredAt: s.now()}
}

func (s *Store) Remove(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.entries[name]
	delete(s.entries, name)
	return ok
}

func (s *Store) Get(name string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[name]
	if !ok {
		return Entry{}, false
	}
	return *e, true
}

// List returns all entries sorted by display name.
func (s *Store) List() []Entry {
	s.mu.RLock()
	out := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, *e)
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName)
	})
	return out
}

// Record applies the result of a health check.
func (s *Store) Record(name string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.entries[name]
	if !found || e.Status == StatusDemo {
		return
	}
	e.LastChecked = s.now()
	if ok {
		e.Misses = 0
		e.Status = StatusHealthy
		return
	}
	e.Misses++
	if e.Misses >= StaleAfter {
		e.Status = StatusStale
	}
}

// Poller periodically checks every non-demo service's health endpoint.
type Poller struct {
	Store    *Store
	Client   *http.Client
	Interval time.Duration
}

func NewPoller(store *Store, interval time.Duration) *Poller {
	return &Poller{Store: store, Client: &http.Client{Timeout: 3 * time.Second}, Interval: interval}
}

// Run blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	t := time.NewTicker(p.Interval)
	defer t.Stop()
	for {
		p.CheckAll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// CheckAll checks every service concurrently and waits for completion.
func (p *Poller) CheckAll(ctx context.Context) {
	var wg sync.WaitGroup
	for _, e := range p.Store.List() {
		if e.Status == StatusDemo {
			continue
		}
		wg.Add(1)
		go func(e Entry) {
			defer wg.Done()
			p.Check(ctx, e)
		}(e)
	}
	wg.Wait()
}

// Check performs one health check and records the result.
func (p *Poller) Check(ctx context.Context, e Entry) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(e.URL, "/")+e.HealthEndpoint, nil)
	if err != nil {
		p.Store.Record(e.Name, false)
		return
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		p.Store.Record(e.Name, false)
		return
	}
	resp.Body.Close()
	p.Store.Record(e.Name, resp.StatusCode/100 == 2)
}
