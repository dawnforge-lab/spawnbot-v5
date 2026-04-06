package providers

import (
	"testing"
)

func TestExtractToolCallsFromXML_Single(t *testing.T) {
	text := `<functioncalls>
<invoke name="read_file">
<parameter name="path">SOUL.md</parameter>
</invoke>
</functioncalls>`

	calls := extractToolCallsFromXML(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Errorf("expected name read_file, got %s", calls[0].Name)
	}
	if calls[0].Arguments["path"] != "SOUL.md" {
		t.Errorf("expected path SOUL.md, got %v", calls[0].Arguments["path"])
	}
}

func TestExtractToolCallsFromXML_Multiple(t *testing.T) {
	text := `<functioncalls>
<invoke name="read_file">
<parameter name="path">SOUL.md</parameter>
</invoke>
<invoke name="list_dir">
<parameter name="path">memory</parameter>
</invoke>
</functioncalls>`

	calls := extractToolCallsFromXML(text)
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Errorf("call 0: expected read_file, got %s", calls[0].Name)
	}
	if calls[1].Name != "list_dir" {
		t.Errorf("call 1: expected list_dir, got %s", calls[1].Name)
	}
}

func TestExtractToolCallsFromXML_WithTypedParams(t *testing.T) {
	text := `<invoke name="exec">
<parameter name="command">ls -la</parameter>
<parameter name="timeout" type="integer">30</parameter>
</invoke>`

	calls := extractToolCallsFromXML(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Arguments["command"] != "ls -la" {
		t.Errorf("expected command 'ls -la', got %v", calls[0].Arguments["command"])
	}
	// 30 is valid JSON, should be parsed as float64
	if calls[0].Arguments["timeout"] != float64(30) {
		t.Errorf("expected timeout 30, got %v (%T)", calls[0].Arguments["timeout"], calls[0].Arguments["timeout"])
	}
}

func TestExtractToolCallsFromXML_NoMatch(t *testing.T) {
	calls := extractToolCallsFromXML("Hello, I'm a helpful assistant")
	if len(calls) != 0 {
		t.Fatalf("expected 0 tool calls, got %d", len(calls))
	}
}

func TestExtractToolCallsFromContent_XML(t *testing.T) {
	text := `Let me read that file for you.

<functioncalls>
<invoke name="read_file">
<parameter name="path">USER.md</parameter>
</invoke>
</functioncalls>`

	calls, remaining := ExtractToolCallsFromContent(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "read_file" {
		t.Errorf("expected read_file, got %s", calls[0].Name)
	}
	if remaining == text {
		t.Error("expected XML to be stripped from remaining content")
	}
}

func TestExtractToolCallsFromContent_NoToolCalls(t *testing.T) {
	text := "Just a normal response with no tool calls."
	calls, remaining := ExtractToolCallsFromContent(text)
	if len(calls) != 0 {
		t.Fatalf("expected 0 tool calls, got %d", len(calls))
	}
	if remaining != text {
		t.Errorf("expected unchanged text, got %q", remaining)
	}
}

func TestStripXMLToolCallsFromText(t *testing.T) {
	text := `Here is some text.

<functioncalls>
<invoke name="read_file">
<parameter name="path">SOUL.md</parameter>
</invoke>
</functioncalls>

And some more text.`

	stripped := stripXMLToolCallsFromText(text)
	if stripped != "Here is some text.\n\n\n\nAnd some more text." {
		t.Errorf("unexpected result: %q", stripped)
	}
}
