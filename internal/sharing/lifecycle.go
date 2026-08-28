package sharing

import "fmt"

type State string

const (
	NotAsked  State = "not_asked"
	Declined  State = "declined"
	Enabling  State = "enabling"
	Enabled   State = "enabled"
	Suspended State = "suspended"
	Revoking  State = "revoking"
	Revoked   State = "revoked"
	Error     State = "error"
)

type Machine struct {
	State State
}

func (m *Machine) Transition(next State) error {
	if m.State == next {
		return nil
	}
	if !validTransition(m.State, next) {
		return fmt.Errorf("invalid sharing transition %s -> %s", m.State, next)
	}
	m.State = next
	return nil
}

func validTransition(from, to State) bool {
	switch from {
	case NotAsked:
		return to == Declined || to == Enabling
	case Declined, Revoked:
		return to == Enabling
	case Enabling:
		return to == Enabled || to == Suspended || to == Error
	case Enabled:
		return to == Suspended || to == Revoking || to == Error
	case Suspended:
		return to == Enabled || to == Revoking || to == Error
	case Revoking:
		return to == Revoked || to == Error
	case Error:
		return to == Enabling || to == Revoking || to == Suspended
	default:
		return false
	}
}
