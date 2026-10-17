package main

import "testing"

func TestReceiptEvent(t *testing.T) {
	cases := []struct {
		name string
		in   receipt
		want string
	}{
		{"small gift", receipt{Donor: "Amina", Amount: 2500, Receipt: "r-17"}, "receipt r-17 recorded for Amina (2500 cents)"},
		{"grant", receipt{Donor: "Kai", Amount: 100000, Receipt: "r-18"}, "receipt r-18 recorded for Kai (100000 cents)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := receiptEvent(tc.in); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
