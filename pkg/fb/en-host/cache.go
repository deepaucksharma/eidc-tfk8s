package enhost

import (
	"sync"
	"time"
)

// HostInfo contains information about a host
type HostInfo struct {
	// Host identification
	Hostname     string
	IP           string
	
	// System information
	OS           string
	Architecture string
	
	// Resource information
	CPUCount     int
	TotalMemory  int64
	
	// Process information
	ProcessMap   map[int]ProcessInfo
	
	// Time when this information was collected
	CollectedAt  time.Time
}

// ProcessInfo contains information about a process
type ProcessInfo struct {
	PID           int
	CommandLine   string
	User          string
	StartTime     time.Time
	CPUUsage      float64
	MemoryUsage   int64
}

// HostInfoCache provides caching for host information
type HostInfoCache struct {
	cache       map[string]*HostInfo  // key is hostname or IP
	mu          sync.RWMutex
	ttl         time.Duration
	cleanupTick time.Duration
	stopCh      chan struct{}
}

// NewHostInfoCache creates a new host info cache
func NewHostInfoCache(ttl time.Duration) *HostInfoCache {
	cache := &HostInfoCache{
		cache:       make(map[string]*HostInfo),
		ttl:         ttl,
		cleanupTick: ttl / 2,  // Clean up at half the TTL interval
		stopCh:      make(chan struct{}),
	}
	
	// Start the cleanup goroutine
	go cache.cleanupLoop()
	
	return cache
}

// cleanupLoop periodically cleans up expired cache entries
func (c *HostInfoCache) cleanupLoop() {
	ticker := time.NewTicker(c.cleanupTick)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup removes expired entries from the cache
func (c *HostInfoCache) cleanup() {
	now := time.Now()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for key, info := range c.cache {
		if now.Sub(info.CollectedAt) > c.ttl {
			delete(c.cache, key)
		}
	}
}

// Get retrieves host info from the cache
func (c *HostInfoCache) Get(key string) (*HostInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	info, found := c.cache[key]
	if !found {
		return nil, false
	}
	
	// Check if expired
	if time.Since(info.CollectedAt) > c.ttl {
		return nil, false
	}
	
	return info, true
}

// Put adds or updates host info in the cache
func (c *HostInfoCache) Put(key string, info *HostInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Update collection time
	info.CollectedAt = time.Now()
	
	c.cache[key] = info
}

// GetProcessInfo retrieves process info from the cache
func (c *HostInfoCache) GetProcessInfo(host string, pid int) (*ProcessInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	hostInfo, found := c.cache[host]
	if !found {
		return nil, false
	}
	
	// Check if expired
	if time.Since(hostInfo.CollectedAt) > c.ttl {
		return nil, false
	}
	
	// Check if process exists
	procInfo, found := hostInfo.ProcessMap[pid]
	if !found {
		return nil, false
	}
	
	return &procInfo, true
}

// SetTTL updates the cache TTL
func (c *HostInfoCache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	c.ttl = ttl
	c.cleanupTick = ttl / 2
	c.mu.Unlock()
}

// Stop stops the cache cleanup loop
func (c *HostInfoCache) Stop() {
	close(c.stopCh)
}

// CollectHostInfo collects information about the current host
// This is a placeholder implementation - in a real system this would
// collect actual system information
func CollectHostInfo() (*HostInfo, error) {
	// In a real implementation, this would:
	// 1. Get hostname, IP, etc. from the operating system
	// 2. Collect system resource information (CPU, memory)
	// 3. Collect process information from /proc (on Linux)
	
	// For now, return a stub
	return &HostInfo{
		Hostname:     "localhost",
		IP:           "127.0.0.1",
		OS:           "linux",
		Architecture: "x86_64",
		CPUCount:     4,
		TotalMemory:  8 * 1024 * 1024 * 1024, // 8 GB
		ProcessMap:   make(map[int]ProcessInfo),
		CollectedAt:  time.Now(),
	}, nil
}

// CollectProcessInfo collects information about a specific process
// This is a placeholder implementation - in a real system this would
// collect actual process information
func CollectProcessInfo(pid int) (*ProcessInfo, error) {
	// In a real implementation, this would:
	// 1. Read process information from /proc/[pid] (on Linux)
	// 2. Get command line, user, start time, etc.
	// 3. Calculate CPU and memory usage
	
	// For now, return a stub
	return &ProcessInfo{
		PID:         pid,
		CommandLine: "example process",
		User:        "user",
		StartTime:   time.Now().Add(-time.Hour), // Started an hour ago
		CPUUsage:    1.5,                        // 1.5% CPU
		MemoryUsage: 100 * 1024 * 1024,          // 100 MB
	}, nil
}
