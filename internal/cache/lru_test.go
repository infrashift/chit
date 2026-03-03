package cache

import (
	"testing"
	"time"
)

func TestNewLRU(t *testing.T) {
	c, err := NewLRU[string, int](10, time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestNewLRU_ZeroSize(t *testing.T) {
	_, err := NewLRU[string, int](0, time.Minute)
	if err == nil {
		t.Fatal("expected error for size 0")
	}
}

func TestSetAndGet(t *testing.T) {
	c, _ := NewLRU[string, int](10, time.Minute)
	c.Set("key1", 42)

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key to be found")
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}
}

func TestGet_Miss(t *testing.T) {
	c, _ := NewLRU[string, int](10, time.Minute)

	val, ok := c.Get("absent")
	if ok {
		t.Fatal("expected miss")
	}
	if val != 0 {
		t.Fatalf("expected zero value, got %d", val)
	}
}

func TestGet_Expired(t *testing.T) {
	c, _ := NewLRU[string, int](10, time.Millisecond)
	c.Set("key1", 42)

	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected expired entry to miss")
	}
}

func TestLRU_Eviction(t *testing.T) {
	c, _ := NewLRU[string, int](2, time.Minute)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3) // should evict "a"

	_, ok := c.Get("a")
	if ok {
		t.Fatal("expected 'a' to be evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("expected 'b' to still exist")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("expected 'c' to still exist")
	}
}

func TestRemove(t *testing.T) {
	c, _ := NewLRU[string, int](10, time.Minute)
	c.Set("key1", 42)
	c.Remove("key1")

	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected removed key to miss")
	}

	// Remove absent key should not panic
	c.Remove("nonexistent")
}

func TestLen(t *testing.T) {
	c, _ := NewLRU[string, int](10, time.Minute)
	if c.Len() != 0 {
		t.Fatal("expected initial Len=0")
	}

	c.Set("a", 1)
	c.Set("b", 2)
	if c.Len() != 2 {
		t.Fatalf("expected Len=2, got %d", c.Len())
	}

	c.Remove("a")
	if c.Len() != 1 {
		t.Fatalf("expected Len=1, got %d", c.Len())
	}
}

func TestPurge(t *testing.T) {
	c, _ := NewLRU[string, int](10, time.Minute)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Purge()

	if c.Len() != 0 {
		t.Fatalf("expected Len=0 after Purge, got %d", c.Len())
	}
}
