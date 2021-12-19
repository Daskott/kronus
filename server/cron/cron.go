package cron

import (
	"log"
	"time"

	"github.com/go-co-op/gocron"
)

func NewCronScheduler(timeZoneArg string) *gocron.Scheduler {
	timeZone, err := time.LoadLocation(timeZoneArg)
	if err != nil {
		log.Printf("warning: %v, falling back to UTC", err)
		timeZone = time.UTC
	}

	cronScheduler := gocron.NewScheduler(timeZone)
	cronScheduler.TagsUnique()

	return cronScheduler
}
