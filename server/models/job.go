package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var ErrDuplicateJob = errors.New("job with the given name already exists in queue")

type Job struct {
	BaseModel
	Fails       int        `json:"fails"`
	Name        string     `json:"name"`
	Handler     string     `json:"handler"`
	Args        string     `json:"args"`
	LastError   string     `json:"last_error"`
	Claimed     bool       `json:"claimed" gorm:"default:false"`
	JobStatusID uint       `json:"job_status_id"`
	JobStatus   *JobStatus `json:"status"`
}

func (job *Job) MarkAsClaimed() (bool, error) {
	inProgressStatus := JobStatus{}
	err := db.Where(&JobStatus{Name: "in-progress"}).Find(&inProgressStatus).Error
	if err != nil {
		return false, err
	}

	res := db.Model(&Job{}).Where("id = ? AND claimed = ?", job.ID, false).Updates(map[string]interface{}{
		"claimed":       true,
		"job_status_id": inProgressStatus.ID,
	})

	if res.Error != nil {
		return false, res.Error
	}

	return res.RowsAffected > 0, nil
}

func (job *Job) Update(data map[string]interface{}) error {
	return db.Model(job).Updates(data).Error
}

func CreateUniqueJobByName(name string, handler string, args string) error {
	queuedJobStatuses := []JobStatus{}
	err := db.Where("name IN ('enqueued', 'in-progress')").Find(&queuedJobStatuses).Error
	if err != nil {
		return err
	}

	// Check to see if a job with the same name is either enqueued or in-progress
	// if one exists, return 'duplicate' error
	statusIDs := []uint{queuedJobStatuses[0].ID, queuedJobStatuses[1].ID}
	results := db.Where("name = ? AND job_status_id IN ?", name, statusIDs).First(&Job{})

	if results.Error != nil && !errors.Is(results.Error, gorm.ErrRecordNotFound) {
		return err
	}

	if results.RowsAffected > 0 {
		return ErrDuplicateJob
	}

	var enqueuedJobStatus JobStatus
	for _, jobStatus := range queuedJobStatuses {
		if jobStatus.Name == ENQUEUED_JOB {
			enqueuedJobStatus = jobStatus
			break
		}
	}

	// If a job with the given name already exists & is 'enqueued', do nothing
	return db.FirstOrCreate(&Job{
		Name:        name,
		Handler:     handler,
		Args:        args,
		JobStatusID: enqueuedJobStatus.ID,
	}, Job{Name: name, JobStatusID: enqueuedJobStatus.ID}).Error
}

func LastJob(status string, claimed bool) (*Job, error) {
	job := Job{}
	err := db.Joins("INNER JOIN job_statuses ON job_statuses.id = jobs.job_status_id AND job_statuses.name = ? AND claimed = ? ",
		status, claimed).Last(&job).Error
	if err != nil {
		return nil, err
	}

	return &job, nil
}

func FetchJobsByStatus(status string, page int) ([]Job, *Paging, error) {
	const JOIN_QUERY = "INNER JOIN job_statuses ON job_statuses.id = jobs.job_status_id AND job_statuses.name = ?"

	var total int64
	jobs := []Job{}

	err := db.Joins(JOIN_QUERY, status).Model(&Job{}).Count(&total).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	err = db.Scopes(paginate(page, MAX_PAGE_SIZE)).
		Preload("JobStatus").Order("jobs.id desc").
		Joins(JOIN_QUERY, status).Find(&jobs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	return jobs, newPaging(int64(page), MAX_PAGE_SIZE, total), nil
}

func FetchJobs(page int) ([]Job, *Paging, error) {
	var total int64
	jobs := []Job{}

	err := db.Model(&Job{}).Count(&total).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	err = db.Scopes(paginate(page, MAX_PAGE_SIZE)).
		Preload("JobStatus").Order("jobs.id desc").Find(&jobs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	return jobs, newPaging(int64(page), MAX_PAGE_SIZE, total), nil
}

func CurrentJobsStats() (*JobsStats, error) {
	const JOIN_QUERY = "INNER JOIN job_statuses ON job_statuses.id = jobs.job_status_id AND job_statuses.name = ?"
	stats := JobsStats{}

	err := db.Joins(JOIN_QUERY, ENQUEUED_JOB).Model(&Job{}).Count(&stats.EnueuedJobCount).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = db.Joins(JOIN_QUERY, IN_PROGRESS_JOB).Model(&Job{}).Count(&stats.InProgressJobCount).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = db.Joins(JOIN_QUERY, SUCCESSFUL_JOB).Model(&Job{}).Count(&stats.SuccessfulJobCount).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = db.Joins(JOIN_QUERY, DEAD_JOB).Model(&Job{}).Count(&stats.DeadJobCount).Error
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// LastJobLastUpdated returns the last job which was last updated 'arg1' minutes ago
// and is of 'arg2' status.
// i.e last record where job.updated_at + 'arg1' minutes <= 'now'.
//
// WARNING: THIS QUERY IS UNIQE TO SQLITE, REMEMBER TO UPDATE IT IF/WHEN
// OTHER SQL DATABASES ARE SUPPORTED
func LastJobLastUpdated(minutesAgo uint, status string) (*Job, error) {
	jobStatus := JobStatus{}
	err := db.Where(&JobStatus{Name: status}).Find(&jobStatus).Error
	if err != nil {
		return nil, err
	}

	job := Job{}
	err = db.Where(
		fmt.Sprintf("job_status_id = ? AND datetime(updated_at, '+%v minute') <= datetime('now')", minutesAgo),
		jobStatus.ID,
	).Last(&job).Error
	if err != nil {
		return nil, err
	}

	return &job, nil
}
