package scheduler

import (
	"context"
	"errors"
	"math"
	"time"
)

type Kind string

const (
	Automatic Kind = "automatic"
	Manual    Kind = "manual"

	MaxAutomatic24H = 4
	MaxTotal24H     = 8
	ManualCooldown  = 30 * time.Minute
)

var (
	ErrAutomaticQuota = errors.New("automatic 24-hour quota exhausted")
	ErrTotalQuota     = errors.New("total 24-hour quota exhausted")
	ErrManualCooldown = errors.New("manual test cooldown is active")
)

type AttemptStore interface {
	CountAttempts(context.Context, time.Time, *Kind) (int, error)
	LastAttempt(context.Context, Kind) (time.Time, error)
	ReserveAttempt(context.Context, Kind, time.Time) error
}

type Scheduler struct {
	Store AttemptStore
	Now   func() time.Time
}

func (s Scheduler) Reserve(ctx context.Context, kind Kind) error {
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	since := now.Add(-24 * time.Hour)
	total, err := s.Store.CountAttempts(ctx, since, nil)
	if err != nil {
		return err
	}
	if total >= MaxTotal24H {
		return ErrTotalQuota
	}
	if kind == Automatic {
		autoKind := Automatic
		automatic, err := s.Store.CountAttempts(ctx, since, &autoKind)
		if err != nil {
			return err
		}
		if automatic >= MaxAutomatic24H {
			return ErrAutomaticQuota
		}
	} else {
		last, err := s.Store.LastAttempt(ctx, Manual)
		if err != nil {
			return err
		}
		if !last.IsZero() && now.Sub(last) < ManualCooldown {
			return ErrManualCooldown
		}
	}
	return s.Store.ReserveAttempt(ctx, kind, now)
}

func NextInterval(ratePerDay, unitRandom float64) time.Duration {
	if ratePerDay < 1 {
		ratePerDay = 1
	}
	if ratePerDay > MaxAutomatic24H {
		ratePerDay = MaxAutomatic24H
	}
	if unitRandom <= 0 {
		unitRandom = 1e-9
	}
	if unitRandom >= 1 {
		unitRandom = math.Nextafter(1, 0)
	}
	hours := -math.Log(1-unitRandom) * 24 / ratePerDay
	interval := time.Duration(hours * float64(time.Hour))
	if interval < time.Second {
		return time.Second
	}
	return interval
}

func RecoveryDelay(unitRandom float64) time.Duration {
	if unitRandom < 0 {
		unitRandom = 0
	}
	if unitRandom > 1 {
		unitRandom = 1
	}
	return 15*time.Minute + time.Duration(unitRandom*45*float64(time.Minute))
}
