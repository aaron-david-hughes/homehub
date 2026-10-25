package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	shared "github.com/aaron-david-hughes/homehub/shared/registry"
)

func reg(name, url string) shared.ServiceRegistration {
	return shared.ServiceRegistration{Name: name, DisplayName: name, URL: url}.Normalize()
}

func TestStaleAfterThreeMisses(t *testing.T) {
	s := NewStore()
	s.Upsert(reg("a", "http://a"))

	s.Record("a", true)
	if e, _ := s.Get("a"); e.Status != StatusHealthy {
		t.Fatalf("want healthy, got %s", e.Status)
	}
	for i := 1; i < StaleAfter; i++ {
		s.Record("a", false)
		if e, _ := s.Get("a"); e.Status != StatusHealthy {
			t.Fatalf("after %d misses want still healthy, got %s", i, e.Status)
		}
	}
	s.Record("a", false)
	if e, _ := s.Get("a"); e.Status != StatusStale {
		t.Fatalf("want stale after %d misses, got %s", StaleAfter, e.Status)
	}
	s.Record("a", true)
	if e, _ := s.Get("a"); e.Status != StatusHealthy || e.Misses != 0 {
		t.Fatalf("want recovery to healthy, got %s/%d", e.Status, e.Misses)
	}
}

func TestReregisterKeepsHealth(t *testing.T) {
	s := NewStore()
	s.Upsert(reg("a", "http://a"))
	s.Record("a", true)
	s.Upsert(reg("a", "http://a2"))
	e, _ := s.Get("a")
	if e.Status != StatusHealthy || e.URL != "http://a2" {
		t.Fatalf("refresh should keep health and update fields, got %s %s", e.Status, e.URL)
	}
}

func TestRealRegistrationReplacesDemo(t *testing.T) {
	s := NewStore()
	s.AddDemo(reg("thermostat", ""))
	s.Upsert(reg("thermostat", "http://thermostat:8080"))
	e, _ := s.Get("thermostat")
	if e.Status != StatusUnknown {
		t.Fatalf("want unknown after real registration, got %s", e.Status)
	}
}

func TestDemoIsNeverPolled(t *testing.T) {
	s := NewStore()
	s.AddDemo(reg("soon", "http://127.0.0.1:1"))
	s.Record("soon", false)
	if e, _ := s.Get("soon"); e.Status != StatusDemo || e.Misses != 0 {
		t.Fatalf("demo entry changed: %+v", e)
	}
}

func TestPollerChecksHealthEndpoint(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			w.WriteHeader(404)
		}
	}))
	defer up.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer down.Close()

	s := NewStore()
	s.Upsert(reg("up", up.URL))
	s.Upsert(reg("down", down.URL))
	p := NewPoller(s, time.Hour)
	for i := 0; i < StaleAfter; i++ {
		p.CheckAll(context.Background())
	}
	if e, _ := s.Get("up"); e.Status != StatusHealthy {
		t.Errorf("up: want healthy, got %s", e.Status)
	}
	if e, _ := s.Get("down"); e.Status != StatusStale {
		t.Errorf("down: want stale, got %s", e.Status)
	}
}
