package util

import "strings"

// ParseDeviceType performs a best-effort device classification based on UA fragments.
func ParseDeviceType(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case ua == "":
		return "unknown"
	case strings.Contains(ua, "mobile"):
		return "mobile"
	case strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad"):
		return "tablet"
	default:
		return "desktop"
	}
}

// ParseBrowser extracts a coarse browser name from the User-Agent string.
func ParseBrowser(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "chrome"):
		return "chrome"
	case strings.Contains(ua, "firefox"):
		return "firefox"
	case strings.Contains(ua, "safari"):
		if strings.Contains(ua, "chrome") {
			return "chrome"
		}
		return "safari"
	case strings.Contains(ua, "edge"):
		return "edge"
	case strings.Contains(ua, "opera") || strings.Contains(ua, "opr/"):
		return "opera"
	default:
		return "unknown"
	}
}

// ParseOS infers the operating system family from UA fragments.
func ParseOS(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "windows"):
		return "windows"
	case strings.Contains(ua, "mac os") || strings.Contains(ua, "macos") || strings.Contains(ua, "darwin"):
		return "macos"
	case strings.Contains(ua, "linux"):
		return "linux"
	case strings.Contains(ua, "android"):
		return "android"
	case strings.Contains(ua, "ios") || strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		return "ios"
	default:
		return "unknown"
	}
}

// IsBot checks if a UA matches a configurable deny list.
func IsBot(ua string, denyList []string) bool {
	if ua == "" {
		return false
	}
	uaLower := strings.ToLower(ua)
	for _, fragment := range denyList {
		fragment = strings.ToLower(strings.TrimSpace(fragment))
		if fragment == "" {
			continue
		}
		if strings.Contains(uaLower, fragment) {
			return true
		}
	}
	return false
}
