package netutil

import "testing"

func TestNormalizeIP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		ok       bool
	}{
		{name: "ipv4 with port", input: "192.0.2.4:8080", expected: "192.0.2.4", ok: true},
		{name: "ipv6 with port", input: "[2001:db8::1]:443", expected: "2001:db8::1", ok: true},
		{name: "ipv6 textual port", input: "[::1]:port", expected: "::1", ok: true},
		{name: "plain ipv4", input: "203.0.113.9", expected: "203.0.113.9", ok: true},
		{name: "plain ipv6", input: "2001:db8::5", expected: "2001:db8::5", ok: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := NormalizeIP(tc.input)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v, got %v", tc.ok, ok)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestNormalizeIPInvalid(t *testing.T) {
	if got, ok := NormalizeIP("not-an-ip"); ok {
		t.Fatalf("expected failure, got success with %q", got)
	}
}

func TestTruncateUserAgent(t *testing.T) {
	longUA := make([]rune, MaxUserAgentLength+10)
	for i := range longUA {
		longUA[i] = 'a'
	}
	truncated := TruncateUserAgent(string(longUA))
	if len([]rune(truncated)) != MaxUserAgentLength {
		t.Fatalf("expected %d runes, got %d", MaxUserAgentLength, len([]rune(truncated)))
	}
}
