package cron

import (
	"time"

	"github.com/go-co-op/gocron"
)

var CronScheduler *gocron.Scheduler

func init() {
	timeZone, err := time.LoadLocation("America/Toronto") // TODO: Read from config
	if err != nil {
		timeZone = time.UTC
	}
	CronScheduler = gocron.NewScheduler(timeZone)
	CronScheduler.TagsUnique()
}
