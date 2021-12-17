package models

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/Daskott/kronus/server/logger"
	sqliteEncrypt "github.com/jackfr0st13/gorm-sqlite-cipher"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

var logg = logger.NewLogger()
var db *gorm.DB

func init() {
	var err error
	db, err = gorm.Open(sqliteEncrypt.Open(dbDSN()), &gorm.Config{
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
		logg.Panicf("failed to connect database: %v", err)
	}
}

// AutoMigrate auo-migrate db schema and insert seed data
func AutoMigrate() {
	db.AutoMigrate(
		&ProbeStatus{}, &JobStatus{}, &Job{},
		&Role{}, &Probe{}, &Contact{}, &ProbeSetting{},
		&User{}, &EmergencyProbe{},
	)

	populateDBWithSeedData()
}

// ---------------------------------------------------------------------------------//
// Helper functions
// --------------------------------------------------------------------------------//

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

func dbDSN() string {
	passPhrase := url.QueryEscape("passphrase")
	dbName := fmt.Sprintf("file:%v", dbFilePath())
	return dbName + fmt.Sprintf("?_pragma_key=%s&_pragma_cipher_page_size=4096&_journal_mode=WAL", passPhrase)
}

func dbFilePath() string {
	configDir, err := os.Getwd()
	if err != nil {
		logg.Panic(err)
	}

	return filepath.Join(configDir, "kronus-dev.db")
}
