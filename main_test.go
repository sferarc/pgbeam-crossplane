package main

import (
	"errors"
	"flag"
	"io"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    options
		wantErr bool
	}{
		{
			name: "defaults",
			want: options{SyncPeriod: time.Hour, PollInterval: time.Minute, MaxReconcileRate: 10},
		},
		{
			name: "overrides",
			args: []string{"--debug", "--sync=30m", "--poll", "2m", "--max-reconcile-rate=5", "--leader-election"},
			want: options{Debug: true, SyncPeriod: 30 * time.Minute, PollInterval: 2 * time.Minute, MaxReconcileRate: 5, LeaderElection: true},
		},
		{name: "zero poll", args: []string{"--poll=0s"}, wantErr: true},
		{name: "negative rate", args: []string{"--max-reconcile-rate=-1"}, wantErr: true},
		{name: "unknown flag", args: []string{"--nope"}, wantErr: true},
		{name: "positional argument", args: []string{"extra"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOptions("provider", tt.args, io.Discard)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseOptions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseOptionsHelp(t *testing.T) {
	if _, err := parseOptions("provider", []string{"--help"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("parseOptions(--help) error = %v, want flag.ErrHelp", err)
	}
}
