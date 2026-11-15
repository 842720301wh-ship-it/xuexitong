package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Account struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Password  string `json:"password,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
	Status    string `json:"status"`
}
type Model struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Endpoint string   `json:"endpoint"`
	Protocol string   `json:"protocol,omitempty"`
	APIKey   string   `json:"api_key,omitempty"`
	Models   []string `json:"models"`
	Primary  string   `json:"primary"`
	Fallback string   `json:"fallback"`
	HasKey   bool     `json:"has_key"`
}
type Config struct {
	Accounts         []Account `json:"accounts"`
	DefaultAccountID string    `json:"default_account_id"`
	Models           []Model   `json:"models"`
	Tasks            []Task    `json:"tasks"`
}
type TaskCourse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Task refers to managed credentials, without copying passwords or model keys.
type Task struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	AccountID     string       `json:"account_id"`
	ModelConfigID string       `json:"model_config_id"`
	Courses       []TaskCourse `json:"courses"`
	AIRetries     int          `json:"ai_retries"`
	Retries       int          `json:"retries"`
	RetryDelay    int          `json:"retry_delay"`
	Speed         float64      `json:"speed"`
	Jobs          int          `json:"jobs"`
	Sequential    bool         `json:"sequential"`
	NotOpenAction string       `json:"notopen_action"`
	AutoSubmit    bool         `json:"auto_submit"`
	CreatedAt     string       `json:"created_at"`
	UpdatedAt     string       `json:"updated_at"`
}
type Store struct {
	mu    sync.Mutex
	dir   string
	aead  cipher.AEAD
	value Config
}

func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Open(dir string) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	keypath := filepath.Join(dir, "vault.key")
	key, e := os.ReadFile(keypath)
	if os.IsNotExist(e) {
		// An existing vault must never be silently replaced when its key is missing.
		if _, stat := os.Stat(filepath.Join(dir, "settings.enc")); !os.IsNotExist(stat) {
			return nil, errors.New("配置密钥缺失，请恢复原密钥")
		}
		key = make([]byte, 32)
		if _, e = rand.Read(key); e != nil {
			return nil, e
		}
		e = os.WriteFile(keypath, key, 0600)
	}
	if e != nil {
		return nil, e
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	aead, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	s := &Store{dir: dir, aead: aead, value: Config{Accounts: []Account{}, Models: []Model{}, Tasks: []Task{}}}
	b, e := os.ReadFile(filepath.Join(dir, "settings.enc"))
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return nil, e
	}
	if len(b) < aead.NonceSize() {
		return nil, errors.New("配置文件损坏")
	}
	plain, e := aead.Open(nil, b[:aead.NonceSize()], b[aead.NonceSize():], nil)
	if e != nil {
		return nil, errors.New("无法解密配置文件")
	}
	if e = json.Unmarshal(plain, &s.value); e != nil {
		return nil, e
	}
	return s, nil
}
func (s *Store) Read() Config { s.mu.Lock(); defer s.mu.Unlock(); return clone(s.value) }
func clone(c Config) Config {
	b, _ := json.Marshal(c)
	var out Config
	_ = json.Unmarshal(b, &out)
	return out
}
func Public(c Config) Config {
	c = clone(c)
	for i := range c.Accounts {
		c.Accounts[i].Password = ""
	}
	for i := range c.Models {
		c.Models[i].HasKey = c.Models[i].APIKey != ""
		c.Models[i].APIKey = ""
	}
	return c
}
func (s *Store) Update(fn func(*Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.value)
	if e := fn(&next); e != nil {
		return e
	}
	plain, e := json.Marshal(next)
	if e != nil {
		return e
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return e
	}
	data := s.aead.Seal(nonce, nonce, plain, nil)
	tmp := filepath.Join(s.dir, "settings.tmp")
	if e = os.WriteFile(tmp, data, 0600); e != nil {
		return e
	}
	if e = os.Rename(tmp, filepath.Join(s.dir, "settings.enc")); e != nil {
		return e
	}
	s.value = next
	return nil
}
func Timestamp() string { return time.Now().UTC().Format(time.RFC3339) }
