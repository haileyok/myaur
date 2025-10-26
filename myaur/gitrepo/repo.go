package gitrepo

import (
	"bufio"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	DefaultAurRepoUrl = "https://github.com/archlinux/aur.git"
)

type Repo struct {
	logger     *slog.Logger
	repoPath   string
	aurRepoUrl string
}

type Args struct {
	RepoPath   string
	AurRepoUrl string
	Debug      bool
}

func New(args *Args) (*Repo, error) {
	level := slog.LevelInfo
	if args.Debug {
		level = slog.LevelDebug
	}

	if args.RepoPath == "" {
		return nil, fmt.Errorf("must supply a valid `RepoPath`")
	}

	if args.AurRepoUrl == "" {
		args.AurRepoUrl = DefaultAurRepoUrl
	}

	if _, err := url.Parse(args.AurRepoUrl); err != nil {
		return nil, fmt.Errorf("failed to parse AUR repo url: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	logger = logger.With("component", "gitrepo", "aururl", args.AurRepoUrl)

	return &Repo{
		logger:     logger,
		repoPath:   args.RepoPath,
		aurRepoUrl: args.AurRepoUrl,
	}, nil
}

func (r *Repo) EnsureRepo() error {
	if _, err := os.Stat(r.repoPath); os.IsNotExist(err) {
		r.logger.Info("aur repo does not exist, cloning...", "path", r.repoPath)
		return r.clone()
	}

	r.logger.Info("aur repo exists, fetching updates...", "path", r.repoPath)
	return r.fetch()
}

func (r *Repo) clone() error {
	cmd := exec.Command("git", "clone", "--mirror", r.aurRepoUrl, r.repoPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}

	r.logger.Info("repo cloned successfully")
	return nil
}

func (r *Repo) fetch() error {
	cmd := exec.Command("git", "-C", r.repoPath, "fetch", "--all", "--prune")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch updates: %w", err)
	}

	r.logger.Info("repo updated successfully")
	return nil
}

func (r *Repo) ListBranches() ([]string, error) {
	cmd := exec.Command("git", "-C", r.repoPath, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var branches []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		branch := strings.TrimSpace(scanner.Text())
		if branch != "" {
			branches = append(branches, branch)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning branch list: %w", err)
	}

	r.logger.Info("found branches", "count", len(branches))
	return branches, nil
}

func (r *Repo) GetFileContent(branch, filePath string) (string, error) {
	ref := filepath.Join("refs/heads", branch)
	gitPath := fmt.Sprintf("%s:%s", ref, filePath)

	cmd := exec.Command("git", "-C", r.repoPath, "show", gitPath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get file content for branch %s: %w", branch, err)
	}

	return string(output), nil
}
