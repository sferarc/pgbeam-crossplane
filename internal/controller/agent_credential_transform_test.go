package controller

import (
	"testing"
	"time"

	"github.com/pgbeam/provider-pgbeam/apis/v1alpha1"
	pgbeam "go.pgbeam.com/sdk"
)

func timep(t time.Time) *time.Time { return &t }

func TestAgentCredentialObservation(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	lastUsed := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	authMethod := pgbeam.ScramSha256

	c := &pgbeam.AgentCredential{
		Id:         "agt_1",
		PgUsername: "agent_reporting",
		AuthMethod: &authMethod,
		LastUsedAt: timep(lastUsed),
		CreatedAt:  created,
		UpdatedAt:  updated,
	}

	obs := agentCredentialObservation(c)
	if obs.ID != "agt_1" {
		t.Errorf("ID = %q", obs.ID)
	}
	if obs.PgUsername != "agent_reporting" {
		t.Errorf("PgUsername = %q", obs.PgUsername)
	}
	if obs.AuthMethod != "scram-sha-256" {
		t.Errorf("AuthMethod = %q", obs.AuthMethod)
	}
	if obs.LastUsedAt != lastUsed.Format(time.RFC3339) {
		t.Errorf("LastUsedAt = %q", obs.LastUsedAt)
	}
	if obs.CreatedAt != created.Format(time.RFC3339) {
		t.Errorf("CreatedAt = %q", obs.CreatedAt)
	}
	if obs.UpdatedAt != updated.Format(time.RFC3339) {
		t.Errorf("UpdatedAt = %q", obs.UpdatedAt)
	}
}

func TestAgentCredentialObservation_NilOptionals(t *testing.T) {
	t.Parallel()

	c := &pgbeam.AgentCredential{
		Id:         "agt_2",
		PgUsername: "agent_min",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	obs := agentCredentialObservation(c)
	if obs.AuthMethod != "" {
		t.Errorf("AuthMethod = %q, want empty", obs.AuthMethod)
	}
	if obs.LastUsedAt != "" {
		t.Errorf("LastUsedAt = %q, want empty", obs.LastUsedAt)
	}
}

func TestIsAgentCredentialUpToDate(t *testing.T) {
	t.Parallel()

	active := &pgbeam.AgentCredential{Id: "agt_1", Status: pgbeam.AgentCredentialStatusActive}

	tests := []struct {
		name string
		fp   v1alpha1.AgentCredentialForProvider
		c    *pgbeam.AgentCredential
		want bool
	}{
		{
			name: "unset desired status is up to date",
			fp:   v1alpha1.AgentCredentialForProvider{},
			c:    active,
			want: true,
		},
		{
			name: "matching status is up to date",
			fp:   v1alpha1.AgentCredentialForProvider{Status: "active"},
			c:    active,
			want: true,
		},
		{
			name: "status drift",
			fp:   v1alpha1.AgentCredentialForProvider{Status: "disabled"},
			c:    active,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isAgentCredentialUpToDate(tt.fp, tt.c); got != tt.want {
				t.Errorf("isAgentCredentialUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}
