package publicpath

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const EnvPublicBasePath = "PICOCLAW_PUBLIC_BASE_PATH"

type originalRequestURIKey struct{}

// Normalize converts a configured public base path to "" or "/name".
func Normalize(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "/" {
		return ""
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	cleaned := path.Clean(raw)
	if cleaned == "/" || cleaned == "." {
		return ""
	}
	return strings.TrimRight(cleaned, "/")
}

// WithBase prefixes absolute application paths with base.
func WithBase(base, appPath string) string {
	base = Normalize(base)
	if appPath == "" {
		appPath = "/"
	}
	if !strings.HasPrefix(appPath, "/") {
		appPath = "/" + appPath
	}
	if base == "" {
		return appPath
	}
	if appPath == "/" {
		return base + "/"
	}
	if appPath == base || strings.HasPrefix(appPath, base+"/") {
		return appPath
	}
	return base + appPath
}

// StripBase removes base from an externally visible path.
func StripBase(base, externalPath string) (string, bool) {
	base = Normalize(base)
	if base == "" {
		if externalPath == "" {
			return "/", true
		}
		return externalPath, true
	}
	if externalPath == base {
		return "/", true
	}
	if strings.HasPrefix(externalPath, base+"/") {
		stripped := strings.TrimPrefix(externalPath, base)
		if stripped == "" {
			return "/", true
		}
		return stripped, true
	}
	return externalPath, false
}

// ExternalRequestURI returns the request URI before StripPrefixMiddleware
// rewrote it, falling back to the current URI.
func ExternalRequestURI(r *http.Request) string {
	if r == nil {
		return "/"
	}
	if v, ok := r.Context().Value(originalRequestURIKey{}).(string); ok && v != "" {
		return v
	}
	if r.URL == nil {
		return "/"
	}
	return r.URL.RequestURI()
}

// EnsureExternalRequestURI returns a public request URI that includes base.
func EnsureExternalRequestURI(base string, r *http.Request) string {
	uri := ExternalRequestURI(r)
	if uri == "" {
		uri = "/"
	}
	if base == "" {
		return uri
	}
	pathPart := uri
	query := ""
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		pathPart = uri[:i]
		query = uri[i:]
	}
	if _, ok := StripBase(base, pathPart); ok && Normalize(base) != "" && strings.HasPrefix(pathPart, Normalize(base)) {
		return uri
	}
	return WithBase(base, pathPart) + query
}

// StripPrefixMiddleware rewrites requests under base to root-relative paths
// before they reach the existing mux. Requests outside base are left unchanged
// so local root-path development remains available.
func StripPrefixMiddleware(base string, next http.Handler) http.Handler {
	base = Normalize(base)
	if base == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originalURI := "/"
		if r.URL != nil {
			originalURI = r.URL.RequestURI()
		}
		ctx := context.WithValue(r.Context(), originalRequestURIKey{}, originalURI)
		r = r.WithContext(ctx)

		if r.URL == nil {
			next.ServeHTTP(w, r)
			return
		}
		stripped, ok := StripBase(base, r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		clone := r.Clone(ctx)
		u := *r.URL
		u.Path = stripped
		u.RawPath = ""
		clone.URL = &u
		next.ServeHTTP(w, clone)
	})
}

// CookiePath returns a path suitable for launcher auth cookies.
func CookiePath(base string) string {
	base = Normalize(base)
	if base == "" {
		return "/"
	}
	return base
}

// AddBaseToURLPath prefixes same-origin absolute URL strings.
func AddBaseToURLPath(base, rawURL string) string {
	base = Normalize(base)
	if base == "" || rawURL == "" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.IsAbs() || !strings.HasPrefix(rawURL, "/") || strings.HasPrefix(rawURL, "//") {
		return rawURL
	}
	u.Path = WithBase(base, u.Path)
	return u.String()
}
