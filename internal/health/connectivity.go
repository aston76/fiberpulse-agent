package health

import (
	"errors"
	"time"
)

type ConnectivityState string

const (
	ConnectivityUnknown          ConnectivityState = "unknown"
	ConnectivityOffline          ConnectivityState = "offline"
	ConnectivityLocalOnly        ConnectivityState = "local_only"
	ConnectivityInternetDegraded ConnectivityState = "internet_degraded"
	ConnectivityInternetUsable   ConnectivityState = "internet_usable"
	ConnectivityUnstable         ConnectivityState = "unstable"
)

type ConnectivitySnapshot struct {
	State          ConnectivityState `json:"state"`
	Stable         ConnectivityState `json:"stable"`
	Candidate      ConnectivityState `json:"candidate,omitempty"`
	Consecutive    int               `json:"consecutive"`
	LastObservedAt time.Time         `json:"last_observed_at,omitempty"`
}

type ConnectivityMachine struct {
	state          ConnectivityState
	stable         ConnectivityState
	candidate      ConnectivityState
	consecutive    int
	lastObservedAt time.Time
}

func (m *ConnectivityMachine) State() ConnectivityState {
	if m.state == "" {
		return ConnectivityUnknown
	}
	return m.state
}

func (m *ConnectivityMachine) Observe(observed ConnectivityState, at time.Time) (ConnectivityState, error) {
	if !validObservedConnectivity(observed) {
		return m.State(), errors.New("invalid observed connectivity state")
	}
	if at.IsZero() {
		return m.State(), errors.New("connectivity observation time is required")
	}
	if m.state == "" {
		m.state = ConnectivityUnknown
		m.stable = ConnectivityUnknown
	}
	observedAt := at.UTC()
	if m.lastObservedAt.IsZero() || observedAt.After(m.lastObservedAt) {
		m.lastObservedAt = observedAt
	}
	if m.stable == ConnectivityUnknown && m.candidate == "" {
		if observed != ConnectivityUnknown {
			m.stable = observed
			m.state = observed
		}
		return m.state, nil
	}
	if observed == m.stable {
		m.state = m.stable
		m.candidate = ""
		m.consecutive = 0
		return m.state, nil
	}
	if m.candidate == observed {
		m.consecutive++
	} else {
		m.candidate = observed
		m.consecutive = 1
	}
	if m.consecutive >= 2 {
		m.stable = observed
		m.state = observed
		m.candidate = ""
		m.consecutive = 0
		return m.state, nil
	}
	m.state = ConnectivityUnstable
	return m.state, nil
}

func (m *ConnectivityMachine) Snapshot() ConnectivitySnapshot {
	return ConnectivitySnapshot{State: m.State(), Stable: m.stableState(), Candidate: m.candidate, Consecutive: m.consecutive, LastObservedAt: m.lastObservedAt}
}

func (m *ConnectivityMachine) Restore(snapshot ConnectivitySnapshot) error {
	if snapshot.State == "" {
		snapshot.State = ConnectivityUnknown
	}
	if snapshot.Stable == "" {
		snapshot.Stable = ConnectivityUnknown
	}
	if !validConnectivity(snapshot.State) || !validObservedConnectivity(snapshot.Stable) || (snapshot.Candidate != "" && !validObservedConnectivity(snapshot.Candidate)) || snapshot.Consecutive < 0 {
		return errors.New("invalid connectivity snapshot")
	}
	if snapshot.State == ConnectivityUnstable {
		if snapshot.Candidate == "" || snapshot.Consecutive != 1 || snapshot.Candidate == snapshot.Stable {
			return errors.New("invalid unstable connectivity snapshot")
		}
	} else if snapshot.Candidate != "" || snapshot.Consecutive != 0 || snapshot.State != snapshot.Stable {
		return errors.New("invalid stable connectivity snapshot")
	}
	m.state = snapshot.State
	m.stable = snapshot.Stable
	m.candidate = snapshot.Candidate
	m.consecutive = snapshot.Consecutive
	m.lastObservedAt = snapshot.LastObservedAt.UTC()
	return nil
}

func (m *ConnectivityMachine) stableState() ConnectivityState {
	if m.stable == "" {
		return ConnectivityUnknown
	}
	return m.stable
}

func validObservedConnectivity(state ConnectivityState) bool {
	switch state {
	case ConnectivityUnknown, ConnectivityOffline, ConnectivityLocalOnly, ConnectivityInternetDegraded, ConnectivityInternetUsable:
		return true
	default:
		return false
	}
}

func validConnectivity(state ConnectivityState) bool {
	return validObservedConnectivity(state) || state == ConnectivityUnstable
}
