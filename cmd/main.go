package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/revo-pos/ducktape/api/pkg/ducktape"
	"github.com/revo-pos/ducktape/internal/api"
	"github.com/revo-pos/ducktape/internal/logging"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	defaultDrainDelay = 10 * time.Second
	shutdownTimeout   = 5 * time.Minute
)

// basicAuthMiddleware provides basic authentication if AUTH_USERNAME and AUTH_PASSWORD are set
func basicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := os.Getenv("AUTH_USERNAME")
		password := os.Getenv("AUTH_PASSWORD")

		// Skip auth if credentials are not configured
		if username == "" || password == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Get credentials from request
		reqUsername, reqPassword, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Use constant time comparison to prevent timing attacks
		usernameMatch := subtle.ConstantTimeCompare([]byte(reqUsername), []byte(username)) == 1
		passwordMatch := subtle.ConstantTimeCompare([]byte(reqPassword), []byte(password)) == 1

		if !usernameMatch || !passwordMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	var level slog.Level
	logLevelEnv := os.Getenv("DUCKTAPE_LOG")

	switch strings.ToLower(logLevelEnv) {
	case "debug", "d":
		level = slog.LevelDebug
	case "info", "i":
		level = slog.LevelInfo
	case "warn", "w", "warning":
		level = slog.LevelWarn
	case "error", "e":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	infoHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Don't filter here, we'll filter in the custom handler
	})

	errorHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Don't filter here, we'll filter in the custom handler
	})

	logger := slog.New(&logging.SplitHandler{
		Level:        level,
		InfoHandler:  infoHandler,
		ErrorHandler: errorHandler,
	})
	slog.SetDefault(logger)

	mux := http.NewServeMux()

	api.RegisterApiRoutes(mux)
	api.RegisterHealthCheckRoutes(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Wrap the mux with h2c to support both HTTP/1.1 and HTTP/2
	h2cHandler := h2c.NewHandler(mux, &http2.Server{
		MaxReadFrameSize:             ducktape.RecommendedBufferSize,
		MaxUploadBufferPerConnection: ducktape.RecommendedBufferSize * 16, // 16 MB connection window
		MaxUploadBufferPerStream:     ducktape.RecommendedBufferSize * 4,  // 4 MB per stream
	})

	// Apply basic auth middleware
	handler := basicAuthMiddleware(h2cHandler)

	api.SetDraining(false)
	server := &http.Server{
		Addr:    "0.0.0.0:" + port,
		Handler: handler,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		log.Printf("Starting server on port %s\n", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrCh:
		log.Fatal(err)
	case <-signalContext.Done():
	}

	drainDelay := 0 * time.Second
	if drainDelayEnv := os.Getenv("DUCKTAPE_DRAIN_DELAY"); drainDelayEnv != "" {
		parsedDrainDelay, err := time.ParseDuration(drainDelayEnv)
		if err != nil {
			log.Printf("Invalid DUCKTAPE_DRAIN_DELAY=%q, using %s", drainDelayEnv, defaultDrainDelay)
			drainDelay = defaultDrainDelay
		} else if parsedDrainDelay < 0 {
			log.Printf("Ignoring negative DUCKTAPE_DRAIN_DELAY=%q, using %s", drainDelayEnv, defaultDrainDelay)
			drainDelay = defaultDrainDelay
		} else {
			drainDelay = parsedDrainDelay
		}
	}

	api.SetDraining(true)
	if drainDelay > 0 {
		log.Printf("Received shutdown signal, entering drain period for %s", drainDelay)
		time.Sleep(drainDelay)
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
		if closeErr := server.Close(); closeErr != nil {
			log.Printf("Forced close failed: %v", closeErr)
		}
	}
}
