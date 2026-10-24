package main

import (
	"testing"
	"time"
)

func TestDecideQueuesOnlyStalePendingPayments(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	events := []PaymentEvent{
		{ID: "p-old", Account: "acct-1", State: "pending", UpdatedAt: now.Add(-25 * time.Hour)},
		{ID: "p-new", Account: "acct-2", State: "pending", UpdatedAt: now.Add(-2 * time.Hour)},
		{ID: "p-paid", Account: "acct-3", State: "settled", UpdatedAt: now.Add(-48 * time.Hour)},
	}
	got := decide(events, now, 24*time.Hour)
	if len(got) != 1 || got[0].PaymentID != "p-old" || got[0].Action != "review" {
		t.Fatalf("unexpected notices: %#v", got)
	}
}
