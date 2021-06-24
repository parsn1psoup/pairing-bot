package main

import (
	"context"
	"testing"
)

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

// TODO more test cases?
var tableMatch = []struct {
	testName string
	input    []Recurser
	want     int
}{
	{"empty_list", nil, 0}, // the length of a nil slice is 0
	{"single_recurser", []Recurser{{email: "test@you.com"}}, 1},
	{"two_recursers", []Recurser{{email: "test@you.com"}, {email: "me@moon.com"}}, 1},
}

func TestMatch_Length(t *testing.T) {
	for _, tt := range tableMatch {
		t.Run(tt.testName, func(t *testing.T) {
			got := len(Match(tt.input))
			if got != tt.want {
				t.Errorf("got %v, wanted %v\n", got, tt.want)
			}
		})
	}
}

// 1. everyone in input is in output exactly once
// 2. 0 or 1 odd one out
func TestMatch(t *testing.T) {

}

func setupPairingLogic() *PairingLogic {

	pl := &PairingLogic{
		rdb: &MockRecurserDB{
			unsetSkippingTomorrowCalled: 0,
			lenListSkippingTomorrow:     0,
		},
		adb: &MockAPIAuthDB{},
		ur:  &mockUserRequest{},
		un:  &mockUserNotification{},
	}

	return pl
}

// TestMatch_ResetSkippers checks whether for a skippersList with n items produced by ListSkippingTomorrow,
// UnsetSkippingTomorrow is called n times
func TestMatch_ResetSkippers(t *testing.T) {
	pl := setupPairingLogic()
	ctx := context.Background()

	pl.resetSkippers(ctx)

	// use type assertion so struct fields can be accessed
	mrdb, ok := pl.rdb.(*MockRecurserDB)
	if !ok {
		t.Error("Failed type assertion\n")
	}
	numSkippers := mrdb.lenListSkippingTomorrow
	numUnset := mrdb.unsetSkippingTomorrowCalled
	if numSkippers != numUnset {
		t.Errorf("TestMatch_ResetSkippers: numSkippers is %v, numUnset is %v\n", numSkippers, numUnset)
	}
}

var tableNotify = []struct {
	testName string
	input    []MatchResult
	want     int
}{
	{"empty_list", nil, 0}, // the length of a nil slice is 0
	{"single_match", []MatchResult{{first: &Recurser{email: "test@you.com"}}}, 1},
	{"two_matches", []MatchResult{{first: &Recurser{email: "test@you.com"}},
		{first: &Recurser{email: "me@moon.com"}, second: &Recurser{email: "hello@moon.com"}}}, 2},
}

func TestPairingLogic_notifyMatches(t *testing.T) {
	for _, tt := range tableNotify {
		t.Run(tt.testName, func(t *testing.T) {
			pl := setupPairingLogic()
			ctx := context.Background()

			notifier, ok := pl.un.(*mockUserNotification)
			if !ok {
				t.Error("Failed type assertion\n")
			}

			pl.notifyMatches(ctx, tt.input)

			got := notifier.sendUserMessageCalled
			if got != tt.want {
				t.Errorf("got %v, wanted %v\n", got, tt.want)
			}
		})
	}
}
