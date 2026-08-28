package incidents

import (
	"errors"
	"time"
)

type State string

const (
	None       State = "none"
	Suspected  State = "suspected"
	Active     State = "active"
	Recovering State = "recovering"
	Resolved   State = "resolved"
	Dismissed  State = "dismissed"
)

type Observation struct {
	At       time.Time
	Degraded bool
	Category string
}

type Record struct {
	ID           string    `json:"id"`
	Category     string    `json:"category"`
	State        State     `json:"state"`
	SuspectedAt  time.Time `json:"suspected_at"`
	ActiveAt     time.Time `json:"active_at,omitempty,omitzero"`
	RecoveringAt time.Time `json:"recovering_at,omitempty,omitzero"`
	ResolvedAt   time.Time `json:"resolved_at,omitempty,omitzero"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Snapshot contains every piece of hysteresis state needed to resume incident
// detection without treating a restart as fresh evidence.
type Snapshot struct {
	State           State     `json:"state"`
	Category        string    `json:"category,omitempty"`
	SuspectedAt     time.Time `json:"suspected_at,omitempty"`
	RecoveringAt    time.Time `json:"recovering_at,omitempty"`
	ConsecutiveBad  int       `json:"consecutive_bad"`
	ConsecutiveGood int       `json:"consecutive_good"`
	Recent          []bool    `json:"recent"`
}

type Machine struct {
	State           State
	Category        string
	SuspectedAt     time.Time
	RecoveringAt    time.Time
	consecutiveBad  int
	consecutiveGood int
	recent          []bool
}

func (m *Machine) Observe(o Observation) State {
	if m.State == "" {
		m.State = None
	}
	m.recent = append(m.recent, o.Degraded)
	if len(m.recent) > 3 {
		m.recent = m.recent[len(m.recent)-3:]
	}
	if o.Degraded {
		m.consecutiveBad++
		m.consecutiveGood = 0
	} else {
		m.consecutiveGood++
		m.consecutiveBad = 0
	}
	bad := 0
	for _, v := range m.recent {
		if v {
			bad++
		}
	}
	switch m.State {
	case None, Resolved:
		if bad >= 2 {
			m.State = Suspected
			m.SuspectedAt = o.At
			m.Category = o.Category
		}
	case Suspected:
		if m.consecutiveBad >= 3 || (o.Degraded && o.At.Sub(m.SuspectedAt) >= 5*time.Minute) {
			m.State = Active
		}
		if !o.Degraded && bad < 2 {
			m.State = None
			m.Category = ""
		}
	case Active:
		if m.consecutiveGood >= 3 {
			m.State = Recovering
			m.RecoveringAt = o.At
		}
	case Recovering:
		if o.Degraded {
			m.State = Active
			m.RecoveringAt = time.Time{}
		}
		if !o.Degraded && o.At.Sub(m.RecoveringAt) >= 15*time.Minute {
			m.State = Resolved
		}
	case Dismissed:
	}
	return m.State
}

func (m *Machine) Dismiss() {
	if m.State == Active {
		m.State = Dismissed
	}
}

func (m *Machine) Snapshot() Snapshot {
	return Snapshot{
		State:           m.State,
		Category:        m.Category,
		SuspectedAt:     m.SuspectedAt,
		RecoveringAt:    m.RecoveringAt,
		ConsecutiveBad:  m.consecutiveBad,
		ConsecutiveGood: m.consecutiveGood,
		Recent:          append([]bool(nil), m.recent...),
	}
}

func (m *Machine) Restore(snapshot Snapshot) error {
	if snapshot.State == "" {
		snapshot.State = None
	}
	switch snapshot.State {
	case None, Suspected, Active, Recovering, Resolved, Dismissed:
	default:
		return errors.New("invalid incident state")
	}
	if len(snapshot.Recent) > 3 || snapshot.ConsecutiveBad < 0 || snapshot.ConsecutiveGood < 0 {
		return errors.New("invalid incident hysteresis snapshot")
	}
	if snapshot.State != None && snapshot.State != Resolved && snapshot.State != Dismissed && snapshot.Category == "" {
		return errors.New("active incident state requires a category")
	}
	m.State = snapshot.State
	m.Category = snapshot.Category
	m.SuspectedAt = snapshot.SuspectedAt
	m.RecoveringAt = snapshot.RecoveringAt
	m.consecutiveBad = snapshot.ConsecutiveBad
	m.consecutiveGood = snapshot.ConsecutiveGood
	m.recent = append(m.recent[:0], snapshot.Recent...)
	return nil
}
