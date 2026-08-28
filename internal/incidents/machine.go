package incidents

import "time"

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
