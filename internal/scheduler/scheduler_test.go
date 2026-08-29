package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryStore struct {
	attempts []struct {
		kind Kind
		at   time.Time
	}
}

func (m *memoryStore) CountAttempts(_ context.Context, since time.Time, kind *Kind) (int, error) {
	n := 0
	for _, a := range m.attempts {
		if !a.at.Before(since) && (kind == nil || a.kind == *kind) {
			n++
		}
	}
	return n, nil
}
func (m *memoryStore) LastAttempt(_ context.Context, kind Kind) (time.Time, error) {
	var last time.Time
	for _, a := range m.attempts {
		if a.kind == kind && a.at.After(last) {
			last = a.at
		}
	}
	return last, nil
}
func (m *memoryStore) ReserveAttempt(_ context.Context, kind Kind, at time.Time) error {
	m.attempts = append(m.attempts, struct {
		kind Kind
		at   time.Time
	}{kind, at})
	return nil
}

func TestAutomaticQuotaPersistsAcrossSchedulerInstances(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	store := &memoryStore{}
	for i := 0; i < MaxAutomatic24H; i++ {
		s := Scheduler{Store: store, Now: func() time.Time { return now.Add(time.Duration(i) * time.Hour) }}
		if err := s.Reserve(context.Background(), Automatic); err != nil {
			t.Fatalf("reserve %d: %v", i, err)
		}
	}
	s := Scheduler{Store: store, Now: func() time.Time { return now.Add(5 * time.Hour) }}
	if !errors.Is(s.Reserve(context.Background(), Automatic), ErrAutomaticQuota) {
		t.Fatal("expected automatic quota")
	}
}

func TestManualCooldown(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	store := &memoryStore{}
	s := Scheduler{Store: store, Now: func() time.Time { return now }}
	if err := s.Reserve(context.Background(), Manual); err != nil {
		t.Fatal(err)
	}
	s.Now = func() time.Time { return now.Add(4 * time.Minute) }
	if err := s.Reserve(context.Background(), Manual); !errors.Is(err, ErrManualCooldown) {
		t.Fatal("expected cooldown")
	} else if got := err.Error(); got == ErrManualCooldown.Error() {
		t.Fatal("cooldown error should report the remaining wait")
	}
	s.Now = func() time.Time { return now.Add(5 * time.Minute) }
	if err := s.Reserve(context.Background(), Manual); err != nil {
		t.Fatalf("after cooldown: %v", err)
	}
}

func TestIntervalsAreBoundedAndPositive(t *testing.T) {
	for _, u := range []float64{0, .01, .5, .99, 1} {
		if got := NextInterval(2, u); got <= 0 {
			t.Fatalf("u=%v got=%v", u, got)
		}
		if got := RecoveryDelay(u); got < 15*time.Minute || got > time.Hour {
			t.Fatalf("recovery %v", got)
		}
	}
}
