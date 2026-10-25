// Package registry defines the contract between HomeHub services and the
// portal: the ServiceRegistration payload and helpers for registering with it.
package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var nameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)

// ServiceRegistration is what a service sends to the portal at startup.
//
//	URL is the in-cluster address the portal uses for health checks.
//	UIPath is the browser path (served via the ingress) that cards link to.
type ServiceRegistration struct {
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description,omitempty"`
	Icon           string `json:"icon,omitempty"`
	URL            string `json:"url"`
	UIPath         string `json:"uiPath,omitempty"`
	HealthEndpoint string `json:"healthEndpoint,omitempty"`
	Version        string `json:"version,omitempty"`
}

// Normalize fills in defaults. It returns a copy.
func (r ServiceRegistration) Normalize() ServiceRegistration {
	r.Name = strings.ToLower(strings.TrimSpace(r.Name))
	r.DisplayName = strings.TrimSpace(r.DisplayName)
	if r.DisplayName == "" {
		r.DisplayName = r.Name
	}
	if r.HealthEndpoint == "" {
		r.HealthEndpoint = "/health"
	}
	if r.UIPath == "" {
		r.UIPath = "/" + r.Name
	}
	return r
}

// Validate checks a (normalized) registration.
func (r ServiceRegistration) Validate() error {
	if !nameRe.MatchString(r.Name) {
		return errors.New("name must be 1-40 chars of a-z, 0-9 and '-', not starting or ending with '-'")
	}
	u, err := url.Parse(r.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("url must be an absolute http(s) URL")
	}
	if !strings.HasPrefix(r.HealthEndpoint, "/") {
		return errors.New("healthEndpoint must start with '/'")
	}
	if !strings.HasPrefix(r.UIPath, "/") {
		return errors.New("uiPath must start with '/'")
	}
	return nil
}

// Register sends a single registration to the portal. It is idempotent: the
// portal upserts by name.
func Register(ctx context.Context, portalURL string, reg ServiceRegistration) error {
	body, err := json.Marshal(reg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(portalURL, "/")+"/api/services/register", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("portal returned %s", resp.Status)
	}
	return nil
}

// Deregister removes the service from the portal. Call on graceful shutdown.
func Deregister(ctx context.Context, portalURL, name string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		strings.TrimRight(portalURL, "/")+"/api/services/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("portal returned %s", resp.Status)
	}
	return nil
}

// KeepRegistered registers with the portal (retrying with backoff until it
// succeeds) and then refreshes the registration every refresh interval, so a
// portal restart never loses the service. It blocks until ctx is cancelled,
// then deregisters. Run it in a goroutine:
//
//	go registry.KeepRegistered(ctx, portalURL, reg, time.Minute)
func KeepRegistered(ctx context.Context, portalURL string, reg ServiceRegistration, refresh time.Duration) {
	backoff := time.Second
	for {
		wait := refresh
		if err := Register(ctx, portalURL, reg); err != nil {
			wait = backoff
			if backoff < 30*time.Second {
				backoff *= 2
			}
		} else {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = Deregister(shutdown, portalURL, reg.Name)
			return
		case <-time.After(wait):
		}
	}
}
