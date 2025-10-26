package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/haileyok/myaur/myaur/database"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

type Server struct {
	logger       *slog.Logger
	echo         *echo.Echo
	httpd        *http.Server
	metricsHttpd *http.Server
	db           *database.Database
}

type Args struct {
	Addr         string
	MetricsAddr  string
	DatabasePath string
	Debug        bool
}

func New(args *Args) (*Server, error) {
	level := slog.LevelInfo
	if args.Debug {
		level = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	e := echo.New()

	httpd := http.Server{
		Addr:    args.Addr,
		Handler: e,
	}

	metricsHttpd := http.Server{
		Addr: args.MetricsAddr,
	}

	db, err := database.New(&database.Args{
		DatabasePath: args.DatabasePath,
		Debug:        args.Debug,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new database client: %w", err)
	}

	s := Server{
		echo:         e,
		httpd:        &httpd,
		metricsHttpd: &metricsHttpd,
		db:           db,
		logger:       logger,
	}

	return &s, nil
}

func (s *Server) Serve(ctx context.Context) error {
	go func() {
		logger := s.logger.With("component", "metrics-httpd")

		go func() {
			if err := s.metricsHttpd.ListenAndServe(); err != http.ErrServerClosed {
				logger.Error("error listening", "err", err)
			}
		}()

		logger.Info("myaur metrics server listening", "addr", s.metricsHttpd.Addr)
	}()

	shutdownEcho := make(chan struct{})
	echoShutdown := make(chan struct{})
	go func() {
		logger := s.logger.With("component", "echo")

		logger.Info("adding routes...")
		s.addRoutes()
		logger.Info("routes added")

		go func() {
			if err := s.httpd.ListenAndServe(); err != http.ErrServerClosed {
				logger.Error("error listning", "err", err)
				close(shutdownEcho)
			}
		}()

		logger.Info("myaur api server listening", "addr", s.httpd.Addr)

		<-shutdownEcho

		logger.Info("shutting down myaur api server")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer func() {
			cancel()
			close(echoShutdown)
		}()

		if err := s.httpd.Shutdown(ctx); err != nil {
			logger.Error("failed to shutdown myaur api server", "err", err)
			return
		}

		log.Info("myaur api server shutdown")
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	select {
	// if we receive a signal to shutdown, do so gracefully
	case sig := <-signals:
		s.logger.Info("shutting down on signal", "signal", sig)

		// close echo here since it shouldn't have been closed yet
		close(shutdownEcho)
	// if echo shutdowns unexepectdly, cleanup
	case <-echoShutdown:
		s.logger.Warn("echo shutdown unexpectedly")
		// echo should have already been closed
	}

	select {
	case <-echoShutdown:
		s.logger.Info("echo shutdown gracefully")
	case <-time.After(5 * time.Second):
		s.logger.Warn("echo did not shut down after five seconds. forcefully exiting.")
	}

	s.logger.Info("myaur shutdown")

	return nil
}

func (s *Server) addRoutes() {
	s.echo.GET("/rpc/v5/info", s.handleGetInfo)
	s.echo.GET("/rpc/v5/search", s.handleGetSearch)
}

func makeErrorJson(error string, message string) map[string]string {
	jsonMap := map[string]string{
		"error": error,
	}
	if message != "" {
		jsonMap["message"] = message
	}
	return jsonMap
}
