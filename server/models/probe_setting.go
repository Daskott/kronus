package models

import "fmt"

const (
	DEFAULT_PROBE_CRON_DAY    = "3"
	DEFAULT_PROBE_CRON_HOUR   = "18"
	DEFAULT_PROBE_CRON_MINUTE = "0"
)

var (
	// At 18:00 every Wednesday
	DEFAULT_PROBE_CRON_EXPRESSION = fmt.Sprintf(
		"%v %v * * %v", DEFAULT_PROBE_CRON_MINUTE, DEFAULT_PROBE_CRON_HOUR, DEFAULT_PROBE_CRON_DAY)

	// Maps day of the week to it's numeric equivalent e.g. "sun": "0", "mon": "1" ...
	CRON_DAY_MAPPINGS = map[string]string{
		"sun": "0", "mon": "1", "tue": "2", "wed": "3", "thu": "4", "fri": "5", "sat": "5",
	}
)

type ProbeSetting struct {
	BaseModel
	UserID         uint   `gorm:"not null;unique"`
	Active         bool   `gorm:"default:false"`
	CronExpression string `gorm:"not null"`
}
