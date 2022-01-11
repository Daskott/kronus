package models

const (
	PENDING_PROBE     = "pending"
	BAD_PROBE         = "bad"
	GOOD_PROBE        = "good"
	CANCELLED_PROBE   = "cancelled"
	UNAVAILABLE_PROBE = "unavailable"
)

var ProbeStatusNameMap = map[string]bool{
	PENDING_PROBE:     true,
	BAD_PROBE:         true,
	GOOD_PROBE:        true,
	CANCELLED_PROBE:   true,
	UNAVAILABLE_PROBE: true,
}

type ProbeStats struct {
	PendingProbeCount     int64 `json:"pending_probe_count"`
	BadProbeCount         int64 `json:"bad_probe_count"`
	GoodProbeCount        int64 `json:"good_probe_count"`
	CancelledProbeCount   int64 `json:"cancelled_probe_count"`
	UnavailableProbeCount int64 `json:"unavailable_probe_count"`
}

type ProbeStatus struct {
	BaseModel
	Name   string  `json:"name"`
	Probes []Probe `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func FindProbeStatus(name string) (*ProbeStatus, error) {
	probeStatus := ProbeStatus{}
	err := db.Select("id", "name").First(&probeStatus, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return &probeStatus, nil
}
