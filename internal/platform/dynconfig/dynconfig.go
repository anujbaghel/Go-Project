package dynconfig

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

type Values struct {
	RestrictedFlows map[string][]int64 `json:"restrictedFlows"`
}

type Config struct {
	path string
	cur  atomic.Pointer[Values] // lock-free : readers load, the watcher stores
}

func Load(path string) (*Config, error) {
	c := &Config{path: path}
	if err := c.reload(); err != nil {
		return nil, err
	}
	go c.watch()
	return c, nil
}

func (c *Config) reload() error {
	b, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	var v Values
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	c.cur.Store(&v)
	return nil
}

func (c *Config) watch() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer w.Close()
	_ = w.Add(c.path)
	for ev := range w.Events {
		if ev.Op&(fsnotify.Write|fsnotify.Create) != 0 {
			if err := c.reload(); err != nil {
				slog.Error("config reload failed", "err", err)
			} else {
				slog.Info("config reloaded")
			}
		}
	}
}

// Snapshot — call this INSIDE functions, never cache it at init.
func (c *Config) Snapshot() *Values { return c.cur.Load() }

func (c *Config) IsFlowRestricted(flow string, ltid int64) bool {
	for _, id := range c.Snapshot().RestrictedFlows[flow] {
		if id == ltid {
			return true
		}
	}
	return false
}
