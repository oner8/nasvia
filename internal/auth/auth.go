// Package auth 提供会话管理、密码校验与登录限速。
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// CookieName 会话 Cookie 名称。
const CookieName = "nasvia_session"

// DefaultTTL 会话有效期。
const DefaultTTL = 7 * 24 * time.Hour

// Manager 基于内存的会话管理器（重启即失效，无需持久化）。
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]time.Time
	ttl      time.Duration
}

// NewManager 创建会话管理器。
func NewManager(ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Manager{sessions: make(map[string]time.Time), ttl: ttl}
}

// TTL 返回会话有效期。
func (m *Manager) TTL() time.Duration { return m.ttl }

// Create 生成新会话令牌。
func (m *Manager) Create() (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	expires := time.Now().Add(m.ttl)
	m.mu.Lock()
	m.sessions[token] = expires
	m.mu.Unlock()
	return token, expires, nil
}

// Valid 校验令牌是否有效；过期令牌会被清理。
func (m *Manager) Valid(token string) bool {
	if token == "" {
		return false
	}
	m.mu.RLock()
	expires, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(expires) {
		m.Revoke(token)
		return false
	}
	return true
}

// Revoke 使令牌立即失效。
func (m *Manager) Revoke(token string) {
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}

// RevokeAll 使全部会话立即失效（改密码后踢掉所有设备）。
func (m *Manager) RevokeAll() {
	m.mu.Lock()
	clear(m.sessions)
	m.mu.Unlock()
}

// GC 清理过期会话，返回被清理数量。
func (m *Manager) GC() int {
	now := time.Now()
	n := 0
	m.mu.Lock()
	for token, expires := range m.sessions {
		if now.After(expires) {
			delete(m.sessions, token)
			n++
		}
	}
	m.mu.Unlock()
	return n
}

// Count 当前活跃会话数。
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// HashPassword 生成 bcrypt 密码散列。
func HashPassword(password string) (string, error) {
	buf, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// VerifyPassword 校验 bcrypt 散列；散列为空时返回 false。
func VerifyPassword(hash, password string) bool {
	if hash == "" || password == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// SecureEqual 常数时间字符串比较。
func SecureEqual(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Limiter 简单的登录失败限速器。
type Limiter struct {
	mu       sync.Mutex
	failures map[string]*attempt
	max      int
	lockFor  time.Duration
}

type attempt struct {
	count    int
	lockedAt time.Time
}

// NewLimiter 创建限速器：连续 max 次失败后锁定 lockFor。
func NewLimiter(max int, lockFor time.Duration) *Limiter {
	if max <= 0 {
		max = 5
	}
	if lockFor <= 0 {
		lockFor = 60 * time.Second
	}
	return &Limiter{failures: make(map[string]*attempt), max: max, lockFor: lockFor}
}

// Take 原子地检查并登记一次尝试：锁定期内返回剩余秒数（>0）；否则先把这次尝试计为失败，返回 0。
// 先计数、后校验密码，并发请求就没法在第一次失败落账前一起穿过检查；登录成功后调用 Reset 清掉。
func (l *Limiter) Take(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.failures[key]
	if !ok {
		a = &attempt{}
		l.failures[key] = a
	}
	if a.count >= l.max {
		if remain := l.lockFor - time.Since(a.lockedAt); remain > 0 {
			return int(remain.Seconds()) + 1
		}
		a.count = 0 // 锁定期已过，重新计数
	}
	a.count++
	a.lockedAt = time.Now()
	return 0
}

// Reset 登录成功后清除失败记录。
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.failures, key)
	l.mu.Unlock()
}

// NormalizeRemote 归一化来源地址，用于限速键。
func NormalizeRemote(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "unknown"
	}
	if idx := strings.LastIndex(addr, ":"); idx > 0 && !strings.Contains(addr[idx+1:], "]") {
		host := addr[:idx]
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			return strings.Trim(host, "[]")
		}
		if strings.Count(addr, ":") == 1 {
			return host
		}
	}
	return addr
}
