package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// UserStat represents connection statistics to a single user
type UserStat struct {
	Username      string    `json:"username"`
	ConnectCount  int       `json:"connect_count"`
	LastConnected time.Time `json:"last_connected"`
}

// StatManager manages user connection stats
type StatManager struct {
	mu       sync.Mutex
	users    map[string]*UserStat
	dataFile string
}

// NewStatManager creates a new StatManager instance
func NewStatManager(dataDir string) *StatManager {
	return &StatManager{
		users:    make(map[string]*UserStat),
		dataFile: filepath.Join(dataDir, "user_stats.json"),
	}
}

// Load reads user stats from JSON files
func (sm *StatManager) Load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, err := os.Stat(sm.dataFile); os.IsNotExist(err) {
		// File does not exist
		return nil
	}

	data, err := os.ReadFile(sm.dataFile)
	if err != nil {
		return err
	}

	var users []*UserStat
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	sm.users = make(map[string]*UserStat)
	for _, u := range users {
		sm.users[u.Username] = u
	}

	return nil
}

// Save writes user stats to JSON file
func (sm *StatManager) Save() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	users := make([]*UserStat, 0, len(sm.users))
	for _, u := range sm.users {
		users = append(users, u)
	}

	// Sort most recent first
	// Is this a kind of bubble sort?
	sort.Slice(users, func(i, j int) bool {
		return users[i].LastConnected.After(users[j].LastConnected)
	})

	data, err := json.MarshalIndent(users, "", " ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(sm.dataFile), 0o755); err != nil {
		return err
	}

	return os.WriteFile(sm.dataFile, data, 0o644)
}

// Record records a user connection
func (sm *StatManager) Record(username string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if u, exists := sm.users[username]; exists {
		u.ConnectCount++
		u.LastConnected = time.Now()
	} else {
		sm.users[username] = &UserStat{
			Username:      username,
			ConnectCount:  1,
			LastConnected: time.Now(),
		}
	}
}

// GetUserStat returns the stat for a specific user
func (sm *StatManager) GetUserStat(username string) (*UserStat, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	user, exists := sm.users[username]
	return user, exists
}

// GetRecentUsers returns the most recent users (excluding the current user)
func (sm *StatManager) GetRecentUsers(excludeUser string, limit int) []*UserStat {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	users := make([]*UserStat, 0, len(sm.users))
	for _, user := range sm.users {
		if user.Username != excludeUser {
			users = append(users, &UserStat{
				Username:      user.Username,
				ConnectCount:  user.ConnectCount,
				LastConnected: user.LastConnected,
			})
		}
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].LastConnected.After(users[j].LastConnected)
	})

	if limit > 0 && len(users) > limit {
		users = users[:limit]
	}

	return users
}

