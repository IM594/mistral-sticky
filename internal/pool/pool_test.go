package pool

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPick_isDeterministic(t *testing.T) {
	p := mustPool(t, []string{"key-a", "key-b", "key-c"})
	i1, k1, ok := p.Pick("sess")
	if !ok {
		t.Fatal("expected pick")
	}
	i2, k2, ok := p.Pick("sess")
	if !ok || i1 != i2 || k1 != k2 {
		t.Fatalf("expected same key, got %d/%q then %d/%q", i1, k1, i2, k2)
	}
}

func TestPick_skipsDisabled(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p := mustPool(t, []string{"key-a", "key-b", "key-c"})
	p.now = func() time.Time { return now }
	i, _, ok := p.Pick("sess")
	if !ok {
		t.Fatal("expected pick")
	}
	p.Disable(i, time.Hour, "401")
	i2, k2, ok := p.Pick("sess")
	if !ok {
		t.Fatal("expected failover")
	}
	if i2 == i {
		t.Fatalf("still on disabled index %d key %s", i2, k2)
	}
}

func TestPick_allDisabled(t *testing.T) {
	now := time.Now()
	p := mustPool(t, []string{"key-a"})
	p.now = func() time.Time { return now }
	p.Disable(0, time.Hour, "401")
	_, _, ok := p.Pick("sess")
	if ok {
		t.Fatal("expected no key")
	}
}

func TestCooldown_persistsAndExpires(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cooldown.json")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p := mustPool(t, []string{"key-a", "key-b"})
	p.now = func() time.Time { return now }
	if err := p.SetCooldownFile(path); err != nil {
		t.Fatal(err)
	}
	p.Disable(0, time.Hour, "401")

	p2 := mustPool(t, []string{"key-a", "key-b"})
	p2.now = func() time.Time { return now }
	if err := p2.SetCooldownFile(path); err != nil {
		t.Fatal(err)
	}
	if p2.DisabledCount() != 1 {
		t.Fatalf("disabled=%d", p2.DisabledCount())
	}

	p2.now = func() time.Time { return now.Add(2 * time.Hour) }
	if p2.DisabledCount() != 0 {
		t.Fatalf("expected expiry, disabled=%d", p2.DisabledCount())
	}
}

func TestLoadKeys_rejectsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.txt")
	if err := os.WriteFile(path, []byte("\n# none\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKeys(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadKeys_skipsEmptyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.txt")
	if err := os.WriteFile(path, []byte("key-a\n\nkey-b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Size() != 2 {
		t.Fatalf("size=%d", p.Size())
	}
}

func mustPool(t *testing.T, keys []string) *Pool {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.txt")
	data := ""
	for _, k := range keys {
		data += k + "\n"
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
