package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
)

// HashURL creates a SHA256 hash of a URL string.
// This is useful for creating consistent, safe keys for Redis.
func HashURL(rawURL string) string {
	h := sha256.New()
	h.Write([]byte(rawURL))
	return hex.EncodeToString(h.Sum(nil))
}

// ToAbsoluteURL converts a relative URL to an absolute URL given a base URL.
func ToAbsoluteURL(base *url.URL, relative string) (string, error) {
	relURL, err := url.Parse(relative)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(relURL).String(), nil
}
```
