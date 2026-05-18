package pricing

import "testing"

func TestCostRoundsDown(t *testing.T) {
	tab := NewTable()
	// 24¢/min × 30s = 12¢ exactos
	if c := tab.Cost("openai_realtime", 30); c != 12 {
		t.Errorf("30s openai_realtime: got %d cents, want 12", c)
	}
	// 24¢/min × 1s = 0.4¢ → 0 (round down)
	if c := tab.Cost("openai_realtime", 1); c != 0 {
		t.Errorf("1s openai_realtime: got %d cents, want 0", c)
	}
	// 24¢/min × 60s = 24¢
	if c := tab.Cost("openai_realtime", 60); c != 24 {
		t.Errorf("60s openai_realtime: got %d cents, want 24", c)
	}
}

func TestCostUnknownProviderIsZero(t *testing.T) {
	tab := NewTable()
	if c := tab.Cost("never_seen", 1000); c != 0 {
		t.Errorf("unknown provider should return 0, got %d", c)
	}
}

func TestCostEchoIsFree(t *testing.T) {
	tab := NewTable()
	if c := tab.Cost("echo", 600); c != 0 {
		t.Errorf("echo should always be 0 (sandbox), got %d", c)
	}
}

func TestCostHandlesNegativeDuration(t *testing.T) {
	tab := NewTable()
	if c := tab.Cost("openai_realtime", -5); c != 0 {
		t.Errorf("negative duration should clamp to 0, got %d", c)
	}
}

func TestNilTableIsSafe(t *testing.T) {
	var tab *Table
	if c := tab.Cost("openai_realtime", 60); c != 0 {
		t.Errorf("nil table should return 0, got %d", c)
	}
	if r := tab.CentsPerMin("openai_realtime"); r != 0 {
		t.Errorf("nil table CentsPerMin should return 0, got %d", r)
	}
}
