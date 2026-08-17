package controller

import (
	"testing"
	"time"

	pgbeam "go.pgbeam.com/sdk"
)

// TestSelfHostEnrollmentObservation covers the observation mapper for the
// immutable, wrapped-create SelfHostEnrollment resource, including the nullable
// last_seen_at / revoked_at timestamps (the inline immutable observation could
// not handle these, which is why this resource uses the mapper).
func TestSelfHostEnrollmentObservation(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	lastSeen := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	revoked := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	createdBy := "usr_1"

	e := &pgbeam.SelfHostEnrollment{
		Id:         "she_1",
		OrgId:      "org_1",
		CreatedBy:  &createdBy,
		CreatedAt:  created,
		LastSeenAt: timep(lastSeen),
		RevokedAt:  timep(revoked),
	}

	obs := selfHostEnrollmentObservation(e)
	if obs.ID != "she_1" {
		t.Errorf("ID = %q", obs.ID)
	}
	if obs.CreatedBy != "usr_1" {
		t.Errorf("CreatedBy = %q", obs.CreatedBy)
	}
	if obs.CreatedAt != created.Format(time.RFC3339) {
		t.Errorf("CreatedAt = %q", obs.CreatedAt)
	}
	if obs.LastSeenAt != lastSeen.Format(time.RFC3339) {
		t.Errorf("LastSeenAt = %q", obs.LastSeenAt)
	}
	if obs.RevokedAt != revoked.Format(time.RFC3339) {
		t.Errorf("RevokedAt = %q", obs.RevokedAt)
	}
}

// TestSelfHostEnrollmentObservation_NilOptionals verifies an enrollment that
// has never been used (nil created_by, last_seen_at, revoked_at) maps without
// panicking.
func TestSelfHostEnrollmentObservation_NilOptionals(t *testing.T) {
	t.Parallel()

	e := &pgbeam.SelfHostEnrollment{
		Id:        "she_2",
		OrgId:     "org_1",
		CreatedAt: time.Now(),
	}
	obs := selfHostEnrollmentObservation(e)
	if obs.CreatedBy != "" {
		t.Errorf("CreatedBy = %q, want empty", obs.CreatedBy)
	}
	if obs.LastSeenAt != "" {
		t.Errorf("LastSeenAt = %q, want empty", obs.LastSeenAt)
	}
	if obs.RevokedAt != "" {
		t.Errorf("RevokedAt = %q, want empty", obs.RevokedAt)
	}
}
