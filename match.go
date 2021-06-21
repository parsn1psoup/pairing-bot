package main

import (
	"log"
)

// A MatchResult can either be a pair or a single unmatched odd one out
type MatchResult struct {
	first Recurser

	// second is none if the match result is not a pair
	second *Recurser
}

func (r *MatchResult) IsAPair() bool {
	return r.second != nil
}

func (r *MatchResult) Pair() (Recurser, Recurser) {
	return r.first, *r.second
}

func (r *MatchResult) OddOneOut() Recurser {
	return r.first
}

func Match(recursers []Recurser) []MatchResult {
	// if for some reason there's no matches today, we're done
	if len(recursers) == 0 {
		log.Println("No one was signed up to pair today -- so there were no matches")
		return nil
	}

	// shuffle our recursers. This will not error if the list is empty
	randSrc.Shuffle(len(recursers), func(i, j int) { recursers[i] = recursers[j] })

	var matches []MatchResult

	// if there's an odd number today, the last person in the list is an odd one out
	// put them in an odd one out "match", and then knock them off the list
	if len(recursers)%2 != 0 {
		matches = append(matches, MatchResult{
			first: recursers[len(recursers)-1],
		})
		recursers = recursers[:len(recursers)-1]
	}

	for i := 0; i < len(recursers); i += 2 {
		matches = append(matches, MatchResult{
			first:  recursers[i],
			second: &recursers[i+1],
		})
	}
	return matches
}
