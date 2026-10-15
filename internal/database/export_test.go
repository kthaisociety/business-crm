package database

import (
	"context"
	"io/fs"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

// MigrateFS exposes migrate to the database_test package.
var MigrateFS = migrate

// DownFS rolls back the latest migration in fsys, to test a migration's Down section.
func DownFS(ctx context.Context, db *gorm.DB, fsys fs.FS) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys)
	if err != nil {
		return err
	}
	_, err = p.Down(ctx)
	return err
}
