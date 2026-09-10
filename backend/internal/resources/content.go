package resources

import (
	"strings"

	"github.com/google/uuid"
)

// normalizeAndValidateContent trims strings, fills in missing question / choice
// ids, and checks the payload makes sense for the type. It mutates and returns
// c so the caller stores the cleaned version.
func normalizeAndValidateContent(t Type, c Content) (Content, error) {
	switch t {
	case TypeMaterial:
		c.URL = strings.TrimSpace(c.URL)
		c.Description = strings.TrimSpace(c.Description)
		if c.FileAssetID == nil && c.URL == "" {
			return c, invalid("A material needs an uploaded file or a link.")
		}
		if c.URL != "" && !isHTTPURL(c.URL) {
			return c, invalid("The link must be a http(s) URL.")
		}
		return stripTo(t, c), nil

	case TypeArticle:
		c.Body = strings.TrimSpace(c.Body)
		if c.Body == "" {
			return c, invalid("An article needs some body text.")
		}
		return stripTo(t, c), nil

	case TypeWriting:
		c.Prompt = strings.TrimSpace(c.Prompt)
		c.Rubric = strings.TrimSpace(c.Rubric)
		if c.Prompt == "" {
			return c, invalid("A writing task needs a prompt.")
		}
		if c.MinWords < 0 {
			return c, invalid("Minimum words can't be negative.")
		}
		return stripTo(t, c), nil

	case TypeQuiz, TypeListening, TypeReading:
		if t == TypeListening && c.AudioAssetID == nil {
			return c, invalid("A listening task needs an uploaded audio file.")
		}
		if t == TypeReading {
			c.Passage = strings.TrimSpace(c.Passage)
			if c.Passage == "" {
				return c, invalid("A reading task needs a passage.")
			}
		}
		if len(c.Questions) == 0 {
			return c, invalid("Add at least one question.")
		}
		qs := make([]Question, len(c.Questions))
		for i, q := range c.Questions {
			cleaned, err := normalizeQuestion(q, i+1)
			if err != nil {
				return c, err
			}
			qs[i] = cleaned
		}
		c.Questions = qs
		return stripTo(t, c), nil

	default:
		return c, invalid("Unknown resource type.")
	}
}

func normalizeQuestion(q Question, n int) (Question, error) {
	q.Prompt = strings.TrimSpace(q.Prompt)
	if q.Prompt == "" {
		return q, invalid("Question %d has no prompt.", n)
	}
	if q.ID == "" {
		q.ID = uuid.NewString()
	}
	if q.Points <= 0 {
		q.Points = 1
	}

	switch q.Kind {
	case "single", "multi":
		if len(q.Choices) < 2 {
			return q, invalid("Question %d needs at least two choices.", n)
		}
		seen := map[string]bool{}
		for i := range q.Choices {
			q.Choices[i].Text = strings.TrimSpace(q.Choices[i].Text)
			if q.Choices[i].Text == "" {
				return q, invalid("Question %d has an empty choice.", n)
			}
			if q.Choices[i].ID == "" {
				q.Choices[i].ID = uuid.NewString()
			}
			seen[q.Choices[i].ID] = true
		}
		correct := dedupe(q.Correct)
		if len(correct) == 0 {
			return q, invalid("Mark the correct answer for question %d.", n)
		}
		if q.Kind == "single" && len(correct) != 1 {
			return q, invalid("Question %d is single-choice but has %d correct answers.", n, len(correct))
		}
		for _, id := range correct {
			if !seen[id] {
				return q, invalid("Question %d marks a correct answer that isn't one of its choices.", n)
			}
		}
		q.Correct = correct

	case "text":
		q.Choices = nil
		accepted := make([]string, 0, len(q.Correct))
		for _, a := range q.Correct {
			a = strings.TrimSpace(a)
			if a != "" {
				accepted = append(accepted, a)
			}
		}
		if len(accepted) == 0 {
			return q, invalid("Question %d needs at least one accepted answer.", n)
		}
		q.Correct = accepted

	default:
		return q, invalid("Question %d has an unknown kind %q.", n, q.Kind)
	}
	return q, nil
}

// stripTo zeroes the fields that don't belong to t, so the stored JSON only
// carries the payload for its type.
func stripTo(t Type, c Content) Content {
	out := Content{}
	switch t {
	case TypeMaterial:
		out.FileAssetID, out.URL, out.Description = c.FileAssetID, c.URL, c.Description
	case TypeArticle:
		out.Body = c.Body
	case TypeWriting:
		out.Prompt, out.MinWords, out.Rubric = c.Prompt, c.MinWords, c.Rubric
	case TypeQuiz:
		out.Questions = c.Questions
	case TypeListening:
		out.AudioAssetID, out.Questions = c.AudioAssetID, c.Questions
	case TypeReading:
		out.Passage, out.Questions = c.Passage, c.Questions
	}
	return out
}

func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
