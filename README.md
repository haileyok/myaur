# myaur

A simple, self-hosted AUR (Arch User Repository) mirror written in Go. Provides both RPC API endpoints and git protocol access to AUR packages.

myaur takes advantage of the [official AUR mirror](https://github.com/archlinux/aur.git) on GitHub. You may use any mirror that you wish, however, note that it must have the same format as the official repo, in that each individual package should be a branch within the repo.

## Installation

### Using Docker Compose

The easiest way to start the mirror is to use `docker compose up -d`. This will start both the myaur service and set up a Caddy reverse proxy.

```bash
docker compose up -d
```

If you wish to use your own domain, modify the `Caddyfile` and change `:443` to your domain.

### Building from Source

Requirements:
- Go 1.25.3 or later
- Git

```bash
go build -o myaur ./cmd/myaur
```

## Usage

### Populate Database

If you wish to clone the mirror repo and populate the database, you can do so without actually serving the mirror API.

```bash
./myaur populate \
  --database-path ./myaur.db \
  --repo-path ./aur-mirror \
  --concurrency 10
```

Options:
- `--database-path`: Path to SQLite database file (default: `./myaur.db`)
- `--repo-path`: Path to clone/update AUR git mirror (default: `./aur-mirror`)
- `--remote-repo-url`: Remote AUR repository URL (default: `https://github.com/archlinux/aur.git`)
- `--concurrency`: Number of worker threads for parsing (default: `10`)
- `--debug`: Enable debug logging

### Serve

To serve the API:

```bash
./myaur serve \
  --listen-addr :8080 \
  --database-path ./myaur.db \
  --repo-path ./aur-mirror \
  --concurrency 10
```

Options:
- `--listen-addr`: HTTP server listen address (default: `:8080`)
- `--database-path`: Path to SQLite database file (default: `./myaur.db`)
- `--repo-path`: Path to AUR git mirror (default: `./aur-mirror`)
- `--remote-repo-url`: Remote AUR repository URL (default: `https://github.com/archlinux/aur.git`)
- `--concurrency`: Number of worker threads for parsing (default: `10`)
- `--auto-update`: Whether or not to automtically fetch updates from the remote repo (default: `true`)
- `--update-interval`: Time between automatic fetches (default: `1h`)
- `--debug`: Enable debug logging
