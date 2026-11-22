// Package cmd assembles the camelot command tree and builds every
// dependency once: settings, logger, database, services.
package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/Wigata-Intech/w-tools/cli"
	"github.com/Wigata-Intech/w-tools/logger"

	"github.com/mrdhira/camelot/services/camelot/config"
	"github.com/mrdhira/camelot/services/camelot/internal/docrepository"
	"github.com/mrdhira/camelot/services/camelot/internal/docservice"
	"github.com/mrdhira/camelot/services/camelot/internal/feedrepository"
	"github.com/mrdhira/camelot/services/camelot/internal/feedservice"
	"github.com/mrdhira/camelot/services/camelot/internal/insightservice"
	"github.com/mrdhira/camelot/services/camelot/internal/pkg/sqlitedb"
	"github.com/mrdhira/camelot/services/camelot/internal/taskrepository"
	"github.com/mrdhira/camelot/services/camelot/internal/taskservice"
)

// keepBackups is how many daily backups are kept.
const keepBackups = 14

// Root returns the command tree; command output (JSON) goes to out, logs to logOut.
func Root(out, logOut io.Writer) *cli.Command {
	var loader *config.Loader
	load := func() (*config.Config, error) { return loader.Load() }
	return &cli.Command{
		Name:  "camelot",
		Short: "Dita's cockpit: cards, documents and insights (web UI for Dhira; CLI and MCP for agents)",
		Flags: func(fs *cli.FlagSet) {
			loader = config.Declare(fs)
		},
		Commands: []*cli.Command{
			serveCommand(load, logOut),
			taskCommand(load, out, logOut),
			docCommand(load, out, logOut),
			insightsCommand(load, out, logOut),
			checkCommand(load, out, logOut),
			backupCommand(load, out, logOut),
			importBoardCommand(load, out, logOut),
			exportBoardCommand(load, out, logOut),
			mcpCommand(load, out, logOut),
			probeCommand(load),
		},
	}
}

// deps is every built dependency.
type deps struct {
	config   *config.Config
	logger   *slog.Logger
	db       *sql.DB
	tasks    taskservice.Service
	docs     docservice.Service
	insights insightservice.Service
	feed     feedservice.Service
}

// open loads the settings, opens and migrates the database, takes the
// day's backup when none exists yet, and builds the services.
func open(ctx context.Context, load func() (*config.Config, error), logOut io.Writer) (*deps, error) {
	cfg, err := load()
	if err != nil {
		return nil, err
	}
	log := newLogger(logOut, cfg.LogLevel)
	if err := checkOwner(cfg.DB); err != nil {
		return nil, err
	}
	db, err := sqlitedb.Open(cfg.DB)
	if err != nil {
		return nil, err
	}
	for _, set := range []sqlitedb.MigrationSet{taskrepository.Migrations, docrepository.Migrations, feedrepository.Migrations} {
		if err := sqlitedb.Migrate(ctx, log, db, set); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if err := dailyBackup(ctx, log, db, cfg.DB); err != nil {
		log.WarnContext(ctx, "daily backup failed", "error", err)
	}
	docSvc := docservice.New(docservice.Config{Zone: cfg.Zone}, docrepository.NewMarkdown(cfg.Vault, cfg.DocRoots), docrepository.NewSQLite(db))
	taskSvc := taskservice.New(taskrepository.NewSQLite(db), taskrepository.NewFolders(cfg.Vault), docSvc)
	return &deps{
		config:   cfg,
		logger:   log,
		db:       db,
		tasks:    taskSvc,
		docs:     docSvc,
		insights: insightservice.New(taskSvc, docSvc),
		feed:     feedservice.New(feedrepository.NewSQLite(db), taskSvc, docSvc),
	}, nil
}

// close releases the database.
func (a *deps) close() {
	_ = a.db.Close()
}

// newLogger returns JSON logs to w at level, through the w-tools redacting wrapper.
func newLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(logger.Wrap(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}), logger.WrapConfig{}))
}

// dailyBackup writes backups/<date>.db beside the database once a day and
// keeps the newest keepBackups.
func dailyBackup(ctx context.Context, log *slog.Logger, db *sql.DB, dbPath string) error {
	dir := filepath.Join(filepath.Dir(dbPath), "backups")
	target := filepath.Join(dir, time.Now().UTC().Format(time.DateOnly)+".db")
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	if err := sqlitedb.Backup(ctx, db, target); err != nil {
		return err
	}
	log.InfoContext(ctx, "daily backup written", "path", target)
	return pruneBackups(dir)
}

// pruneBackups deletes all but the newest keepBackups backups in dir.
func pruneBackups(dir string) error {
	names, err := filepath.Glob(filepath.Join(dir, "????-??-??*.db"))
	if err != nil {
		return err
	}
	slices.Sort(names)
	for len(names) > keepBackups {
		if err := os.Remove(names[0]); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}

// writeJSON writes v to out as indented JSON.
func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// runWith opens the app, runs fn and closes it.
func runWith(load func() (*config.Config, error), logOut io.Writer, fn func(ctx context.Context, a *deps) error) func(ctx context.Context, args []string) error {
	return func(ctx context.Context, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unexpected arguments %q: every value is a flag (see --help)", args)
		}
		if err := refuseCardEnv(); err != nil {
			return err
		}
		a, err := open(ctx, load, logOut)
		if err != nil {
			return err
		}
		defer a.close()
		return fn(ctx, a)
	}
}
