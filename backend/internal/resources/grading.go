package resources

import "strings"

// autoScore grades a set of answers against a quiz-like resource's questions.
// answers maps question id -> the student's answer: chosen choice ids for
// single/multi, or a single-element slice holding the free-text answer for
// text. Each question is all-or-nothing (no partial credit): it always adds
// q.Points to max, and adds q.Points to score only on an exact match.
func autoScore(content Content, answers map[string][]string) (score, max int) {
	for _, q := range content.Questions {
		max += q.Points
		if questionCorrect(q, answers[q.ID]) {
			score += q.Points
		}
	}
	return score, max
}

// questionCorrect compares a student's answer to one question's key.
func questionCorrect(q Question, given []string) bool {
	if q.Kind == "text" {
		if len(given) == 0 {
			return false
		}
		answer := strings.ToLower(strings.TrimSpace(given[0]))
		if answer == "" {
			return false
		}
		for _, accepted := range q.Correct {
			if strings.ToLower(strings.TrimSpace(accepted)) == answer {
				return true
			}
		}
		return false
	}
	// single / multi: the given choice-id set must equal the correct set
	// exactly (dedupe defends against a client sending a choice twice).
	return sameSet(dedupe(given), q.Correct)
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	want := make(map[string]bool, len(b))
	for _, v := range b {
		want[v] = true
	}
	for _, v := range a {
		if !want[v] {
			return false
		}
	}
	return true
}
