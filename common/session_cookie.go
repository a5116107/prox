package common

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

func normalizeHTTPSOrigin(rawOrigin string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawOrigin))
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(parsedURL.Scheme, "https") || parsedURL.Host == "" || parsedURL.Hostname() == "" {
		return "", fmt.Errorf("origin must use https and include a host")
	}
	if parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" || (parsedURL.EscapedPath() != "" && parsedURL.EscapedPath() != "/") {
		return "", fmt.Errorf("origin must not include credentials, path, query, or fragment")
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	port := parsedURL.Port()
	if port == "443" {
		port = ""
	}
	host := hostname
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	return "https://" + host, nil
}

func IsSessionCookieTrustedOrigin(origin string) bool {
	normalized, err := normalizeHTTPSOrigin(origin)
	if err != nil {
		return false
	}
	for _, trustedOrigin := range SessionCookieTrustedURLs {
		if normalized == trustedOrigin {
			return true
		}
	}
	return false
}

func InitSessionCookieSettings() error {
	secureRaw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE"))
	trustedURLsRaw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_TRUSTED_URL"))

	SessionCookieSecure = false
	SessionCookieTrustedURLs = nil

	if secureRaw == "" || strings.EqualFold(secureRaw, "false") {
		if trustedURLsRaw != "" {
			return fmt.Errorf("SESSION_COOKIE_TRUSTED_URL requires SESSION_COOKIE_SECURE=true")
		}
		return nil
	}

	if !strings.EqualFold(secureRaw, "true") {
		return fmt.Errorf("SESSION_COOKIE_SECURE must be true or false")
	}

	if trustedURLsRaw == "" {
		return fmt.Errorf("SESSION_COOKIE_SECURE=true requires SESSION_COOKIE_TRUSTED_URL")
	}

	trustedURLs := make([]string, 0, strings.Count(trustedURLsRaw, ",")+1)
	seen := make(map[string]struct{}, cap(trustedURLs))
	for _, trustedURL := range strings.Split(trustedURLsRaw, ",") {
		trustedURL = strings.TrimSpace(trustedURL)
		if trustedURL == "" {
			return fmt.Errorf("SESSION_COOKIE_TRUSTED_URL contains an empty URL")
		}
		trustedOrigin, err := normalizeHTTPSOrigin(trustedURL)
		if err != nil {
			return fmt.Errorf("invalid SESSION_COOKIE_TRUSTED_URL %q: %w", trustedURL, err)
		}
		if _, ok := seen[trustedOrigin]; ok {
			continue
		}
		seen[trustedOrigin] = struct{}{}
		trustedURLs = append(trustedURLs, trustedOrigin)
	}

	SessionCookieTrustedURLs = trustedURLs
	SessionCookieSecure = true
	return nil
}
