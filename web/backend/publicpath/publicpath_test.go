package publicpath

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalize(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"", ""},
		{"/", ""},
		{"diclaw", "/diclaw"},
		{"/diclaw/", "/diclaw"},
		{"/diclaw//", "/diclaw"},
	} {
		if got := Normalize(tc.in); got != tc.want {
			t.Fatalf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWithBase(t *testing.T) {
	for _, tc := range []struct {
		base, path, want string
	}{
		{"", "/mobile", "/mobile"},
		{"/diclaw", "/mobile", "/diclaw/mobile"},
		{"/diclaw", "/", "/diclaw/"},
		{"/diclaw", "/diclaw/mobile", "/diclaw/mobile"},
	} {
		if got := WithBase(tc.base, tc.path); got != tc.want {
			t.Fatalf("WithBase(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}

func TestStripBase(t *testing.T) {
	for _, tc := range []struct {
		base, path, want string
		ok               bool
	}{
		{"", "/mobile", "/mobile", true},
		{"/diclaw", "/diclaw", "/", true},
		{"/diclaw", "/diclaw/mobile", "/mobile", true},
		{"/diclaw", "/mobile", "/mobile", false},
	} {
		got, ok := StripBase(tc.base, tc.path)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("StripBase(%q, %q) = %q, %t; want %q, %t", tc.base, tc.path, got, ok, tc.want, tc.ok)
		}
	}
}

func TestStripPrefixMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Path", r.URL.Path)
		w.Header().Set("X-External-URI", ExternalRequestURI(r))
		w.WriteHeader(http.StatusNoContent)
	})
	handler := StripPrefixMiddleware("/diclaw", next)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/diclaw/mobile?x=1", nil)
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Path"); got != "/mobile" {
		t.Fatalf("rewritten path = %q, want /mobile", got)
	}
	if got := rec.Header().Get("X-External-URI"); got != "/diclaw/mobile?x=1" {
		t.Fatalf("external uri = %q, want /diclaw/mobile?x=1", got)
	}
}
