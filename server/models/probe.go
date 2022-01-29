package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Probe struct {
	BaseModel
	LastResponse   string          `json:"last_response"`
	RetryCount     int             `json:"retry_count"`
	EmergencyProbe *EmergencyProbe `json:"emergency_probe,omitempty"`
	UserID         uint            `json:"user_id" gorm:"not null"`
	ProbeStatusID  uint            `json:"probe_status_id"`
	ProbeStatus    *ProbeStatus    `json:"status"`

	// TODO: Remove defaults later & set fields to "not null"
	MaxRetries        int `json:"max_retries" gorm:"default:3"`
	WaitTimeInMinutes int `json:"wait_time_in_minutes" gorm:"default:60"`
}

var ProbeStatusMapToResponse = map[string]map[string]bool{
	GOOD_PROBE: {"yes": true, "yeah": true, "yh": true, "y": true},
	BAD_PROBE:  {"no": true, "nope": true, "nah": true, "na": true, "n": true},
}

func (probe *Probe) Update(data map[string]interface{}) error {
	return db.Model(probe).Updates(data).Error
}

func (probe *Probe) Save() error {
	return db.Save(&probe).Error
}

func (probe *Probe) IsPending() (bool, error) {
	probeStatus := ProbeStatus{}

	// If no record, then probe isn't pending
	err := db.Where("id = ? AND name = ?", probe.ProbeStatusID, PENDING_PROBE).First(&probeStatus).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}

	// return false on error if it's not a 'ErrRecordNotFound'
	if err != nil {
		return false, err
	}

	return true, nil
}

// StatusFromLastResponse returns the derived probe 'status' (i.e. 'good', 'bad', or '')
// based on 'LastResponse'(i.e. the linked user's last response) for the current probe
func (probe *Probe) StatusFromLastResponse() string {
	status := ""
	for key, probeResp := range ProbeStatusMapToResponse {
		if probeResp[strings.TrimSpace(strings.ToLower(probe.LastResponse))] {
			status = key
			break
		}
	}
	return status
}

func SetProbeStatus(probeID interface{}, status string) error {
	probeStatus := ProbeStatus{}

	err := db.First(&probeStatus, &ProbeStatus{Name: status}).Error
	if err != nil {
		return err
	}

	err = db.Model(&Probe{}).Where("id = ?", probeID).Update("probe_status_id", probeStatus.ID).Error
	if err != nil {
		return err
	}

	return nil
}

func FetchProbesByStatus(status, order string, page int) ([]Probe, *Paging, error) {
	const JOIN_QUERY = "INNER JOIN probe_statuses ON probe_statuses.id = probes.probe_status_id AND probe_statuses.name = ?"

	var total int64
	probes := []Probe{}

	err := db.Joins(JOIN_QUERY, status).Model(&Probe{}).Count(&total).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	err = db.Scopes(paginate(page, MAX_PAGE_SIZE)).
		Preload("ProbeStatus").
		Preload("EmergencyProbe").
		Order(fmt.Sprintf("probes.id %v", order)).Joins(JOIN_QUERY, status).Find(&probes).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	return probes, newPaging(int64(page), MAX_PAGE_SIZE, total), nil
}

func FetchProbes(page int, query interface{}, args ...interface{}) ([]Probe, *Paging, error) {
	var total int64
	probes := []Probe{}

	countQuery := db.Model(&Probe{})
	probeQuery := db.Scopes(paginate(page, MAX_PAGE_SIZE)).
		Preload("ProbeStatus").Preload("EmergencyProbe").Order("probes.id desc")

	if query != nil && args != nil {
		countQuery = countQuery.Where(query, args)
		probeQuery = probeQuery.Where(query, args)
	}

	err := countQuery.Count(&total).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	err = probeQuery.Find(&probes).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	return probes, newPaging(int64(page), MAX_PAGE_SIZE, total), nil
}

func FindProbe(id interface{}) (*Probe, error) {
	probe := Probe{}
	err := db.First(&probe, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &probe, nil
}

func CreateProbe(userID interface{}, waitTimeInMinutes, maxRetries int) error {
	currentTime := time.Now()
	pendingProbeStatus := ProbeStatus{}
	err := db.Where(&ProbeStatus{Name: "pending"}).Find(&pendingProbeStatus).Error
	if err != nil {
		return err
	}

	return db.Model(&Probe{}).Create(map[string]interface{}{
		"user_id":              userID,
		"probe_status_id":      pendingProbeStatus.ID,
		"wait_time_in_minutes": waitTimeInMinutes,
		"max_retries":          maxRetries,
		"created_at":           currentTime,
		"updated_at":           currentTime,
	}).Error
}

func CurrentProbeStats() (*ProbeStats, error) {
	const JOIN_QUERY = "INNER JOIN probe_statuses ON probe_statuses.id = probes.probe_status_id AND probe_statuses.name = ?"
	stats := ProbeStats{}

	err := db.Joins(JOIN_QUERY, PENDING_PROBE).Model(&Probe{}).Count(&stats.PendingProbeCount).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = db.Joins(JOIN_QUERY, GOOD_PROBE).Model(&Probe{}).Count(&stats.GoodProbeCount).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = db.Joins(JOIN_QUERY, BAD_PROBE).Model(&Probe{}).Count(&stats.BadProbeCount).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = db.Joins(JOIN_QUERY, CANCELLED_PROBE).Model(&Probe{}).Count(&stats.CancelledProbeCount).Error
	if err != nil {
		return nil, err
	}

	err = db.Joins(JOIN_QUERY, UNAVAILABLE_PROBE).Model(&Probe{}).Count(&stats.UnavailableProbeCount).Error
	if err != nil {
		return nil, err
	}

	return &stats, nil
}
