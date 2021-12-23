package models

import "time"

type EmergencyProbe struct {
	BaseModel
	ContactID uint `json:"contact_id"`
	ProbeID   uint `json:"probe_id"`
}

func CreateEmergencyProbe(probeID, contactID interface{}) error {
	currentTime := time.Now()
	return db.Model(&EmergencyProbe{}).Create(map[string]interface{}{
		"probe_id":   probeID,
		"contact_id": contactID,
		"created_at": currentTime,
		"updated_at": currentTime,
	}).Error
}
