package auth

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	manager := NewManager(time.Minute)
	token, expires, err := manager.Create()
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || expires.Before(time.Now()) {
		t.Fatalf("会话令牌无效：%q %v", token, expires)
	}
	if !manager.Valid(token) {
		t.Fatal("刚创建的会话应有效")
	}
	if manager.Valid("nope") {
		t.Fatal("未知令牌不应有效")
	}
	if manager.Valid("") {
		t.Fatal("空令牌不应有效")
	}
	manager.Revoke(token)
	if manager.Valid(token) {
		t.Fatal("撤销后的会话应失效")
	}
	if manager.Count() != 0 {
		t.Fatalf("活跃会话数应为 0，得到 %d", manager.Count())
	}
}

func TestExpiredSessionCleaned(t *testing.T) {
	manager := NewManager(time.Millisecond)
	token, _, err := manager.Create()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if manager.Valid(token) {
		t.Fatal("过期会话不应有效")
	}
	if manager.Count() != 0 {
		t.Fatal("过期会话应被清理")
	}
}

func TestConcurrentSessions(t *testing.T) {
	manager := NewManager(time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, _, err := manager.Create()
			if err != nil {
				t.Error(err)
				return
			}
			if !manager.Valid(token) {
				t.Error("并发创建的会话应有效")
			}
		}()
	}
	wg.Wait()
	if manager.Count() != 32 {
		t.Fatalf("应创建 32 个会话，得到 %d", manager.Count())
	}
}

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("s3cret-password")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "s3cret-password" {
		t.Fatal("不应明文保存密码")
	}
	if !VerifyPassword(hash, "s3cret-password") {
		t.Fatal("正确密码应通过校验")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("错误密码不应通过校验")
	}
	if VerifyPassword("", "anything") {
		t.Fatal("空散列不应通过校验")
	}
}

func TestSecureEqual(t *testing.T) {
	if !SecureEqual("abc", "abc") {
		t.Fatal("相同字符串应相等")
	}
	if SecureEqual("abc", "abd") || SecureEqual("", "") || SecureEqual("abc", "") {
		t.Fatal("不同或空字符串不应相等")
	}
}

func TestLimiter(t *testing.T) {
	limiter := NewLimiter(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		if retry := limiter.Take("1.2.3.4"); retry != 0 {
			t.Fatalf("第 %d 次尝试应放行，得到 retry=%d", i+1, retry)
		}
	}
	if limiter.Take("1.2.3.4") <= 0 {
		t.Fatal("达到上限后应被锁定并返回剩余秒数")
	}
	if limiter.Take("5.6.7.8") != 0 {
		t.Fatal("其他来源不应被连带锁定")
	}
	time.Sleep(60 * time.Millisecond)
	if limiter.Take("1.2.3.4") != 0 {
		t.Fatal("锁定时间过后应恢复")
	}

	limiter.Take("9.9.9.9")
	limiter.Take("9.9.9.9")
	limiter.Reset("9.9.9.9")
	for i := 0; i < 3; i++ {
		if limiter.Take("9.9.9.9") != 0 {
			t.Fatal("Reset 后应重新计数")
		}
	}
}

// TestLimiterConcurrent 并发请求不能一起穿过检查：同一来源同时发起再多尝试，也只放行 max 次。
func TestLimiterConcurrent(t *testing.T) {
	limiter := NewLimiter(5, time.Minute)
	var passed atomic.Int32
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if limiter.Take("1.2.3.4") == 0 {
				passed.Add(1)
			}
		})
	}
	wg.Wait()
	if got := passed.Load(); got != 5 {
		t.Fatalf("并发 50 次尝试应只放行 5 次，得到 %d", got)
	}
}

func TestRevokeAll(t *testing.T) {
	manager := NewManager(time.Minute)
	a, _, _ := manager.Create()
	b, _, _ := manager.Create()
	manager.RevokeAll()
	if manager.Valid(a) || manager.Valid(b) {
		t.Fatal("RevokeAll 后所有会话都应失效")
	}
}

func TestNormalizeRemote(t *testing.T) {
	cases := map[string]string{
		"192.168.1.10:54321": "192.168.1.10",
		"192.168.1.10":       "192.168.1.10",
		"[fd00::1]:8080":     "fd00::1",
		"":                   "unknown",
	}
	for input, want := range cases {
		if got := NormalizeRemote(input); got != want {
			t.Fatalf("NormalizeRemote(%q) = %q, want %q", input, got, want)
		}
	}
}
