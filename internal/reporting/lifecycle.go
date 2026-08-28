package reporting

import (
	"errors"
	"time"
)

type State string

const (
	Drafting  State = "drafting"
	Ready     State = "ready"
	Exporting State = "exporting"
	Exported  State = "exported"
	Failed    State = "failed"
	Deleted   State = "deleted"
)

type Record struct {
	ID          string    `json:"id"`
	Format      string    `json:"format"`
	State       State     `json:"state"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	ByteCount   int64     `json:"byte_count"`
	ErrorCode   string    `json:"error_code,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Machine struct{ State State }

func (m *Machine) Transition(next State) error {
	if m.State == "" {
		m.State = Drafting
	}
	allowed := map[State]map[State]bool{
		Drafting:  {Ready: true, Failed: true},
		Ready:     {Exporting: true, Deleted: true},
		Exporting: {Exported: true, Failed: true},
		Exported:  {Deleted: true},
		Failed:    {Deleted: true},
		Deleted:   {},
	}
	if !allowed[m.State][next] {
		return errors.New("invalid report state transition")
	}
	m.State = next
	return nil
}

func ValidState(state State) bool {
	switch state {
	case Drafting, Ready, Exporting, Exported, Failed, Deleted:
		return true
	default:
		return false
	}
}
