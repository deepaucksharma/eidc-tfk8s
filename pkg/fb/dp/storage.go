package dp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/rs/zerolog/log"
)

// DeduplicationStore defines the interface for deduplication storage backends
type DeduplicationStore interface {
	// Put stores a deduplication entry with the given key and TTL
	Put(key []byte, ttl time.Duration) error

	// Has checks if a deduplication key exists
	Has(key []byte) (bool, error)

	// Close closes the storage backend
	Close() error

	// Flush ensures data is persisted
	Flush() error
}

// ErrKeyAlreadyExists is returned when a key already exists in the store
var ErrKeyAlreadyExists = errors.New("key already exists in deduplication store")

// MemoryStore implements an in-memory deduplication store
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]time.Time
}

// NewMemoryStore creates a new in-memory deduplication store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make(map[string]time.Time),
	}
}

// Put stores a deduplication entry with the given key and TTL
func (s *MemoryStore) Put(key []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	strKey := string(key)
	expiryTime, exists := s.entries[strKey]

	// If key exists and hasn't expired, return error
	if exists && expiryTime.After(time.Now()) {
		return ErrKeyAlreadyExists
	}

	// Store the key with its expiration time
	s.entries[strKey] = time.Now().Add(ttl)
	return nil
}

// Has checks if a deduplication key exists
func (s *MemoryStore) Has(key []byte) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	strKey := string(key)
	expiryTime, exists := s.entries[strKey]
	
	// Key exists and hasn't expired
	if exists && expiryTime.After(time.Now()) {
		return true, nil
	}

	// Key doesn't exist or has expired
	return false, nil
}

// Close closes the memory store (no-op for in-memory store)
func (s *MemoryStore) Close() error {
	return nil
}

// Flush ensures data is persisted (no-op for in-memory store)
func (s *MemoryStore) Flush() error {
	return nil
}

// runGC runs garbage collection to remove expired entries
func (s *MemoryStore) runGC() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, expiryTime := range s.entries {
		if expiryTime.Before(now) {
			delete(s.entries, key)
		}
	}
}

// BadgerStore implements a persistent deduplication store using BadgerDB
type BadgerStore struct {
	db *badger.DB
}

// NewBadgerStore creates a new BadgerDB-backed deduplication store
func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(path)
	// Configure BadgerDB options
	opts.Logger = nil           // Disable BadgerDB's logger
	opts.SyncWrites = false     // Async writes for better performance
	opts.ValueLogFileSize = 1 << 26 // 64MB
	opts.NumVersionsToKeep = 1  // Only need the latest version
	opts.NumMemtables = 2       // Use 2 memory tables

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	return &BadgerStore{
		db: db,
	}, nil
}

// Put stores a deduplication entry with the given key and TTL
func (s *BadgerStore) Put(key []byte, ttl time.Duration) error {
	// First check if the key already exists
	exists, err := s.Has(key)
	if err != nil {
		return fmt.Errorf("failed to check key existence: %w", err)
	}

	if exists {
		return ErrKeyAlreadyExists
	}

	// Key doesn't exist, add it
	err = s.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry(key, []byte{1}).WithTTL(ttl)
		return txn.SetEntry(entry)
	})

	if err != nil {
		return fmt.Errorf("failed to set key in BadgerDB: %w", err)
	}

	return nil
}

// Has checks if a deduplication key exists
func (s *BadgerStore) Has(key []byte) (bool, error) {
	var exists bool

	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			exists = false
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})

	if err != nil {
		return false, fmt.Errorf("failed to check key in BadgerDB: %w", err)
	}

	return exists, nil
}

// Close closes the BadgerDB store
func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// Flush ensures data is persisted
func (s *BadgerStore) Flush() error {
	return s.db.Sync()
}

// StartGarbageCollection starts the BadgerDB garbage collection in the background
func (s *BadgerStore) StartGarbageCollection(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Run value log garbage collection with 0.5 discard ratio
				err := s.db.RunValueLogGC(0.5)
				if err != nil && err != badger.ErrNoRewrite {
					log.Error().Err(err).Msg("BadgerDB value log GC failed")
				}
			}
		}
	}()
}
