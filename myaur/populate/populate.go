package populate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"

	"github.com/haileyok/myaur/myaur/database"
	"github.com/haileyok/myaur/myaur/gitrepo"
	"github.com/haileyok/myaur/myaur/srcinfo"
	"golang.org/x/sync/semaphore"
)

type Populate struct {
	logger *slog.Logger
	repo   *gitrepo.Repo
	db     *database.Database
	sem    *semaphore.Weighted
}

type Args struct {
	DatabasePath  string
	RepoPath      string
	RemoteRepoUrl string
	Debug         bool
	Concurrency   int
}

func New(args *Args) (*Populate, error) {
	level := slog.LevelInfo
	if args.Debug {
		level = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	logger = logger.With("component", "populate")

	if args.Concurrency == 0 {
		// TODO: good default? idk
		args.Concurrency = 10
	}

	repo, err := gitrepo.New(&gitrepo.Args{
		RepoPath:   args.RepoPath,
		AurRepoUrl: args.RemoteRepoUrl,
		Debug:      args.Debug,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create repo client: %w", err)
	}

	db, err := database.New(&database.Args{
		DatabasePath: args.DatabasePath,
		Debug:        args.Debug,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create database client: %w", err)
	}

	sem := semaphore.NewWeighted(int64(args.Concurrency))

	return &Populate{
		logger: logger,
		repo:   repo,
		db:     db,
		sem:    sem,
	}, nil
}

func (p *Populate) Run(ctx context.Context) error {
	p.logger.Info("starting populate process")

	// get the repo if we need to
	if err := p.repo.EnsureRepo(); err != nil {
		return fmt.Errorf("failed to ensure repository: %w", err)
	}

	// get all the branches that exist
	branches, err := p.repo.ListBranches()
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	p.logger.Info("processing branches", "total", len(branches))

	return p.processBranches(ctx, branches)
}

func (p *Populate) processBranches(ctx context.Context, branches []string) error {
	var wg sync.WaitGroup

	var processed, succeeded, failed atomic.Int64

	logger := p.logger.With("component", "branch-processor")

	for _, b := range branches {
		if err := p.sem.Acquire(ctx, 1); err != nil {
			logger.Error("failed to acuiqre semaphore", "err", err)
			continue
		}

		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				p.sem.Release(1)
			}()

			if err := p.processBranch(b); err != nil {
				logger.Error("failed to process branch", "branch", b, "err", err)
				failed.Add(1)
			} else {
				succeeded.Add(1)
			}
			processed.Add(1)

			logger.Info("progress", "processed", processed.Load(), "succeeded", succeeded.Load(), "failed", failed.Load(), "total", len(branches))
		}()
	}

	wg.Wait()

	logger.Info("database populated successfully", "processed", processed.Load(), "succeeded", succeeded.Load(), "failed", failed.Load())

	return nil
}

func (p *Populate) processBranch(branch string) error {
	content, err := p.repo.GetFileContent(branch, ".SRCINFO")
	if err != nil {
		return fmt.Errorf("failed to get .SRCINFO: %w", err)
	}

	pkg, err := srcinfo.Parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse .SRCINFO: %w", err)
	}

	if pkg.PackageBase == "" {
		pkg.PackageBase = branch
	}

	if err := p.db.UpsertPackage(pkg); err != nil {
		return fmt.Errorf("failed to upsert package: %w", err)
	}

	p.logger.Debug("processed package", "name", pkg.Name, "version", pkg.Version)

	return nil
}
