package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erp/pkg/config"
	"erp/pkg/httpserver"
	"erp/pkg/postgres"
	configclient "erp/services/purchasing-service/internal/adapters/config"
	httpadapter "erp/services/purchasing-service/internal/adapters/http"
	pgadapter "erp/services/purchasing-service/internal/adapters/postgres"
	stockclient "erp/services/purchasing-service/internal/adapters/stock"
	cashclient "erp/services/purchasing-service/internal/adapters/cashflow"
	"erp/services/purchasing-service/internal/application"
	"erp/services/purchasing-service/migrations"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, cfg.Postgres.DSN())
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool, migrations.FS, "."); err != nil {
		log.Fatal(err)
	}

	repo := pgadapter.New(pool)
	cfgClient := configclient.New(cfg.ConfigBaseURL)
	svc := application.New(pgadapter.Orders{Repo: repo}, pgadapter.Quotes{Repo: repo}, cfgClient, stockclient.New(cfg.StockBaseURL), cfgClient, cashclient.New(cfg.CashflowBaseURL))

	engine := httpserver.New(cfg.ServiceName)
	httpadapter.New(svc).Register(engine, httpserver.JWT(cfg.JWTSecret, cfg.JWTIssuer))

	srv := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: engine}
	go func() {
		log.Printf("%s listening on %s", cfg.ServiceName, srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
