package proxy

import (
	"math/rand"
	"sync"
	"time"
)

// Manager handles the rotation of proxies and user agents.
type Manager struct {
	proxies    []string
	userAgents []string
	mu         sync.Mutex
	proxyIndex int
}

func NewManager() *Manager {
	// In production, load these from config or a remote service
	return &Manager{
		proxies: []string{
			// "http://user:pass@proxy1.com:8000",
			// "http://user:pass@proxy2.com:8000",
		},
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36",
		},
	}
}

// GetProxy returns a proxy URL from the list, rotating sequentially.
func (m *Manager) GetProxy() string {
	if len(m.proxies) == 0 {
		return "" // No proxy
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	proxy := m.proxies[m.proxyIndex]
	m.proxyIndex = (m.proxyIndex + 1) % len(m.proxies)
	return proxy
}

// GetUserAgent returns a random user agent string.
func (m *Manager) GetUserAgent() string {
	if len(m.userAgents) == 0 {
		return ""
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return m.userAgents[r.Intn(len(m.userAgents))]
}
