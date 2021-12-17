package models

const (
	ENQUEUED_JOB    = "enqueued"
	IN_PROGRESS_JOB = "in-progress"
	SUCCESSFUL_JOB  = "successful"
	DEAD_JOB        = "dead"
)

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
