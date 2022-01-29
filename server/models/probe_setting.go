package models

// At 18:00 every Wednesday
const DEFAULT_PROBE_CRON_EXPRESSION = "0 18 * * 3"

type ProbeSetting struct {
	BaseModel
	UserID            uint   `json:"user_id" gorm:"not null;unique"`
	Active            bool   `json:"active" gorm:"default:false"`
	CronExpression    string `json:"cron_expression" gorm:"not null"`
	MaxRetries        int    `json:"max_retries" gorm:"default:3"`
	WaitTimeInMinutes int    `json:"wait_time_in_minutes" gorm:"default:60"`
}
