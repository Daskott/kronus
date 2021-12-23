package models

type Contact struct {
	BaseModel
	FirstName          string           `json:"first_name" validate:"required"`
	LastName           string           `json:"last_name" validate:"required"`
	PhoneNumber        string           `json:"phone_number" validate:"required,e164" gorm:"not null;unique"`
	Email              string           `json:"email" validate:"required,email" gorm:"unique"`
	IsEmergencyContact bool             `json:"is_emergency_contact"`
	UserID             uint             `json:"user_id" gorm:"not null"`
	EmergencyProbes    []EmergencyProbe `json:"emergency_probes,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
