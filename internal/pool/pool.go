package pool

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	CooldownUnauthorized = 30 * 24 * time.Hour
	CooldownUpstream     = 30 * time.Second
)

type cooldownEntry struct {
	Index  int    `json:"index"`
	Until  int64  `json:"until"`
	Reason string `json:"reason"`
}

type cooldownFile struct {
	Entries []cooldownEntry `json:"entries"`
}

type Pool struct {
	keys         []string
	cooldown     map[int]cooldownEntry
	cooldownPath string
	mu           sync.Mutex
	now          func() time.Time
}

func LoadKeys(path string) (*Pool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		keys = append(keys, line)
	}
	if len(keys) == 0 {
		return nil, errors.New("no keys in file")
	}
	return &Pool{
		keys:     keys,
		cooldown: map[int]cooldownEntry{},
		now:      time.Now,
	}, nil
}

func (p *Pool) SetCooldownFile(path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cooldownPath = path
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var file cooldownFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	now := p.now().Unix()
	for _, e := range file.Entries {
		if e.Index < 0 || e.Index >= len(p.keys) {
			continue
		}
		if e.Until > now {
			p.cooldown[e.Index] = e
		}
	}
	return nil
}

func (p *Pool) Size() int {
	return len(p.keys)
}

func (p *Pool) DisabledCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now().Unix()
	n := 0
	for _, e := range p.cooldown {
		if e.Until > now {
			n++
		}
	}
	return n
}

func (p *Pool) Pick(sessionKey string) (int, string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(p.keys)
	if n == 0 {
		return 0, "", false
	}
	start := hashIndex(sessionKey, n)
	now := p.now().Unix()
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		if e, ok := p.cooldown[idx]; ok && e.Until > now {
			continue
		}
		return idx, p.keys[idx], true
	}
	return 0, "", false
}

func (p *Pool) Disable(index int, d time.Duration, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if index < 0 || index >= len(p.keys) {
		return
	}
	p.cooldown[index] = cooldownEntry{
		Index:  index,
		Until:  p.now().Add(d).Unix(),
		Reason: reason,
	}
	_ = p.saveLocked()
}

func (p *Pool) saveLocked() error {
	if p.cooldownPath == "" {
		return nil
	}
	now := p.now().Unix()
	file := cooldownFile{Entries: make([]cooldownEntry, 0, len(p.cooldown))}
	for _, e := range p.cooldown {
		if e.Until > now {
			file.Entries = append(file.Entries, e)
		}
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	tmp := p.cooldownPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.cooldownPath)
}

func hashIndex(sessionKey string, n int) int {
	sum := sha256.Sum256([]byte(sessionKey))
	return int(binary.BigEndian.Uint64(sum[:8]) % uint64(n))
}
