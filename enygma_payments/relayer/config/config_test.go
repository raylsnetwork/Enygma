package config

import "testing"

// ── parseAPIKeys (Fix H-06) ───────────────────────────────────────────────────

func TestParseAPIKeys_Empty(t *testing.T) {
	got, err := parseAPIKeys("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty map", got)
	}
}

func TestParseAPIKeys_MultipleBanks(t *testing.T) {
	got, err := parseAPIKeys("bank-a:tok-a,bank-b:tok-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["tok-a"] != "bank-a" || got["tok-b"] != "bank-b" {
		t.Errorf("got %v, want tok-a->bank-a, tok-b->bank-b", got)
	}
}

func TestParseAPIKeys_TokenContainingColon(t *testing.T) {
	// SplitN(entry, ":", 2) must stop at the first colon, so a token that
	// itself contains ':' is preserved intact rather than truncated.
	got, err := parseAPIKeys("bank-a:tok:with:colons")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["tok:with:colons"] != "bank-a" {
		t.Errorf("got %v, want \"tok:with:colons\"->bank-a", got)
	}
}

func TestParseAPIKeys_Malformed(t *testing.T) {
	for _, bad := range []string{"no-colon-here", "bank-a:", ":tok-a"} {
		if _, err := parseAPIKeys(bad); err == nil {
			t.Errorf("parseAPIKeys(%q): expected error, got nil", bad)
		}
	}
}

func TestParseAPIKeys_DuplicateTokenRejected(t *testing.T) {
	// Two banks must never be able to share one token — that is exactly
	// H-06's "no per-bank attribution" problem re-introduced by config.
	if _, err := parseAPIKeys("bank-a:shared-tok,bank-b:shared-tok"); err == nil {
		t.Fatal("expected error for duplicate token across two banks, got nil")
	}
}
