package server

import (
	"fmt"

	"github.com/Daskott/kronus/server/work"
)

// TODO: pull db from google storage if it exist, before db starts
func backupSqliteDb(map[string]interface{}) error {
	fmt.Println("Hello world!!!")
	return nil
}

func registerJobHandlers(wpa *work.WorkerPoolAdapter) {
	wpa.Register("backupSqliteDb", backupSqliteDb)
}

func enqueueJobs(wpa *work.WorkerPoolAdapter) {
	wpa.PeriodicallyPerform("*/2 * * * *", work.JobParams{
		Name:    "backupSqliteDb",
		Handler: "backupSqliteDb",
		Unique:  false,
		Args:    map[string]interface{}{},
	})
}
