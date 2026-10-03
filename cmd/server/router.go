package main

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"net/url"
	"os"
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

// applyDBEnvOverrides lets the discrete DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/
// DB_NAME environment variables take precedence over (or fill in for) the
// corresponding parts of the DSN, so a stale host in DATABASE_URL does not
// prevent connecting to the real database host.
func applyDBEnvOverrides(dsn string) string {
	c, err := mysql.ParseDSN(dsn)
	if err != nil {
		c = mysql.NewConfig()
	}
	host := firstEnv("DB_HOST", "MYSQL_HOST")
	port := firstEnv("DB_PORT", "MYSQL_PORT")
	if host != "" {
		if port == "" {
			if _, p, err := net.SplitHostPort(c.Addr); err == nil && p != "" {
				port = p
			} else {
				port = "3306"
			}
		}
		c.Net = "tcp"
		c.Addr = net.JoinHostPort(host, port)
	}
	if v := firstEnv("DB_USER", "DB_USERNAME", "MYSQL_USER"); v != "" && c.User == "" {
		c.User = v
	}
	if v := firstEnv("DB_PASSWORD", "MYSQL_PASSWORD"); v != "" && c.Passwd == "" {
		c.Passwd = v
	}
	if v := firstEnv("DB_NAME", "DB_DATABASE", "MYSQL_DATABASE"); v != "" && c.DBName == "" {
		c.DBName = v
	}
	return c.FormatDSN()
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
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
	db, err := sql.Open("mysql", applyDBEnvOverrides(mysqlDSN(cfg.DatabaseURL)))
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