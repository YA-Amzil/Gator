package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"

	"gator/internal/cli"
	"gator/internal/config"
	"gator/internal/database"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	db, err := sql.Open("postgres", cfg.DBURL)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}

	s := &cli.State{
		DB:  database.New(db),
		Cfg: cfg,
	}

	cmds := cli.NewCommands()
	cmds.Register("register", cli.HandlerRegister)
	cmds.Register("login", cli.HandlerLogin)
	cmds.Register("reset", cli.HandlerReset)
	cmds.Register("users", cli.HandlerUsers)
	cmds.Register("feeds", cli.HandlerFeeds)
	cmds.Register("addfeed", cli.MiddlewareLoggedIn(cli.HandlerAddFeed))
	cmds.Register("follow", cli.MiddlewareLoggedIn(cli.HandlerFollow))
	cmds.Register("following", cli.MiddlewareLoggedIn(cli.HandlerFollowing))
	cmds.Register("unfollow", cli.MiddlewareLoggedIn(cli.HandlerUnfollow))
	cmds.Register("browse", cli.MiddlewareLoggedIn(cli.HandlerBrowse))
	cmds.Register("read", cli.MiddlewareLoggedIn(cli.HandlerRead))
	cmds.Register("unread", cli.MiddlewareLoggedIn(cli.HandlerUnread))
	cmds.Register("agg", cli.HandlerAgg)

	if len(os.Args) < 2 {
		return fmt.Errorf("usage: gator <command> [args...]")
	}

	cmd := cli.Command{
		Name: os.Args[1],
		Args: os.Args[2:],
	}

	return cmds.Run(s, cmd)
}
