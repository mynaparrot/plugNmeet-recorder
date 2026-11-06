package config

import (
	"fmt"
	"net/url"
	"strings"
)

// normalizeNatsUrls validates nats_info.nats_urls and maps http(s):// to ws(s)://,
// so both native NATS and WebSocket urls can be configured (nats.go does not
// translate http(s) itself). All urls must use the same scheme.
func normalizeNatsUrls(urls []string) ([]string, error) {
	var (
		out    []string
		scheme string
		seen   = make(map[string]struct{}, len(urls))
	)

	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid NATS url %q: %w", raw, err)
		}

		switch u.Scheme {
		case "http":
			u.Scheme = "ws"
		case "https":
			u.Scheme = "wss"
		case "ws", "wss", "nats", "tls":
			// keep as is
		default:
			return nil, fmt.Errorf("unsupported NATS url scheme in %q (allowed: nats, tls, ws, wss, http, https)", raw)
		}

		if u.Host == "" {
			return nil, fmt.Errorf("NATS url %q is missing a host", raw)
		}

		normalized := u.String()
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}

		if scheme == "" {
			scheme = u.Scheme
		} else if scheme != u.Scheme {
			return nil, fmt.Errorf("nats_info.nats_urls must not mix schemes (%s and %s): nats.go applies TLS globally and cannot mix websocket with native NATS urls", scheme, u.Scheme)
		}

		out = append(out, normalized)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("nats_info.nats_urls is empty, at least one NATS url is required")
	}

	return out, nil
}
