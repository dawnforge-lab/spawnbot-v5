package skills

import (
	"testing"
)

func TestSubstituteArgs_AllVariables(t *testing.T) {
	body := "Query: ${ARGUMENTS}\nFirst: ${ARG1}\nSecond: ${ARG2}\nDir: ${SKILL_DIR}\nWS: ${WORKSPACE}"
	result := SubstituteArgs(body, []string{"hello", "world"}, "/skills/test", "/workspace")
	expected := "Query: hello world\nFirst: hello\nSecond: world\nDir: /skills/test\nWS: /workspace"
	if result != expected {
		t.Errorf("expected:\n%s\n\ngot:\n%s", expected, result)
	}
}

func TestSubstituteArgs_NoArgs(t *testing.T) {
	body := "Run with: ${ARGUMENTS}\nFirst: ${ARG1}"
	result := SubstituteArgs(body, nil, "/skills/test", "/workspace")
	if result != "Run with: \nFirst: ${ARG1}" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestSubstituteArgs_ExtraArgs(t *testing.T) {
	body := "All: ${ARGUMENTS}\nFirst: ${ARG1}\nThird: ${ARG3}"
	result := SubstituteArgs(body, []string{"a", "b", "c"}, "/skills/test", "/workspace")
	expected := "All: a b c\nFirst: a\nThird: c"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSubstituteArgs_UndefinedLeftAsIs(t *testing.T) {
	body := "First: ${ARG1}\nSecond: ${ARG2}"
	result := SubstituteArgs(body, []string{"only-one"}, "/skills/test", "/workspace")
	expected := "First: only-one\nSecond: ${ARG2}"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSubstituteArgs_NoVariables(t *testing.T) {
	body := "No variables here."
	result := SubstituteArgs(body, []string{"arg1"}, "/skills/test", "/workspace")
	if result != body {
		t.Errorf("expected unchanged body, got %q", result)
	}
}
