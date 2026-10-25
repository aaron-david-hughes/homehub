package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeAndValidate(t *testing.T) {
	r := ServiceRegistration{Name: " Bills ", URL: "http://bills:8080"}.Normalize()
	if r.Name != "bills" || r.DisplayName != "bills" || r.HealthEndpoint != "/health" || r.UIPath != "/bills" {
		t.Fatalf("unexpected defaults: %+v", r)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []ServiceRegistration{
		{Name: "", URL: "http://x"},
		{Name: "-x", URL: "http://x"},
		{Name: "x", URL: "x"},
		{Name: "x", URL: "http://x", HealthEndpoint: "h", UIPath: "/x"},
	} {
		if err := bad.Normalize().Validate(); err == nil {
			t.Errorf("expected error for %+v", bad)
		}
	}
}

func TestKeepRegisteredRetriesAndDeregisters(t *testing.T) {
	var posts, deletes atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if posts.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable) // first attempt fails
				return
			}
			var reg ServiceRegistration
			_ = json.NewDecoder(r.Body).Decode(&reg)
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			deletes.Add(1)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		KeepRegistered(ctx, srv.URL, ServiceRegistration{Name: "x", URL: "http://x"}.Normalize(), time.Hour)
		close(done)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for posts.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if posts.Load() < 2 {
		t.Fatal("did not retry after a failed registration")
	}
	cancel()
	<-done
	if deletes.Load() != 1 {
		t.Fatalf("want 1 deregister on shutdown, got %d", deletes.Load())
	}
}
