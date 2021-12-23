package models

const (
	ENQUEUED_JOB    = "enqueued"
	IN_PROGRESS_JOB = "in-progress"
	SUCCESSFUL_JOB  = "successful"
	DEAD_JOB        = "dead"
	SCHEDULED_JOB   = "scheduled"
)

var JobStatusNameMap = map[string]bool{
	ENQUEUED_JOB:    true,
	IN_PROGRESS_JOB: true,
	SUCCESSFUL_JOB:  true,
	DEAD_JOB:        true,
	SCHEDULED_JOB:   true,
}

type JobsStats struct {
	EnueuedJobCount    int64 `json:"enueued_job_count"`
	InProgressJobCount int64 `json:"in_progress_job_count"`
	SuccessfulJobCount int64 `json:"successful_job_count"`
	DeadJobCount       int64 `json:"dead_job_count"`
}

type JobStatus struct {
	BaseModel
	Name string
	Jobs []Job `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func FindJobStatus(name string) (*JobStatus, error) {
	jobStatus := JobStatus{}
	err := db.Select("ID", "Name").First(&jobStatus, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return &jobStatus, nil
}
