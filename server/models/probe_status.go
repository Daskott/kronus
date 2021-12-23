package models

const (
	PENDING_PROBE     = "pending"
	UNAVAILABLE_PROBE = "unavailable"
	BAD_PROBE         = "bad"
	GOOD_PROBE        = "good"
	CANCELLED_PROBE   = "cancelled"
)

type ProbeStatus struct {
	BaseModel
	Name   string
	Probes []Probe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func FindProbeStatus(name string) (*ProbeStatus, error) {
	probeStatus := ProbeStatus{}
	err := db.Select("id", "name").First(&probeStatus, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return &probeStatus, nil
}
