package resources

import (
	"strings"
	"testing"
)

func TestValidateContent_Material(t *testing.T) {
	if _, err := normalizeAndValidateContent(TypeMaterial, Content{}); err == nil {
		t.Error("empty material should fail (no file, no url)")
	}
	if _, err := normalizeAndValidateContent(TypeMaterial, Content{URL: "ftp://x"}); err == nil {
		t.Error("non-http url should fail")
	}
	c, err := normalizeAndValidateContent(TypeMaterial, Content{URL: "  https://ex.com/x.pdf  ", Description: " a doc "})
	if err != nil {
		t.Fatalf("valid material: %v", err)
	}
	if c.URL != "https://ex.com/x.pdf" || c.Description != "a doc" {
		t.Errorf("not trimmed: %+v", c)
	}
	if c.Body != "" || c.Prompt != "" {
		t.Errorf("stripTo left cross-type fields: %+v", c)
	}
}

func TestValidateContent_Writing(t *testing.T) {
	if _, err := normalizeAndValidateContent(TypeWriting, Content{Prompt: "  "}); err == nil {
		t.Error("blank prompt should fail")
	}
	if _, err := normalizeAndValidateContent(TypeWriting, Content{Prompt: "Write about X", MinWords: -1}); err == nil {
		t.Error("negative min words should fail")
	}
}

func TestValidateContent_QuizRules(t *testing.T) {
	single := Question{Prompt: "2+2?", Kind: "single", Choices: []Choice{
		{ID: "a", Text: "3"}, {ID: "b", Text: "4"},
	}, Correct: []string{"b"}}

	if _, err := normalizeAndValidateContent(TypeQuiz, Content{}); err == nil {
		t.Error("quiz with no questions should fail")
	}

	// unknown correct id
	bad := single
	bad.Correct = []string{"zzz"}
	if _, err := normalizeAndValidateContent(TypeQuiz, Content{Questions: []Question{bad}}); err == nil {
		t.Error("correct id not among choices should fail")
	}

	// single with 2 correct
	multi2 := single
	multi2.Correct = []string{"a", "b"}
	if _, err := normalizeAndValidateContent(TypeQuiz, Content{Questions: []Question{multi2}}); err == nil {
		t.Error("single-choice with 2 correct should fail")
	}

	// happy path — ids get filled, points default to 1
	noID := Question{Prompt: "pick", Kind: "multi", Choices: []Choice{{Text: "x"}, {Text: "y"}}, Correct: nil}
	noID.Choices[0].ID = "x1"
	noID.Choices[1].ID = "y1"
	noID.Correct = []string{"x1", "y1"}
	out, err := normalizeAndValidateContent(TypeQuiz, Content{Questions: []Question{single, noID}})
	if err != nil {
		t.Fatalf("valid quiz: %v", err)
	}
	if len(out.Questions) != 2 {
		t.Fatalf("want 2 questions")
	}
	if out.Questions[1].ID == "" || out.Questions[1].Points != 1 {
		t.Errorf("question not normalized: %+v", out.Questions[1])
	}
}

func TestValidateContent_TextQuestion(t *testing.T) {
	q := Question{Prompt: "Capital of France?", Kind: "text", Correct: []string{" Paris ", ""}}
	out, err := normalizeAndValidateContent(TypeReading, Content{Passage: "France...", Questions: []Question{q}})
	if err != nil {
		t.Fatalf("valid text question: %v", err)
	}
	if got := out.Questions[0].Correct; len(got) != 1 || got[0] != "Paris" {
		t.Errorf("accepted answers not cleaned: %v", got)
	}
	if out.Questions[0].Choices != nil {
		t.Error("text question kept choices")
	}
}

func TestValidateContent_Listening_NeedsAudio(t *testing.T) {
	q := Question{Prompt: "q", Kind: "single", Choices: []Choice{{ID: "a", Text: "1"}, {ID: "b", Text: "2"}}, Correct: []string{"a"}}
	_, err := normalizeAndValidateContent(TypeListening, Content{Questions: []Question{q}})
	if err == nil || !strings.Contains(err.Error(), "audio") {
		t.Errorf("listening without audio: got %v", err)
	}
}
