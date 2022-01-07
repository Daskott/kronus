package models

import "errors"

var (
	ErrDuplicateContactEmail  = errors.New("contact with the same 'email' already exist")
	ErrDuplicateContactNumber = errors.New("contact with the same 'phone_number' already exist")
)

type Contact struct {
	BaseModel
	FirstName          string           `json:"first_name" validate:"required"`
	LastName           string           `json:"last_name" validate:"required"`
	PhoneNumber        string           `json:"phone_number" validate:"required,e164" gorm:"index:idx_user_id_phone_number,priority:2;not null"`
	Email              string           `json:"email" validate:"required,email" gorm:"index:idx_user_id_email,priority:2;not null"`
	UserID             uint             `json:"user_id" gorm:"index:idx_user_id_email,priority:1,unique;index:idx_user_id_phone_number,priority:1,unique;not null"`
	IsEmergencyContact bool             `json:"is_emergency_contact"`
	EmergencyProbes    []EmergencyProbe `json:"emergency_probes,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
