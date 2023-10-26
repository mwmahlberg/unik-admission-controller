package main

import (
	"net/http"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	RequestIDHeader = "X-Request-ID"
)

// uuidMiddleware is a middleware that adds checks a request for a UUID.
// If the request does not have a UUID, it will generate one and add it
// to the request context and the response headers.
func uuidMiddleware(logger *zap.Logger, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("Checking for request ID")
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = uuid.New().String()
			logger.Debug("Generated request ID", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.String("request_id", id), zap.String("remote_addr", r.RemoteAddr))
			r.Header.Set(RequestIDHeader, id)
		}
		w.Header().Set(RequestIDHeader, id)
		logger.Debug("Serving request", zap.String("request_id", id))
		next.ServeHTTP(w, r)
		logger.Debug("Finished serving request", zap.String("request_id", id))
	}
}
