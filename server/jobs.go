package server

import (
	"path/filepath"

	"github.com/Daskott/kronus/server/models"
	"github.com/Daskott/kronus/server/work"
	"github.com/Daskott/kronus/utils"
)

func backupSqliteDb(map[string]interface{}) error {
	logg.Info("Backing up Sqlite db...")

	dbDir, err := models.DbDirectory(configDir)
	if err != nil {
		return err
	}

	// Upload db file
	file := filepath.Join(dbDir, models.DB_NAME)
	if utils.FileExist(file) {
		err = storage.UploadFile(file)
		if err != nil {
			return err
		}
	}

	// Upload db shm file
	file = filepath.Join(dbDir, models.DB_NAME+"-shm")
	if utils.FileExist(file) {
		err = storage.UploadFile(file)
		if err != nil {
			return err
		}
	}

	// Upload db wal file
	file = filepath.Join(dbDir, models.DB_NAME+"-wal")
	if utils.FileExist(file) {
		err = storage.UploadFile(file)
		if err != nil {
			return err
		}
	}

	logg.Info("Sqlite db backup done")
	return nil
}

func registerJobHandlers(wpa *work.WorkerPoolAdapter) {
	wpa.Register("backupSqliteDb", backupSqliteDb)
}

func enqueueJobs(wpa *work.WorkerPoolAdapter) {
	if enabled, ok := config.Google.Storage.EnableSqliteBackupAndSync.(bool); ok && enabled {
		wpa.PeriodicallyPerform(config.Google.Storage.SqliteBackupSchedule,
			work.JobParams{
				Name:    "backupSqliteDb",
				Handler: "backupSqliteDb",
				Unique:  false,
				Args:    map[string]interface{}{},
			})
	} else {
		logg.Info("Sqlite db backup turned off")
	}
}
