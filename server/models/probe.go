package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Probe struct {
	BaseModel
	LastResponse   string
	RetryCount     int
	EmergencyProbe EmergencyProbe
	UserID         uint `gorm:"not null"`
	ProbeStatusID  uint
}

var ProbeStatusMapToResponse = map[string]map[string]bool{
	GOOD_PROBE: {"yes": true, "yeah": true, "yh": true, "y": true},
	BAD_PROBE:  {"no": true, "nope": true, "nah": true, "na": true, "n": true},
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

func ProbesByStatus(status string) ([]Probe, error) {
	probes := []Probe{}

	err := db.Joins(
		"INNER JOIN probe_statuses ON probe_statuses.id = probes.probe_status_id AND probe_statuses.name = ?", status).
		Find(&probes).Error

	if err != nil {
		return nil, err
	}

	return probes, err
}

func FindProbe(id interface{}) (*Probe, error) {
	probe := Probe{}
	err := db.First(&probe, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return &probe, nil
}

func CreateProbe(userID interface{}) error {
	currentTime := time.Now()
	pendingProbeStatus := ProbeStatus{}
	err := db.Where(&ProbeStatus{Name: "pending"}).Find(&pendingProbeStatus).Error
	if err != nil {
		return err
	}

	return db.Model(&Probe{}).Create(map[string]interface{}{
		"user_id":         userID,
		"probe_status_id": pendingProbeStatus.ID,
		"created_at":      currentTime,
		"updated_at":      currentTime,
	}).Error
}
