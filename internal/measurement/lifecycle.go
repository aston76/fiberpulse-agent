package measurement

import "errors"

type TestState string

const (
	TestIdle          TestState = "idle"
	TestPreflight     TestState = "preflight"
	TestQuotaReserved TestState = "quota_reserved"
	TestLocate        TestState = "locate"
	TestDownload      TestState = "download"
	TestUpload        TestState = "upload"
	TestValidate      TestState = "validate"
	TestPersist       TestState = "persist"
	TestShareQueued   TestState = "share_queued"
	TestComplete      TestState = "complete"
	TestFailed        TestState = "failed"
	TestCancelled     TestState = "cancelled"
)

type TestMachine struct{ State TestState }

func (m *TestMachine) Transition(next TestState) error {
	if m.State == "" {
		m.State = TestIdle
	}
	allowed := map[TestState]map[TestState]bool{
		TestIdle:          {TestPreflight: true},
		TestPreflight:     {TestQuotaReserved: true, TestFailed: true, TestCancelled: true},
		TestQuotaReserved: {TestLocate: true, TestFailed: true, TestCancelled: true},
		TestLocate:        {TestDownload: true, TestValidate: true, TestFailed: true, TestCancelled: true},
		TestDownload:      {TestUpload: true, TestValidate: true, TestFailed: true, TestCancelled: true},
		TestUpload:        {TestValidate: true, TestFailed: true, TestCancelled: true},
		TestValidate:      {TestPersist: true, TestFailed: true, TestCancelled: true},
		TestPersist:       {TestShareQueued: true, TestComplete: true, TestFailed: true, TestCancelled: true},
		TestShareQueued:   {TestComplete: true, TestFailed: true, TestCancelled: true},
		TestComplete:      {TestIdle: true},
		TestFailed:        {TestIdle: true},
		TestCancelled:     {TestIdle: true},
	}
	if !allowed[m.State][next] {
		return errors.New("invalid measurement state transition")
	}
	m.State = next
	return nil
}
