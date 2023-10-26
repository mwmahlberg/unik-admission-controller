package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	zaplogfmt "github.com/jsternberg/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	debug bool = false
	addr  string

	runtimeScheme = runtime.NewScheme()
	codecFactory  = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecFactory.UniversalDeserializer()
)

func init() {
	flag.BoolVar(&debug, "debug", false, "enable debug mode")
	flag.StringVar(&addr, "addr", ":9090", "address to listen on")
}

func main() {
	flag.Parse()
	var cfg zapcore.EncoderConfig
	var level zapcore.Level
	if debug {
		cfg = zap.NewDevelopmentEncoderConfig()
		level = zap.DebugLevel
	} else {
		cfg = zap.NewProductionEncoderConfig()
		level = zap.InfoLevel
	}
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	logger := zap.New(zapcore.NewCore(zaplogfmt.NewEncoder(cfg), os.Stdout, level))
	if logger == nil {
		panic("logger is nil")
	}

	logger.Info("Starting unik admission controller")
	defer logger.Info("Exiting unik admission controller")
	defer logger.Sync()

	mux := http.NewServeMux()
	mux.HandleFunc("/validate", uuidMiddleware(logger.Named("request-id").With(zap.String("handler", "uuid")), serveValidate(logger.Named("validate").With(zap.String("handler", "validate")))))
	ctx, cancel := context.WithCancel(context.Background())

	srv := &http.Server{
		Addr:        addr,
		Handler:     mux,
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}
	srv.RegisterOnShutdown(func() { logger.Info("HTTP server shutdown complete") })
	srv.RegisterOnShutdown(cancel)
	go func() {
		logger.Info("Starting HTTP server", zap.String("addr", addr), zap.String("protocol", "http"))
		if err := srv.ListenAndServe(); err != nil {
			logger.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	s := <-sigs
	logger.Info("Shutting down", zap.String("signal", s.String()))

	gracefuleCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(gracefuleCtx); err != nil {
		logger.Error("Failed to shutdown HTTP server gracefully", zap.Error(err))
		defer os.Exit(1)
		return
	}
	defer os.Exit(0)
}

func serve(logger *zap.Logger, handler ValidationHandlerV1) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		logger.Debug("Received request", zap.String("request_id", id))
		defer logger.Debug("Finished request", zap.String("request_id", id))
		logger.Info("Validating Service", zap.String("request_id", id))

		if r.Body == nil {
			http.Error(w, "Please send a request body", http.StatusBadRequest)
			return
		} else if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			http.Error(w, fmt.Sprintf("Content-Type is %s, must be application/json", contentType), http.StatusBadRequest)
			return
		}

		var buffer = bytes.NewBuffer(nil)
		if _, err := buffer.ReadFrom(r.Body); err != nil {
			http.Error(w, fmt.Sprintf("Failed to read request body: %s", err), http.StatusInternalServerError)
			return
		}

		rto, gvk, err := deserializer.Decode(buffer.Bytes(), nil, nil)
		if err != nil {
			logger.Error("Failed to decode request body", zap.String("request_id", id), zap.Error(err))
			http.Error(w, fmt.Sprintf("Failed to decode request body: %s", err), http.StatusInternalServerError)
			return
		}
		requestedAdmissionReview, ok := rto.(*admissionv1.AdmissionReview)
		if !ok {
			logger.Error("Expected v1.AdmissionReview", zap.Any("got", rto))
			return
		}
		var responseObj runtime.Object
		responseAdmissionReview := &admissionv1.AdmissionReview{}
		responseAdmissionReview.SetGroupVersionKind(*gvk)
		responseAdmissionReview.Response = handler.validate(*requestedAdmissionReview)
		responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		responseObj = responseAdmissionReview

		if err := json.NewEncoder(w).Encode(responseObj); err != nil {
			logger.Error("Failed to encode response", zap.String("request_id", id), zap.Error(err))
			http.Error(w, fmt.Sprintf("Failed to encode response: %s", err), http.StatusInternalServerError)
			return
		}
	}
}

func serveValidate(logger *zap.Logger) http.HandlerFunc {
	return serve(logger, ValidationHandler(logger))
}
