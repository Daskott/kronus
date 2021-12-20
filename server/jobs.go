package server

import (
	"path/filepath"

	"github.com/Daskott/kronus/models"
	"github.com/Daskott/kronus/server/work"
	"github.com/Daskott/kronus/utils"
)

func backupSqliteDb(map[string]interface{}) error {
	bucket := config.GetString("google.storage.bucket")

	dbDir, err := models.DbDirectory(configDir)
	if err != nil {
		return err
	}

	// Upload db file
	file := filepath.Join(dbDir, models.DB_NAME)
	if utils.FileExist(file) {
		err = storage.UploadFile(bucket, file)
		if err != nil {
			return err
		}
	}

	// Upload db shm file
	file = filepath.Join(dbDir, models.DB_NAME+"-shm")
	if utils.FileExist(file) {
		err = storage.UploadFile(bucket, file)
		if err != nil {
			return err
		}
	}

	// Upload db wal file
	file = filepath.Join(dbDir, models.DB_NAME+"-wal")
	if utils.FileExist(file) {
		err = storage.UploadFile(bucket, file)
		if err != nil {
			return err
		}
	}

	return nil
}

func registerJobHandlers(wpa *work.WorkerPoolAdapter) {
	wpa.Register("backupSqliteDb", backupSqliteDb)
}

func enqueueJobs(wpa *work.WorkerPoolAdapter) {
	if config.GetBool("google.storage.enableSQliteDbBackupAndSync") {
		wpa.PeriodicallyPerform("*/5 * * * *", work.JobParams{
			Name:    "backupSqliteDb",
			Handler: "backupSqliteDb",
			Unique:  false,
			Args:    map[string]interface{}{},
		})
	}
}
