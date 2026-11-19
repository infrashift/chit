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
)

// ErrNotFound is returned by Get for a key the store does not hold.
var ErrNotFound = errors.New("blob: object not found")

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
	return &Memory{objects: map[string][]byte{}, types: map[string]string{}}
}

func (m *Memory) Put(_ context.Context, key, contentType string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.PutErr != nil {
		return m.PutErr
	}
	m.objects[key] = slices.Clone(data)
	m.types[key] = contentType
	return nil
}

func (m *Memory) Get(_ context.Context, key string) (io.ReadCloser, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	if !ok {
		return nil, 0, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), int64(len(b)), nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	delete(m.types, key)
	return nil
}

// Keys lists the stored keys, sorted.
func (m *Memory) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Sorted(maps.Keys(m.objects))
}

// Object returns a stored object's bytes.
func (m *Memory) Object(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	return slices.Clone(b), ok
}
