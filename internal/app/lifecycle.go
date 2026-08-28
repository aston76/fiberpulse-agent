package app

import "fmt"

// LifecycleState is the single authoritative state exposed by the agent. The
// persisted pause preference remains separate because consent_required must
// take precedence in the UI without forgetting that preference.
type LifecycleState string

const (
	LifecycleStarting        LifecycleState = "starting"
	LifecycleConsentRequired LifecycleState = "consent_required"
	LifecycleMonitoring      LifecycleState = "monitoring"
	LifecyclePaused          LifecycleState = "paused"
	LifecycleDegraded        LifecycleState = "degraded"
	LifecycleStopping        LifecycleState = "stopping"
	LifecycleStopped         LifecycleState = "stopped"
	LifecycleFailed          LifecycleState = "failed"
)

type LifecycleMachine struct {
	State LifecycleState
}

func (m *LifecycleMachine) Transition(next LifecycleState) error {
	if m.State == next {
		return nil
	}
	if !validLifecycleTransition(m.State, next) {
		return fmt.Errorf("invalid agent lifecycle transition %s -> %s", m.State, next)
	}
	m.State = next
	return nil
}

func validLifecycleTransition(from, to LifecycleState) bool {
	if to == LifecycleStopping {
		return from != LifecycleStopped && from != LifecycleStopping
	}
	switch from {
	case LifecycleStarting:
		return to == LifecycleConsentRequired || to == LifecycleMonitoring || to == LifecyclePaused || to == LifecycleFailed
	case LifecycleConsentRequired:
		return to == LifecycleMonitoring || to == LifecyclePaused || to == LifecycleFailed
	case LifecycleMonitoring:
		return to == LifecycleConsentRequired || to == LifecyclePaused || to == LifecycleDegraded || to == LifecycleFailed
	case LifecyclePaused:
		return to == LifecycleConsentRequired || to == LifecycleMonitoring || to == LifecycleFailed
	case LifecycleDegraded:
		return to == LifecycleConsentRequired || to == LifecycleMonitoring || to == LifecyclePaused || to == LifecycleFailed
	case LifecycleStopping:
		return to == LifecycleStopped || to == LifecycleFailed
	default:
		return false
	}
}
