package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/kthaisociety/business-crm/frontend"
	"github.com/kthaisociety/business-crm/internal/config"
	"github.com/kthaisociety/business-crm/internal/database"
	"github.com/kthaisociety/business-crm/internal/handlers"
)

func main() {
	// Best effort: in a container the variables are set directly and there's no .env.
	_ = godotenv.Load()

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	if u := cfg.DevUser(); u != nil {
		log.Printf("WARNING: every request is signed in as dev user %s (docs/pcr/0005)", u.Email)
	} else {
		log.Printf("no DEV_USER_EMAIL: the API refuses every request but health until identity-service sign-in exists")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Connected but holding nothing yet: tables arrive with the first feature that stores data.
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := database.Migrate(ctx, db); err != nil {
		log.Fatal(err)
	}

	router := handlers.NewRouter(handlers.Deps{
		PingDB:   func(ctx context.Context) error { return database.Ping(ctx, db) },
		Frontend: frontend.Dist(),
		DevUser:  cfg.DevUser(),
	})

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("business-crm listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
