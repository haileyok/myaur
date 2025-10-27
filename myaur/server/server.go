package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/haileyok/myaur/myaur/database"
	"github.com/haileyok/myaur/myaur/gitrepo"
	"github.com/haileyok/myaur/myaur/populate"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	slogecho "github.com/samber/slog-echo"
)

type Server struct {
	logger        *slog.Logger
	echo          *echo.Echo
	httpd         *http.Server
	metricsHttpd  *http.Server
	db            *database.Database
	populator     *populate.Populate
	remoteRepoUrl string
	repoPath      string
}

type Args struct {
	Addr          string
	MetricsAddr   string
	DatabasePath  string
	RemoteRepoUrl string
	RepoPath      string
	Debug         bool
}

func New(args *Args) (*Server, error) {
	level := slog.LevelInfo
	if args.Debug {
		level = slog.LevelDebug
	}

	if args.RemoteRepoUrl == "" {
		args.RemoteRepoUrl = gitrepo.DefaultAurRepoUrl
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	e := echo.New()

	e.Use(middleware.Recover())
	e.Use(slogecho.New(logger))

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

	populator, err := populate.New(&populate.Args{
		DatabasePath:  args.DatabasePath,
		RepoPath:      args.RepoPath,
		RemoteRepoUrl: args.RemoteRepoUrl,
		Debug:         args.Debug,
		Concurrency:   20, // TODO: make an env-var for this
	})

	s := Server{
		echo:          e,
		httpd:         &httpd,
		metricsHttpd:  &metricsHttpd,
		db:            db,
		populator:     populator,
		logger:        logger,
		remoteRepoUrl: args.RemoteRepoUrl,
		repoPath:      args.RepoPath,
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

	shutdownTicker := make(chan struct{})
	tickerShutdown := make(chan struct{})
	go func() {
		logger := s.logger.With("component", "update-routine")

		ticker := time.NewTicker(1 * time.Hour)

		go func() {
			logger.Info("performing initial database population")

			if err := s.populator.Run(ctx); err != nil {
				logger.Info("error populating", "err", err)
			}

			for range ticker.C {
				if err := s.populator.Run(ctx); err != nil {
					logger.Info("error populating", "err", err)
				}
			}

			close(tickerShutdown)
		}()

		<-shutdownTicker

		ticker.Stop()
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

	close(shutdownTicker)

	s.logger.Info("send ctrl+c to forcefully shutdown without waiting for routines to finish")

	forceShutdownSignals := make(chan os.Signal, 1)
	signal.Notify(forceShutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Go(func() {
		s.logger.Info("waiting up to 5 seconds for echo to shut down")
		select {
		case <-echoShutdown:
			s.logger.Info("echo shutdown gracefully")
		case <-time.After(5 * time.Second):
			s.logger.Warn("echo did not shut down after five seconds. forcefully exiting.")
		case <-forceShutdownSignals:
			s.logger.Warn("received forceful shutdown signal before echo shut down")
		}
	})

	wg.Go(func() {
		s.logger.Info("waiting up to 60 seconds for ticker to shut down")
		select {
		case <-tickerShutdown:
			s.logger.Info("ticker shutdown gracefully")
		case <-time.After(60 * time.Second):
			s.logger.Warn("waited 60 seconds for ticker to shut down. forcefully exiting.")
		case <-forceShutdownSignals:
			s.logger.Warn("received forceful shutdown signal before ticker shut down")
		}
	})

	s.logger.Info("waiting for routines to finish")
	wg.Wait()

	s.logger.Info("myaur shutdown")

	return nil
}

func (s *Server) addRoutes() {
	s.echo.GET("/rpc", s.handleRpc)
	s.echo.GET("/rpc/v5/info", s.handleGetInfo)
	s.echo.GET("/rpc/v5/search/:term", s.handleGetSearch)

	// git will make both get and post requests
	s.echo.GET("/*", s.handleGit)
	s.echo.POST("/*", s.handleGit)
}
