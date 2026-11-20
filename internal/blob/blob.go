// Package blob stores attachment bytes in an S3-compatible object store
// (SeaweedFS in Chit's deployments), or in memory for tests.
//
// The S3 client speaks the four calls Chit needs (PUT, GET and DELETE of an
// object, and HEAD/PUT of the bucket), path-style, signed with AWS
// Signature Version 4. It is small enough to read in one sitting, which an
// SDK pulled in for four calls would not be.
package blob

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"slices"
	"sync"

	"github.com/infrashift/chit/internal/assert"
)

// ErrNotFound is returned by Get for a key the store does not hold.
var ErrNotFound = errors.New("blob: object not found") // semscope: allow(NASA-P10-06) sentinel error: errors.Is needs one identity

// Store holds objects by key.
type Store interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	// Get returns the object's bytes and size. The caller closes the
	// reader.
	Get(ctx context.Context, key string) (io.ReadCloser, int64, error)
	// Delete removes an object; a missing one is not an error.
	Delete(ctx context.Context, key string) error
}

// Memory is an in-memory Store for tests.
type Memory struct {
	mu      sync.Mutex
	objects map[string][]byte
	types   map[string]string
	// PutErr, when set, fails every Put.
	PutErr error
}

// NewMemory returns an empty Memory.
func NewMemory() *Memory {
	m := &Memory{objects: map[string][]byte{}, types: map[string]string{}}
	assert.That(m.objects != nil, "NewMemory: nil object map")
	assert.That(m.types != nil, "NewMemory: nil type map")
	return m
}

func (m *Memory) Put(_ context.Context, key, contentType string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	assert.That(m.objects != nil && m.types != nil, "Memory.Put: not made by NewMemory")
	assert.That(len(m.objects) == len(m.types), "Memory.Put: object and type maps out of step")
	if m.PutErr != nil {
		return m.PutErr
	}
	m.objects[key] = slices.Clone(data)
	m.types[key] = contentType
	assert.That(len(m.objects[key]) == len(data), "Memory.Put: stored copy has another length")
	assert.That(len(m.objects) == len(m.types), "Memory.Put: object and type maps out of step after the put")
	return nil
}

func (m *Memory) Get(_ context.Context, key string) (io.ReadCloser, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	assert.That(m.objects != nil, "Memory.Get: not made by NewMemory")
	assert.That(len(m.objects) == len(m.types), "Memory.Get: object and type maps out of step")
	b, ok := m.objects[key]
	if !ok {
		return nil, 0, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), int64(len(b)), nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	assert.That(m.objects != nil && m.types != nil, "Memory.Delete: not made by NewMemory")
	delete(m.objects, key)
	delete(m.types, key)
	_, still := m.objects[key]
	assert.That(!still, "Memory.Delete: the object is still there")
	assert.That(len(m.objects) == len(m.types), "Memory.Delete: object and type maps out of step")
	return nil
}

// Keys lists the stored keys, sorted.
func (m *Memory) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	assert.That(m.objects != nil, "Memory.Keys: not made by NewMemory")
	keys := slices.Sorted(maps.Keys(m.objects))
	assert.That(len(keys) == len(m.objects), "Memory.Keys: a key is missing")
	return keys
}

// Object returns a stored object's bytes.
func (m *Memory) Object(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	assert.That(m.objects != nil, "Memory.Object: not made by NewMemory")
	b, ok := m.objects[key]
	out := slices.Clone(b)
	assert.That(len(out) == len(b), "Memory.Object: the copy has another length")
	return out, ok
}
