package database

import (
	"fmt"
	"log/slog"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Database struct {
	logger *slog.Logger
	db     *gorm.DB
}

type Args struct {
	DatabasePath string
	Debug        bool
}

func New(args *Args) (*Database, error) {
	level := slog.LevelInfo
	if args.Debug {
		level = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	logger = logger.With("component", "database")

	gormDb, err := gorm.Open(sqlite.Open(args.DatabasePath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := gormDb.AutoMigrate(
		&PackageInfo{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate db: %w", err)
	}

	db := Database{
		logger: logger,
		db:     gormDb,
	}

	return &db, nil
}

func (db *Database) UpsertPackage(pkg *PackageInfo) error {
	result := db.db.Where("name = ?", pkg.Name).FirstOrCreate(pkg)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		if err := db.db.Model(pkg).Where("name = ?", pkg.Name).Updates(pkg).Error; err != nil {
			return err
		}
	}

	return nil
}

func (db *Database) GetPackageByName(name string) (*PackageInfo, error) {
	var pkg PackageInfo
	if err := db.db.Where("name = ?", name).First(&pkg).Error; err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (db *Database) GetPackagesByName(name string) ([]PackageInfo, error) {
	var pkgs []PackageInfo
	searchTerm := "%" + name + "%"
	if err := db.db.Where("name LIKE ?", searchTerm).Find(&pkgs).Error; err != nil {
		return nil, err
	}
	return pkgs, nil
}

func (db *Database) GetPackageByDescriptionOrName(query string) (*PackageInfo, error) {
	var pkg PackageInfo
	if err := db.db.Where("name = ? OR description = ?", query, query).First(&pkg).Error; err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (db *Database) GetPackagesByDescriptionOrName(query string) ([]PackageInfo, error) {
	var pkgs []PackageInfo
	searchTerm := "%" + query + "%"
	if err := db.db.Where("name LIKE ? OR description LIKE ?", searchTerm, searchTerm).Find(&pkgs).Error; err != nil {
		return nil, err
	}
	return pkgs, nil
}

func (db *Database) GetPackagesByNames(names []string) ([]PackageInfo, error) {
	var pkgs []PackageInfo
	if err := db.db.Where("name IN ?", names).Find(&pkgs).Error; err != nil {
		return nil, err
	}
	return pkgs, nil
}
