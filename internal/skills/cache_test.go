package skills

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCache_GetReturnsNilBeforeRefresh(t *testing.T) {
	t.Parallel()
	c := NewCache()
	if c.Get() != nil {
		t.Errorf("expected nil inventory before Refresh, got %v", c.Get())
	}
}

func TestCache_RefreshSwapsInInventory(t *testing.T) {
	t.Parallel()
	c := NewCache()
	want := &Inventory{ScanTime: time.Now()}
	err := c.Refresh(func() (*Inventory, error) { return want, nil }, nil)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if c.Get() != want {
		t.Errorf("expected swapped-in pointer")
	}
}

func TestCache_RefreshErrorPreservesPrevious(t *testing.T) {
	t.Parallel()
	c := NewCache()
	first := &Inventory{ScanTime: time.Unix(1, 0)}
	if err := c.Refresh(func() (*Inventory, error) { return first, nil }, nil); err != nil {
		t.Fatalf("first Refresh: %v", err)
	}
	if err := c.Refresh(func() (*Inventory, error) { return nil, errors.New("scan failed") }, nil); err == nil {
		t.Fatal("expected error from failing scanFn")
	}
	if c.Get() != first {
		t.Errorf("last-good inventory should survive failed Refresh")
	}
}

func TestCache_RefreshNilInventoryIsError(t *testing.T) {
	t.Parallel()
	c := NewCache()
	err := c.Refresh(func() (*Inventory, error) { return nil, nil }, nil)
	if err == nil {
		t.Fatal("expected error when scanFn returns nil inventory + nil error")
	}
}

func TestCache_StaleDetectsMtimeChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tracked := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(tracked, []byte("v1"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	c := NewCache()
	if err := c.Refresh(func() (*Inventory, error) { return &Inventory{}, nil }, []string{tracked}); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if c.Stale() {
		t.Error("freshly refreshed cache should not be stale")
	}

	// Sleep enough to guarantee a different mtime — file systems vary in mtime
	// resolution; 10ms is conservative.
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(tracked, []byte("v2"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !c.Stale() {
		t.Error("cache should be stale after tracked file rewrite")
	}
}

func TestCache_StaleDetectsMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tracked := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(tracked, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	c := NewCache()
	if err := c.Refresh(func() (*Inventory, error) { return &Inventory{}, nil }, []string{tracked}); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if err := os.Remove(tracked); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !c.Stale() {
		t.Error("cache should be stale after tracked file removal")
	}
}

func TestCache_ConcurrentGetWhileRefresh(t *testing.T) {
	t.Parallel()
	c := NewCache()
	if err := c.Refresh(func() (*Inventory, error) { return &Inventory{}, nil }, nil); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = c.Get()
			}
		}()
	}
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = c.Refresh(func() (*Inventory, error) { return &Inventory{}, nil }, nil)
			}
		}()
	}
	wg.Wait()
}

func TestTrackedPathsFor_ReturnsBothScopes(t *testing.T) {
	t.Parallel()
	paths := TrackedPathsFor("/proj", "/home")
	if len(paths) < 4 {
		t.Errorf("expected at least 4 tracked paths, got %d", len(paths))
	}
	var foundProj, foundHome bool
	for _, p := range paths {
		if p == "/proj/.claude/settings.json" {
			foundProj = true
		}
		if p == "/home/.claude/settings.json" {
			foundHome = true
		}
	}
	if !foundProj || !foundHome {
		t.Errorf("expected proj+home tracked: proj=%v home=%v", foundProj, foundHome)
	}
}
