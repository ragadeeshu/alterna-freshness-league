package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Side Order run details (SideOrderChallengeDetailQuery) never change, so
// they are cached for up to a year, in memory and, when NXAPI_DATA_PATH is
// set (it is in the sidecar image), on disk so they survive restarts. The
// image URLs inside a run are signed and expire, though, so an entry is only
// kept until shortly before its earliest signed URL expires; after that the
// run is fetched again, which brings fresh URLs.
const (
	runCacheMaxAge    = 365 * 24 * time.Hour
	runCacheURLMargin = 7 * 24 * time.Hour // refresh this long before signed URLs expire
	runCacheMinAge    = time.Hour
)

var signedURLExpiry = regexp.MustCompile(`Expires=(\d+)`)

type cachedRun struct {
	Data    json.RawMessage `json:"data"`
	Expires time.Time       `json:"expires"`
}

type runCache struct {
	mu      sync.Mutex
	dir     string // "" keeps runs in memory only
	entries map[string]cachedRun
}

var runs = newRunCache(os.Getenv("NXAPI_DATA_PATH"))

func newRunCache(dataPath string) *runCache {
	c := &runCache{entries: map[string]cachedRun{}}
	if dataPath == "" {
		return c
	}
	dir := filepath.Join(dataPath, "side-order-runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("run cache: %v; keeping runs in memory only", err)
		return c
	}
	c.dir = dir
	c.prune()
	return c
}

// get returns a cached run if it is still fresh.
func (c *runCache) get(id string) (json.RawMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[id]
	if !ok && c.dir != "" {
		if data, err := os.ReadFile(c.path(id)); err == nil && json.Unmarshal(data, &entry) == nil {
			ok = true
			c.entries[id] = entry
		}
	}
	if !ok || time.Now().After(entry.Expires) {
		return nil, false
	}
	return entry.Data, true
}

func (c *runCache) put(id string, data json.RawMessage) {
	entry := cachedRun{Data: data, Expires: runExpiry(data, time.Now())}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[id] = entry
	if c.dir == "" {
		return
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// Write then rename, so a crash never leaves a half-written entry.
	tmp := c.path(id) + ".tmp"
	if err := os.WriteFile(tmp, encoded, 0o644); err != nil {
		log.Printf("run cache: %v", err)
		return
	}
	if err := os.Rename(tmp, c.path(id)); err != nil {
		log.Printf("run cache: %v", err)
	}
}

// path names a run's file after a hash of its ID (run IDs are base64 and may
// contain '/').
func (c *runCache) path(id string) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".json")
}

// prune removes expired runs from disk.
func (c *runCache) prune() {
	files, _ := filepath.Glob(filepath.Join(c.dir, "*.json"))
	now := time.Now()
	removed := 0
	for _, file := range files {
		var entry cachedRun
		data, err := os.ReadFile(file)
		if err != nil || json.Unmarshal(data, &entry) != nil || now.After(entry.Expires) {
			if os.Remove(file) == nil {
				removed++
			}
		}
	}
	log.Printf("run cache: %d runs on disk in %s (%d expired removed)", len(files)-removed, c.dir, removed)
}

// runExpiry keeps a run for a year, but only until a week before the
// earliest signed image URL in it expires (and at least an hour).
func runExpiry(data json.RawMessage, now time.Time) time.Time {
	expiry := now.Add(runCacheMaxAge)
	for _, match := range signedURLExpiry.FindAllSubmatch(data, -1) {
		if seconds, err := strconv.ParseInt(string(match[1]), 10, 64); err == nil {
			if urlExpiry := time.Unix(seconds, 0).Add(-runCacheURLMargin); urlExpiry.Before(expiry) {
				expiry = urlExpiry
			}
		}
	}
	if minimum := now.Add(runCacheMinAge); expiry.Before(minimum) {
		expiry = minimum
	}
	return expiry
}
