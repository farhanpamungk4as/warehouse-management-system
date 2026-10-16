package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	inventoryhandler "warehouse/backend/internal/modules/inventory/handler"
	inventorypostgres "warehouse/backend/internal/modules/inventory/repository/postgres"
	inventoryservice "warehouse/backend/internal/modules/inventory/service"
	itemhandler "warehouse/backend/internal/modules/item/handler"
	itempostgres "warehouse/backend/internal/modules/item/repository/postgres"
	itemservice "warehouse/backend/internal/modules/item/service"
	locationhandler "warehouse/backend/internal/modules/location/handler"
	locationpostgres "warehouse/backend/internal/modules/location/repository/postgres"
	locationservice "warehouse/backend/internal/modules/location/service"
	stockhandler "warehouse/backend/internal/modules/stock/handler"
	stockpostgres "warehouse/backend/internal/modules/stock/repository/postgres"
	stockservice "warehouse/backend/internal/modules/stock/service"
	"warehouse/backend/internal/platform/cache"
	"warehouse/backend/internal/platform/config"
	"warehouse/backend/internal/platform/database"
	"warehouse/backend/internal/platform/httpx"
	"warehouse/backend/internal/platform/middleware"
)

// Pinger exists to test readiness independently of a running database.
type Pinger interface{ Ping(context.Context) error }

func NewRouter(cfg config.Config, logger *slog.Logger, db Pinger, items itemhandler.UseCases, stock stockhandler.UseCases) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	_ = r.SetTrustedProxies(nil)
	r.Use(middleware.RequestID(), middleware.Logging(logger), middleware.Errors(mapError, logger), middleware.Recovery(), middleware.Timeout(cfg.RequestTimeout), middleware.BodyLimit(64<<10))
	if items != nil {
		itemhandler.New(items).RegisterRoutes(r.Group("/api/v1/items"))
	}
	if stock != nil {
		stockhandler.New(stock).RegisterRoutes(r.Group("/api/v1/stock"))
	}
	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			httpx.Reject(c, database.ErrUnavailable)
			return
		}
		httpx.Success(c, http.StatusOK, "Service ready", gin.H{"status": "ready"})
	})
	r.NoRoute(func(c *gin.Context) { httpx.Reject(c, httpx.NewFailure(404, "Not found", "Route does not exist")) })
	r.NoMethod(func(c *gin.Context) {
		if c.Request.URL.Path == "/healthz" {
			c.Header("Allow", "GET")
		} else if c.Request.URL.Path == "/api/v1/dashboard/summary" || c.Request.URL.Path == "/api/v1/inventory/overview" {
			c.Header("Allow", "GET")
		} else if c.Request.URL.Path == "/api/v1/locations" {
			c.Header("Allow", "GET, POST")
		} else if strings.HasPrefix(c.Request.URL.Path, "/api/v1/locations/") && !strings.Contains(strings.TrimPrefix(c.Request.URL.Path, "/api/v1/locations/"), "/") {
			c.Header("Allow", "GET, PUT, DELETE")
		} else if items != nil && c.Request.URL.Path == "/api/v1/items" {
			c.Header("Allow", "GET, POST")
		} else if items != nil && strings.HasPrefix(c.Request.URL.Path, "/api/v1/items/") && !strings.Contains(strings.TrimPrefix(c.Request.URL.Path, "/api/v1/items/"), "/") {
			c.Header("Allow", "GET, PUT, DELETE")
		}
		if stock != nil && strings.HasPrefix(c.Request.URL.Path, "/api/v1/stock/") && !strings.Contains(strings.TrimPrefix(c.Request.URL.Path, "/api/v1/stock/"), "/") {
			if c.Request.URL.Path == "/api/v1/stock/receive" || c.Request.URL.Path == "/api/v1/stock/transfer" {
				c.Header("Allow", "GET, POST")
			} else {
				c.Header("Allow", "GET")
			}
		}
		httpx.Reject(c, httpx.NewFailure(405, "Method not allowed", "Method is not supported for this route"))
	})
	return r
}

// NewApplicationRouter composes the same concrete modules for runtime and API tests.
func NewApplicationRouter(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool) *gin.Engine {
	items := itemservice.New(itempostgres.New(pool))
	stock := stockservice.New(stockpostgres.New(pool), NewReceiveRunner(pool), logger, middleware.ID)
	return composeRouter(cfg, logger, pool, items, stock, nil)
}
func composeRouter(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, items itemhandler.UseCases, stock stockhandler.UseCases, locationsCache locationservice.Cache) *gin.Engine {
	r := NewRouter(cfg, logger, pool, items, stock)
	stockhandler.RegisterTransfer(r, stockservice.NewTransfer(NewTransferRunner(pool), logger, middleware.ID))
	locationhandler.Register(r, locationservice.New(locationpostgres.New(pool), locationsCache, cfg.RedisTTL))
	inventoryhandler.Register(r, inventoryservice.New(inventorypostgres.New(pool)))
	return r
}

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	pool, err := database.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns, cfg.StartupTimeout)
	if err != nil {
		return err
	}
	defer pool.Close()
	return serve(ctx, cfg, logger, pool)
}

func serve(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool) error {
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return errors.New("could not listen on HTTP_ADDR")
	}
	var locationsCache locationservice.Cache
	if cfg.RedisURL != "" {
		redis, err := cache.Open(cfg.RedisURL)
		if err != nil {
			return errors.New("REDIS_URL is invalid")
		}
		defer redis.Close()
		locationsCache = redis
	}
	items := itemservice.New(itempostgres.New(pool))
	stock := stockservice.New(stockpostgres.New(pool), NewReceiveRunner(pool), logger, middleware.ID)
	server := &http.Server{Handler: composeRouter(cfg, logger, pool, items, stock, locationsCache), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: cfg.RequestTimeout + 5*time.Second, IdleTimeout: time.Minute}
	failed := make(chan error, 1)
	go func() { failed <- server.Serve(listener) }()
	logger.Info("server.started", "address", listener.Addr().String())
	select {
	case err := <-failed:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP server failed")
	case <-ctx.Done():
		logger.Info("server.stopping")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return errors.New("HTTP shutdown deadline exceeded")
		}
		<-failed
		logger.Info("server.stopped")
		return nil
	}
}
