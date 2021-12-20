package models

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/utils"
	sqliteEncrypt "github.com/jackfr0st13/gorm-sqlite-cipher"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

const DB_NAME = "kronus.db"

var logg = logger.NewLogger()
var db *gorm.DB

// AutoMigrate auo-migrate db schema and insert seed data
func AutoMigrate(passPhrase string, dbRootDir string) error {
	err := openDB(passPhrase, dbRootDir)
	if err != nil {
		return err
	}

	db.AutoMigrate(
		&ProbeStatus{}, &JobStatus{}, &Job{},
		&Role{}, &Probe{}, &Contact{}, &ProbeSetting{},
		&User{}, &EmergencyProbe{},
	)

	populateDBWithSeedData()

	return nil
}

// ---------------------------------------------------------------------------------//
// Helper functions
// --------------------------------------------------------------------------------//
func openDB(passPhrase string, dbRootDir string) error {
	var err error
	var dbDSNVal string

	// TODO: pull db from google storage if it exist, before db starts

	dbDSNVal, err = dbDSN(passPhrase, dbRootDir)
	if err != nil {
		return fmt.Errorf("failed to set sqlite DSN: %v", err)
	}

	db, err = gorm.Open(sqliteEncrypt.Open(dbDSNVal), &gorm.Config{
		Logger: gormLogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormLogger.Config{
				LogLevel:                  gormLogger.Silent,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
	})
	if err != nil {
		return fmt.Errorf("failed to connect database: %v", err)
	}

	return nil
}

func populateDBWithSeedData() {
	if err := db.First(&ProbeStatus{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		logg.Info("Inserting seed data into 'ProbeStatus'")
		db.Create(&[]ProbeStatus{{Name: PENDING_PROBE}, {Name: GOOD_PROBE}, {Name: BAD_PROBE}, {Name: UNAVAILABLE_PROBE}, {Name: CANCELLED_PROBE}})
	}

	if err := db.First(&JobStatus{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		logg.Info("Inserting seed data into 'JobStatus'")
		db.Create(&[]JobStatus{{Name: ENQUEUED_JOB}, {Name: IN_PROGRESS_JOB}, {Name: SUCCESSFUL_JOB}, {Name: DEAD_JOB}})
	}

	if err := db.First(&Role{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		logg.Info("Inserting seed data into 'Role'")
		db.Create(&[]Role{{Name: "admin"}, {Name: "basic"}})
	}
}

func dbDSN(passPhrase string, dbRootDir string) (string, error) {
	dbDir, err := DbDirectory(dbRootDir)
	if err != nil {
		return "", err
	}

	dbFilePath := filepath.Join(dbDir, DB_NAME)
	dbName := fmt.Sprintf("file:%v", dbFilePath)

	return fmt.Sprintf(
		"%v?_pragma_key=%s&_pragma_cipher_page_size=4096&_journal_mode=WAL",
		dbName,
		passPhrase,
	), nil
}

func DbDirectory(dbRootDir string) (string, error) {
	dbDir := filepath.Join(dbRootDir, "db")

	err := utils.CreateDirIfNotExist(dbDir)
	if err != nil {
		return "", err
	}

	return dbDir, nil
}
