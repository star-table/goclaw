package model

// ActiveHoursConfig represents optional active window for heartbeat
type ActiveHoursConfig struct {
	Start string `json:"start"` // e.g. "08:00"
	End   string `json:"end"`   // e.g. "22:00"
}

// HeartbeatConfig represents heartbeat configuration (matches Python format)
type HeartbeatConfig struct {
	Enabled     bool              `json:"enabled"`
	Every       string            `json:"every"`                  // e.g. "6h", "30m"
	Target      string            `json:"target"`                 // e.g. "main", "last"
	ActiveHours *ActiveHoursConfig `json:"active_hours,omitempty"` // optional active window
}

// DefaultHeartbeatConfig returns default heartbeat configuration
func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Enabled: false,
		Every:   "6h",
		Target:  "main",
		ActiveHours: &ActiveHoursConfig{
			Start: "08:00",
			End:   "22:00",
		},
	}
}
