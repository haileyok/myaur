package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/haileyok/myaur/myaur/gitrepo"
	"github.com/haileyok/myaur/myaur/populate"
	"github.com/haileyok/myaur/myaur/server"
	_ "github.com/joho/godotenv/autoload"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.App{
		Name:  "myaur",
		Usage: "a AUR mirror service",
		Commands: cli.Commands{
			&cli.Command{
				Name: "populate",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "database-path",
						Usage: "path to database file",
						Value: "./myaur.db",
					},
					&cli.StringFlag{
						Name:  "repo-path",
						Usage: "path to store/update the AUR git mirror",
						Value: "./aur-mirror",
					},
					&cli.StringFlag{
						Name:  "remote-repo-url",
						Usage: "remote aur repo url",
						Value: gitrepo.DefaultAurRepoUrl,
					},
					&cli.BoolFlag{
						Name:  "debug",
						Usage: "flag to enable debug logs",
					},
					&cli.IntFlag{
						Name:  "concurrency",
						Usage: "worker concurrency for parsing and adding packages to database",
						Value: 10,
					},
				},
				Action: func(cmd *cli.Context) error {
					ctx := context.Background()

					p, err := populate.New(&populate.Args{
						DatabasePath:  cmd.String("database-path"),
						RepoPath:      cmd.String("repo-path"),
						RemoteRepoUrl: cmd.String("remote-repo-url"),
						Debug:         cmd.Bool("debug"),
						Concurrency:   cmd.Int("concurrency"),
					})
					if err != nil {
						return fmt.Errorf("failed to create populate client: %w", err)
					}

					if err := p.Run(ctx); err != nil {
						return fmt.Errorf("failed to populate database: %w", err)
					}

					return nil
				},
			},
			&cli.Command{
				Name: "serve",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "listen-addr",
						Usage: "address to listen on for the web service",
						Value: ":8080",
					},
					&cli.StringFlag{
						Name:  "metrics-listen-addr",
						Usage: "metrics listen address",
						Value: ":8081",
					},
					&cli.StringFlag{
						Name:  "database-path",
						Usage: "path to database file",
						Value: "./myaur.db",
					},
					&cli.StringFlag{
						Name:  "remote-repo-url",
						Usage: "remote aur repo url",
						Value: gitrepo.DefaultAurRepoUrl,
					},
					&cli.StringFlag{
						Name:  "repo-path",
						Usage: "path to store/update the AUR git mirror",
						Value: "./aur-mirror",
					},
					&cli.BoolFlag{
						Name:  "debug",
						Usage: "flag to enable debug logs",
					},
					&cli.IntFlag{
						Name:  "concurrency",
						Usage: "worker concurrency for parsing and adding packages to database",
						Value: 10,
					},
					&cli.BoolFlag{
						Name:  "auto-update",
						Usage: "automatically pull updates from the remote repo at the set interval",
						Value: true,
					},
					&cli.DurationFlag{
						Name:  "update-interval",
						Usage: "the interval at which updates will be fetched. note that this should likely be at most one hour.",
						Value: time.Hour,
					},
				},
				Action: func(cmd *cli.Context) error {
					ctx := context.Background()

					s, err := server.New(&server.Args{
						Addr:           cmd.String("listen-addr"),
						DatabasePath:   cmd.String("database-path"),
						RemoteRepoUrl:  cmd.String("remote-repo-url"),
						RepoPath:       cmd.String("repo-path"),
						Concurrency:    cmd.Int("concurrency"),
						AutoUpdate:     cmd.Bool("auto-update"),
						UpdateInterval: cmd.Duration("update-interval"),
						Debug:          cmd.Bool("debug"),
					})
					if err != nil {
						return fmt.Errorf("failed to create new myaur server: %w", err)
					}

					if err := s.Serve(ctx); err != nil {
						return fmt.Errorf("failed to serve myaur server: %w", err)
					}

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
