package scheduler

import "fmt"

type State string

const (
	Disabled       State = "disabled"
	Waiting        State = "waiting"
	Due            State = "due"
	BlockedMetered State = "blocked_metered"
	BlockedQuota   State = "blocked_quota"
	DeferredBusy   State = "deferred_busy"
	Running        State = "running"
	Cooldown       State = "cooldown"
	Recovered      State = "recovered"
	Stopped        State = "stopped"
)

type Machine struct {
	State State
}

func (m *Machine) Transition(next State) error {
	if m.State == next {
		return nil
	}
	if !validTransition(m.State, next) {
		return fmt.Errorf("invalid scheduler transition %s -> %s", m.State, next)
	}
	m.State = next
	return nil
}

func (m Machine) CanBecomeDue() bool {
	switch m.State {
	case Waiting, BlockedMetered, BlockedQuota, DeferredBusy, Cooldown, Recovered:
		return true
	default:
		return false
	}
}

func validTransition(from, to State) bool {
	if from == "" {
		return to == Recovered || to == Stopped
	}
	if to == Stopped {
		return from != Stopped
	}
	if to == Disabled {
		return from != Stopped
	}
	switch from {
	case Disabled:
		return to == Waiting || to == Running || to == Recovered
	case Waiting:
		return to == Due || to == Recovered
	case Due:
		return to == BlockedMetered || to == BlockedQuota || to == DeferredBusy || to == Running || to == Waiting
	case BlockedMetered, BlockedQuota, DeferredBusy:
		return to == Due || to == Waiting || to == Recovered
	case Running:
		return to == Cooldown || to == Waiting
	case Cooldown:
		return to == Waiting || to == Due || to == Recovered
	case Recovered:
		return to == Waiting
	case Stopped:
		return to == Recovered
	default:
		return false
	}
}
