package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog/log"

	"migrated-app/internal/config"
	"migrated-app/internal/httpapi"
	"migrated-app/internal/model"
	"migrated-app/internal/repository"
	"migrated-app/internal/service"
)

// mysqlDSN converts DATABASE_URL into a go-sql-driver/mysql DSN. A
// "mysql://user:pass@host:port/db?params" URL is translated; any other
// value is assumed to already be a native driver DSN.
func mysqlDSN(raw string) string {
	if !strings.HasPrefix(raw, "mysql://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		log.Error().Err(err).Msg("invalid DATABASE_URL")
		return raw
	}
	c := mysql.NewConfig()
	if u.User != nil {
		c.User = u.User.Username()
		if p, ok := u.User.Password(); ok {
			c.Passwd = p
		}
	}
	c.Net = "tcp"
	c.Addr = u.Host
	c.DBName = strings.TrimPrefix(u.Path, "/")
	if c.Params == nil {
		c.Params = map[string]string{}
	}
	for k, v := range u.Query() {
		if len(v) > 0 {
			c.Params[k] = v[0]
		}
	}
	return c.FormatDSN()
}

func openDB() *sql.DB {
	cfg, err := config.Load()
	if err != nil {
		log.Error().Err(err).Msg("failed to load config")
		cfg = &config.Config{}
	}
	if cfg.DatabaseURL == "" {
		log.Warn().Msg("DATABASE_URL is not set")
	}
	db, err := sql.Open("mysql", mysqlDSN(cfg.DatabaseURL))
	if err != nil {
		log.Error().Err(err).Msg("failed to open database")
		return db
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := model.EnsureUserSchema(ctx, db); err != nil {
		log.Error().Err(err).Msg("failed to ensure user schema")
	}
	return db
}

func buildRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	db := openDB()
	repo := repository.NewMySQLUserRepository(db)
	svc := service.NewUserService(repo)
	h := httpapi.NewHandler(svc, log.Logger)

	r.Mount("/", h.Routes())
	return r
}