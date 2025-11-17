package db

import (
	"database/sql"

	"github.com/lechgu/tichy/internal/injectors"
	_ "github.com/lechgu/tichy/internal/migrations"
	"github.com/pressly/goose/v3"
	"github.com/samber/do/v2"
	"github.com/spf13/cobra"
)

var reset = &cobra.Command{
	Use:   "reset",
	Short: "Roll back all migrations",
	RunE:  doReset,
}

func doReset(cmd *cobra.Command, args []string) error {
	db, err := do.Invoke[*sql.DB](injectors.Default)
	if err != nil {
		return err
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	goose.SetLogger(goose.NopLogger())

	if err := goose.Reset(db, "."); err != nil {
		return err
	}

	cmd.Println("OK")
	return nil
}
