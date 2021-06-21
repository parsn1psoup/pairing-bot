package main

import "testing"

var tableIsPair = []struct {
	testName string
	match    MatchResult
	want     bool
}{
	{"is_not_pair", MatchResult{first: &Recurser{}}, false},
	{"is_pair", MatchResult{first: &Recurser{}, second: &Recurser{}}, true},
}

func TestMatchResult_IsPair(t *testing.T) {
	for _, tt := range tableIsPair {
		t.Run(tt.testName, func(t *testing.T) {
			got := tt.match.IsPair()
			if got != tt.want {
				t.Errorf("got %v, wanted %v\n", got, tt.want)
			}
		})
	}
}
