package models

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	sqliteEncrypt "github.com/Daskott/gorm-sqlite-cipher"
	"github.com/Daskott/kronus/server/gstorage"
	"github.com/Daskott/kronus/server/logger"
	"github.com/Daskott/kronus/utils"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

const DB_NAME = "kronus.db"

var logg = logger.NewLogger()
var db *gorm.DB

// InitialiazeDb does 4 things to initialize the database
//
// - download sqlite backup db if backup is enabled to blob storage
//
// - open the db file for read & write
//
// - auto migrate schema
//
// - and finally populate db with seed data
func InitialiazeDb(passPhrase string, dbRootDir string, storage *gstorage.GStorage) error {
	// if blob storage client is provided, download backup sqlite files
	err := downloadDbBackups(storage, dbRootDir)
	if err != nil {
		return fmt.Errorf("failed to download sqlite backup: %v", err)
	}

	err = openDB(passPhrase, dbRootDir)
	if err != nil {
		return err
	}

	return autoMigrateAndSeedDb()
}

func InitializeTestDb() error {
	var err error

	db, err = gorm.Open(sqliteEncrypt.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		return err
	}

	return autoMigrateAndSeedDb()
}

func DbDirectory(dbRootDir string) (string, error) {
	dbDir := filepath.Join(dbRootDir, "db")

	err := utils.CreateDirIfNotExist(dbDir)
	if err != nil {
		return "", err
	}

	return dbDir, nil
}

// ---------------------------------------------------------------------------------//
// Helper functions
// --------------------------------------------------------------------------------//
func openDB(passPhrase string, dbRootDir string) error {
	var err error
	var dbDSNVal string

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

func autoMigrateAndSeedDb() error {
	err := db.AutoMigrate(
		&ProbeStatus{}, &JobStatus{}, &Job{},
		&Role{}, &Probe{}, &Contact{}, &ProbeSetting{},
		&User{}, &EmergencyProbe{},
	)
	if err != nil {
		return err
	}

	populateDBWithSeedData()

	return nil
}

func populateDBWithSeedData() {
	if err := db.First(&ProbeStatus{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		logg.Info("Inserting seed data into 'ProbeStatus'")
		db.Create(&[]ProbeStatus{{Name: PENDING_PROBE}, {Name: GOOD_PROBE}, {Name: BAD_PROBE}, {Name: UNAVAILABLE_PROBE}, {Name: CANCELLED_PROBE}})
	}

	if err := db.First(&JobStatus{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		logg.Info("Inserting seed data into 'JobStatus'")
		db.Create(&[]JobStatus{{Name: ENQUEUED_JOB}, {Name: IN_PROGRESS_JOB}, {Name: SUCCESSFUL_JOB}, {Name: DEAD_JOB}, {Name: SCHEDULED_JOB}})
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

func downloadDbBackups(storage *gstorage.GStorage, dbRootDir string) error {
	if storage == nil {
		logg.Info("Skipping sqlite db download - gstorage is nil")
		return nil
	}

	logg.Info("Downloading sqlite db backup...")

	dbDir, err := DbDirectory(dbRootDir)
	if err != nil {
		return err
	}

	// Download db file
	object := DB_NAME
	detinationFile := filepath.Join(dbDir, object)
	err = storage.DownloadFile(object, detinationFile)
	if err != nil && err != gstorage.ErrObjectNotExist {
		return err
	}

	// Download db shm file
	object = DB_NAME + "-shm"
	detinationFile = filepath.Join(dbDir, object)
	err = storage.DownloadFile(object, detinationFile)
	if err != nil && err != gstorage.ErrObjectNotExist {
		return err
	}

	// Download db wal file
	object = DB_NAME + "-wal"
	detinationFile = filepath.Join(dbDir, object)
	err = storage.DownloadFile(object, detinationFile)
	if err != nil && err != gstorage.ErrObjectNotExist {
		return err
	}

	logg.Info("Sqlite db download done")
	return nil
}
