package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aaron-david-hughes/homehub/services/portal/internal/registry"
	"github.com/aaron-david-hughes/homehub/services/portal/web"
)

func newTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	store := registry.NewStore()
	srv, err := New(&Server{
		Store: store, Poller: registry.NewPoller(store, time.Hour),
		Version: "test", Static: web.Static(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv, srv.Routes()
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRegisterListDeregister(t *testing.T) {
	_, h := newTestServer(t)

	rec := do(h, "POST", "/api/services/register",
		`{"name":"Thermostat","displayName":"Thermostat","url":"http://127.0.0.1:1","icon":"thermometer"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: want 201, got %d: %s", rec.Code, rec.Body)
	}
	rec = do(h, "POST", "/api/services/register", `{"name":"thermostat","url":"http://127.0.0.1:1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("re-register: want 200, got %d", rec.Code)
	}

	rec = do(h, "GET", "/api/services", "")
	if !strings.Contains(rec.Body.String(), `"uiPath":"/thermostat"`) {
		t.Fatalf("defaults not applied: %s", rec.Body)
	}

	if rec = do(h, "DELETE", "/api/services/thermostat", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("deregister: want 204, got %d", rec.Code)
	}
	if rec = do(h, "DELETE", "/api/services/thermostat", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("deregister twice: want 404, got %d", rec.Code)
	}
}

func TestRegisterValidation(t *testing.T) {
	_, h := newTestServer(t)
	for name, body := range map[string]string{
		"bad json":    `{`,
		"no name":     `{"url":"http://x"}`,
		"bad name":    `{"name":"Bad Name!","url":"http://x"}`,
		"no url":      `{"name":"x"}`,
		"non-http":    `{"name":"x","url":"ftp://x"}`,
		"bad health":  `{"name":"x","url":"http://x","healthEndpoint":"nope"}`,
		"bad ui path": `{"name":"x","url":"http://x","uiPath":"nope"}`,
	} {
		if rec := do(h, "POST", "/api/services/register", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, rec.Code)
		}
	}
}

func TestPagesRender(t *testing.T) {
	srv, h := newTestServer(t)
	srv.Store.AddDemo(sharedReg("bills", "Bills"))

	for _, path := range []string{"/", "/ui/services"} {
		rec := do(h, "GET", path, "")
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Bills") {
			t.Errorf("%s: code %d, body missing card", path, rec.Code)
		}
	}
	if rec := do(h, "GET", "/static/style.css", ""); rec.Code != 200 {
		t.Errorf("static css: %d", rec.Code)
	}
	if rec := do(h, "GET", "/nope", ""); rec.Code != 404 {
		t.Errorf("unknown path: want 404, got %d", rec.Code)
	}
	if rec := do(h, "GET", "/health", ""); rec.Code != 200 {
		t.Errorf("health: %d", rec.Code)
	}
}
