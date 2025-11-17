package databases

import (
	"database/sql"

	"github.com/lechgu/tichy/internal/config"
	_ "github.com/lib/pq"
	"github.com/samber/do/v2"
)

func New(i do.Injector) (*sql.DB, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
