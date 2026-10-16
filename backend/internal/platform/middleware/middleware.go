package middleware

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"warehouse/backend/internal/platform/httpx"
)

type requestIDKey struct{}

func ID(ctx context.Context) string { id, _ := ctx.Value(requestIDKey{}).(string); return id }

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if !validID(id) {
			id = rand.Text()
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), requestIDKey{}, id))
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func validID(s string) bool {
	if len(s) < 1 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._-", r)) {
			return false
		}
	}
	return true
}

func Logging(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		} // Do not log raw paths/query strings containing client data.
		logger.InfoContext(c.Request.Context(), "http.request", "request_id", ID(c.Request.Context()), "method", c.Request.Method, "route", route, "status", c.Writer.Status(), "duration_ms", float64(time.Since(start).Microseconds())/1000)
	}
}

type ErrorMapper func(error) *httpx.Failure

func Errors(mapper ErrorMapper, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		err := c.Errors.Last().Err
		failure := mapper(err)
		if failure.Status >= 500 {
			// Arbitrary errors/panic values can contain secrets; log classifications only.
			attrs := []any{"request_id", ID(c.Request.Context()), "status", failure.Status}
			var panicErr *panicFailure
			if errors.As(err, &panicErr) {
				attrs = append(attrs, "stack", panicErr.stack)
			}
			logger.ErrorContext(c.Request.Context(), "http.error", attrs...)
		}
		httpx.WriteFailure(c, failure)
	}
}

type panicFailure struct{ stack string }

func (*panicFailure) Error() string { return "internal panic" }

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recover() != nil {
				httpx.Reject(c, &panicFailure{stack: string(debug.Stack())})
			}
		}()
		c.Next()
	}
}

func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		// This is cooperative cancellation; never race a handler with a writer goroutine.
		if ctx.Err() != nil && !c.Writer.Written() && len(c.Errors) == 0 {
			httpx.Reject(c, ctx.Err())
		}
	}
}

func BodyLimit(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) { c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit); c.Next() }
}
