package models

import "time"

type Probe struct {
	BaseModel
	Response       string
	RetryCount     int
	EmergencyProbe EmergencyProbe
	UserID         uint `gorm:"not null"`
	ProbeStatusID  uint
}

func (probe *Probe) Save() error {
	return db.Save(&probe).Error
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
