// Command server runs the Velké konflikty meeting-preparation app: the API and,
// in production, the embedded SPA.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/auth"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/config"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/httpapi"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/spa"
	"github.com/kareltilcer-oksystem/konflikty-priprava/internal/store"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if err := cfg.EnsureDirs(); err != nil {
		return err
	}
	users, err := auth.ParseUsers(cfg.AuthUsersRaw)
	if err != nil {
		return err
	}
	if n := users.AdminCount(); n > 1 {
		slog.Warn("AUTH_USERS defines more than one admin", "count", n)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	if n, err := st.PurgeExpiredSessions(context.Background(), time.Now()); err != nil {
		slog.Error("purge sessions at start-up", "err", err)
	} else if n > 0 {
		slog.Info("purged expired sessions", "count", n)
	}

	apiHandler := httpapi.New(httpapi.Deps{
		Store:      st,
		Users:      users,
		Config:     cfg,
		Version:    version,
		Now:        time.Now,
		LoginDelay: time.Second,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		purgeDaily(ctx, st)
	}()

	var servers []*http.Server

	// The API listener. This is the only place CORS exists: it is the only
	// listener a browser can reach cross-origin.
	apiSrv := newServer(cfg.APIPort, httpapi.Recover(httpapi.Log(httpapi.CORS(cfg.CORSOrigins, apiHandler))))
	servers = append(servers, apiSrv)

	// The SPA listener, when enabled. It routes /api to the very same
	// in-process handler, so the browser is always same-origin and root-relative
	// URLs — every attachment src, every download link — resolve identically in
	// development and production. WEB_PORT=0 suppresses it, which is what
	// development sets because Vite already owns that port.
	if cfg.SPAEnabled() {
		if !spa.Available() {
			slog.Warn("no frontend is embedded in this binary; serving a placeholder page. " +
				"Build it with: npm --prefix web run build")
		}
		webMux := http.NewServeMux()
		webMux.Handle("/api/", apiHandler) // no CORS wrapper: same-origin by construction
		webMux.Handle("/", spa.Handler())
		servers = append(servers, newServer(cfg.WebPort, httpapi.Recover(httpapi.Log(webMux))))
	}

	errCh := make(chan error, len(servers))
	for _, srv := range servers {
		wg.Add(1)
		go func(s *http.Server) {
			defer wg.Done()
			slog.Info("listening", "addr", s.Addr)
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("listen on %s: %w", s.Addr, err)
			}
		}(srv)
	}

	var runErr error
	select {
	case <-ctx.Done():
		slog.Info("shutting down")
	case runErr = <-errCh:
	}
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, srv := range servers {
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown", "addr", srv.Addr, "err", err)
		}
	}
	wg.Wait()
	return runErr
}

// newServer builds a listener.
//
// ReadTimeout and WriteTimeout are deliberately unset. A 512 MB upload or a
// 100 MB video download over a slow LAN link exceeds any value that would be
// sane for a normal request, and either one would cut the transfer off
// mid-stream. ReadHeaderTimeout still bounds a client that opens a connection
// and never sends a request.
func newServer(port int, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// purgeDaily clears expired sessions once a day.
func purgeDaily(ctx context.Context, st *store.Store) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := st.PurgeExpiredSessions(ctx, time.Now()); err != nil {
				slog.Error("purge sessions", "err", err)
			} else if n > 0 {
				slog.Info("purged expired sessions", "count", n)
			}
		}
	}
}
