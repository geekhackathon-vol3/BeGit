package service

import (
	"testing"
	"time"
)

func TestNextBeGitTimeRunUsesConfiguredTimezone(t *testing.T) {
	after := time.Date(2026, 10, 16, 23, 59, 0, 0, time.UTC) // Saturday 08:59 in Tokyo
	got, err := NextBeGitTimeRun(after, "09:00", "Asia/Tokyo", []int{6})
	if err != nil {
		t.Fatalf("NextBeGitTimeRun() failed: %v", err)
	}
	want := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestNextBeGitTimeRunSkipsPastOccurrence(t *testing.T) {
	after := time.Date(2026, 10, 19, 0, 1, 0, 0, time.UTC) // Monday 09:01 in Tokyo
	got, err := NextBeGitTimeRun(after, "09:00", "Asia/Tokyo", []int{1, 3})
	if err != nil {
		t.Fatalf("NextBeGitTimeRun() failed: %v", err)
	}
	want := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}
