// Command portal is the HomeHub front door: a dashboard that discovers
// services as they register themselves.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aaron-david-hughes/homehub/services/portal/internal/handlers"
	"github.com/aaron-david-hughes/homehub/services/portal/internal/registry"
	"github.com/aaron-david-hughes/homehub/services/portal/web"
	shared "github.com/aaron-david-hughes/homehub/shared/registry"
)

// version is set at build time: -ldflags "-X main.version=1.2.3"
var version = "dev"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	port := env("PORT", "8080")
	interval := envDuration("POLL_INTERVAL", 30*time.Second)
	seedDemo, _ := strconv.ParseBool(env("SEED_DEMO", "false"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := registry.NewStore()
	if seedDemo {
		seed(store)
	}
	poller := registry.NewPoller(store, interval)
	go poller.Run(ctx)

	srv, err := handlers.New(&handlers.Server{
		Store: store, Poller: poller, Version: version,
		Static: web.Static(), Log: log, BaseCtx: ctx,
	})
	if err != nil {
		log.Error("init", "err", err)
		os.Exit(1)
	}

	httpSrv := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("portal listening", "addr", httpSrv.Addr, "version", version, "pollInterval", interval.String())
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("serve", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdown)
}

// seed adds placeholder cards for services that are planned but not built yet.
// They are never polled and are replaced once the real service registers.
func seed(s *registry.Store) {
	s.AddDemo(shared.ServiceRegistration{
		Name: "thermostat", DisplayName: "Thermostat", Icon: "thermometer",
		Description: "Heating schedule, target temperature and boiler control.",
		UIPath:      "/thermostat",
	})
	s.AddDemo(shared.ServiceRegistration{
		Name: "bills", DisplayName: "Bills", Icon: "receipt",
		Description: "Upcoming bills, due dates and monthly summaries.",
		UIPath:      "/bills",
	})
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}
