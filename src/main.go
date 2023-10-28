/*
 *     main.go is part of github.com/unik-k8s/admission-controller.
 *
 *     Copyright 2023 Markus W Mahlberg <07.federkleid-nagelhaut@icloud.com>
 *
 *     Licensed under the Apache License, Version 2.0 (the "License");
 *     you may not use this file except in compliance with the License.
 *     You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 *     Unless required by applicable law or agreed to in writing, software
 *     distributed under the License is distributed on an "AS IS" BASIS,
 *     WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *     See the License for the specific language governing permissions and
 *     limitations under the License.
 *
 */

package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	zaplogfmt "github.com/jsternberg/zap-logfmt"
	"github.com/unik-k8s/admission-controller/handler"
	"github.com/unik-k8s/admission-controller/validator"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	debug    bool = false
	addr     string
	certFile string
	keyFile  string

	clientset kubernetes.Interface
)

func init() {

	flag.BoolVar(&debug, "debug", false, "enable debug mode")
	flag.StringVar(&addr, "addr", ":9090", "address to listen on")
	flag.StringVar(&certFile, "cert", "/etc/certs/tls.crt", "path to TLS certificate")
	flag.StringVar(&keyFile, "key", "/etc/certs/tls.key", "path to TLS key")

}

func main() {
	flag.Parse()

	// Setup logging
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

	// Setup clientset
	var setupError error
	config, setupError := rest.InClusterConfig()

	if setupError != nil {
		panic(setupError.Error())
	}

	clientset, setupError = kubernetes.NewForConfig(config)
	if setupError != nil {
		panic(setupError.Error())
	}

	logger.Info("Starting unik admission controller")
	defer logger.Info("Exiting unik admission controller")
	defer logger.Sync()

	mux := http.NewServeMux()

	hl := logger.Named("handler").With(zap.String("handler", "validate"))

	validator, err := validator.NewValidationHandlerV1(validator.WithLogger(hl), validator.WithClientset(clientset))
	if err != nil {
		logger.Fatal("Failed to create validation handler", zap.Error(err))
	}

	mux.Handle("/validate", handler.AdmissionReviewRequesthandler(validator))
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
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil {
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
