package main

import "time"

type PaymentEvent struct {
	ID        string    `json:"id"`
	Account   string    `json:"account"`
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"updated_at"`
}
type AuditNotice struct {
	PaymentID string `json:"payment_id"`
	Account   string `json:"account"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
}

func stale(e PaymentEvent, now time.Time, age time.Duration) bool {
	return e.State == "pending" && now.Sub(e.UpdatedAt) >= age
}

func decide(events []PaymentEvent, now time.Time, age time.Duration) []AuditNotice {
	out := make([]AuditNotice, 0)
	for _, e := range events {
		if stale(e, now, age) {
			out = append(out, AuditNotice{e.ID, e.Account, "review", "pending payment exceeded sweep age"})
		}
	}
	return out
}
