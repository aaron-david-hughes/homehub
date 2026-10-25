// Package handlers wires the portal's HTTP routes: the dashboard UI, the HTMX
// fragment it polls, and the service registration API.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/aaron-david-hughes/homehub/services/portal/internal/registry"
	"github.com/aaron-david-hughes/homehub/services/portal/internal/templates"
	shared "github.com/aaron-david-hughes/homehub/shared/registry"
)

// Server holds the dependencies for the portal's handlers.
type Server struct {
	Store   *registry.Store
	Poller  *registry.Poller
	Version string
	Static  fs.FS
	Log     *slog.Logger
	// BaseCtx is used for background health checks triggered by registrations.
	BaseCtx context.Context

	tmpl *template.Template
	now  func() time.Time
}

// New parses templates and returns a ready Server.
func New(s *Server) (*Server, error) {
	s.now = time.Now
	if s.Log == nil {
		s.Log = slog.Default()
	}
	if s.BaseCtx == nil {
		s.BaseCtx = context.Background()
	}
	t, err := template.New("portal").Funcs(template.FuncMap{
		"icon":     icon,
		"greeting": func() string { return greeting(s.now()) },
	}).ParseFS(templates.FS, "*.html")
	if err != nil {
		return nil, err
	}
	s.tmpl = t
	return s, nil
}

// Routes returns the portal's HTTP handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.HandleFunc("GET /ui/services", s.servicesFragment)
	mux.HandleFunc("GET /health", s.health)

	mux.HandleFunc("GET /api/services", s.apiList)
	mux.HandleFunc("POST /api/services/register", s.apiRegister)
	mux.HandleFunc("DELETE /api/services/{name}", s.apiDeregister)

	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheFor(http.FileServerFS(s.Static))))
	return s.logging(mux)
}

// ---- UI ----

type card struct {
	Name, DisplayName, Description, Icon, Href, Version string
	Status, StatusLabel, Checked                        string
	Clickable, Soon                                     bool
}

type pageData struct {
	Cards   []card
	Live    int // non-demo services
	Healthy int
}

func (s *Server) pageData() pageData {
	var d pageData
	for _, e := range s.Store.List() {
		c := card{
			Name: e.Name, DisplayName: e.DisplayName, Description: e.Description,
			Icon: e.Icon, Href: e.UIPath, Version: e.Version,
			Status: string(e.Status), StatusLabel: statusLabel(e.Status),
			Clickable: e.Status != registry.StatusDemo,
			Soon:      e.Status == registry.StatusDemo,
		}
		if !e.LastChecked.IsZero() {
			c.Checked = ago(s.now().Sub(e.LastChecked))
		}
		if e.Status != registry.StatusDemo {
			d.Live++
			if e.Status == registry.StatusHealthy {
				d.Healthy++
			}
		}
		d.Cards = append(d.Cards, c)
	}
	return d
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		s.Log.Error("render", "template", name, "err", err)
	}
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	s.render(w, "index", s.pageData())
}

func (s *Server) servicesFragment(w http.ResponseWriter, r *http.Request) {
	s.render(w, "services", s.pageData())
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": s.Version})
}

// ---- API ----

func (s *Server) apiList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Store.List())
}

func (s *Server) apiRegister(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var reg shared.ServiceRegistration
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&reg); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	reg = reg.Normalize()
	if err := reg.Validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_, existed := s.Store.Get(reg.Name)
	e := s.Store.Upsert(reg)
	if !existed && s.Poller != nil {
		go s.Poller.Check(s.BaseCtx, e) // first check straight away
	}
	s.Log.Info("service registered", "name", reg.Name, "url", reg.URL, "new", !existed)
	code := http.StatusOK
	if !existed {
		code = http.StatusCreated
	}
	writeJSON(w, code, e)
}

func (s *Server) apiDeregister(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !s.Store.Remove(name) {
		writeErr(w, http.StatusNotFound, "no such service")
		return
	}
	s.Log.Info("service deregistered", "name", name)
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func cacheFor(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		h.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(c int) { r.code = c; r.ResponseWriter.WriteHeader(c) }

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: 200}
		next.ServeHTTP(rec, r)
		// The dashboard polls every few seconds; keep that out of the logs.
		if r.URL.Path == "/ui/services" || r.URL.Path == "/health" {
			return
		}
		s.Log.Info("http", "method", r.Method, "path", r.URL.Path, "status", rec.code,
			"dur", time.Since(start).Round(time.Millisecond).String())
	})
}

func statusLabel(s registry.Status) string {
	switch s {
	case registry.StatusHealthy:
		return "Healthy"
	case registry.StatusStale:
		return "Not responding"
	case registry.StatusDemo:
		return "Coming soon"
	default:
		return "Checking"
	}
}

func ago(d time.Duration) string {
	switch {
	case d < 5*time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

func greeting(t time.Time) string {
	switch h := t.Hour(); {
	case h < 5:
		return "Still up?"
	case h < 12:
		return "Good morning"
	case h < 18:
		return "Good afternoon"
	default:
		return "Good evening"
	}
}
