package gitlab_test

import (
	"strings"
	"testing"

	"github.com/dpopsuev/emcee/internal/adapter/driven/gitlab"
)

func TestNewWithURL_ValidURLs(t *testing.T) {
	validURLs := []struct {
		name string
		url  string
	}{
		{"GitLab SaaS", "https://gitlab.com"},
		{"Self-hosted HTTPS", "https://gitlab.cee.redhat.com"},
		{"Enterprise instance", "https://gitlab.company.com"},
		{"Localhost dev", "http://localhost:3000"},
		{"Localhost no port", "http://localhost"},
		{"Subdomain", "https://git.enterprise.io"},
	}

	for _, tc := range validURLs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := gitlab.NewWithURL("gitlab", "test-token", "test/project", tc.url)
			if err != nil {
				t.Errorf("expected %q to be valid, got error: %v", tc.url, err)
			}
		})
	}
}

func TestNewWithURL_BlockedURLs(t *testing.T) {
	blockedURLs := []struct {
		name      string
		url       string
		errSubstr string
	}{
		{"HTTP non-localhost", "http://gitlab.com", "http:// only allowed for localhost"},
		{"Private IP 10.x", "https://10.0.0.1", "private IP addresses are not allowed"},
		{"Private IP 192.168", "https://192.168.1.1", "private IP addresses are not allowed"},
		{"Private IP 172.16", "https://172.16.0.1", "private IP addresses are not allowed"},
		{"AWS metadata", "https://169.254.169.254", "private IP addresses are not allowed"},
		{"Loopback 127.0.0.1", "https://127.0.0.1", "private IP addresses are not allowed"},
		{"IPv6 loopback", "https://[::1]", "private IP addresses are not allowed"},
		{"Invalid scheme", "file:///etc/passwd", "scheme must be https:// or http://"},
		{"FTP scheme", "ftp://gitlab.com", "scheme must be https:// or http://"},
	}

	for _, tc := range blockedURLs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := gitlab.NewWithURL("gitlab", "test-token", "test/project", tc.url)
			if err == nil {
				t.Errorf("expected %q to be blocked, but got no error", tc.url)
			}
			if !strings.Contains(err.Error(), tc.errSubstr) {
				t.Errorf("expected error containing %q, got: %v", tc.errSubstr, err)
			}
		})
	}
}

func TestNewWithURL_EdgeCases(t *testing.T) {
	edgeCases := []struct {
		name      string
		url       string
		shouldErr bool
	}{
		{"Empty URL defaults", "", false}, // empty should use default
		{"Trailing slash", "https://gitlab.com/", false},
		{"Port number", "https://gitlab.com:443", false},
		{"Path component", "https://gitlab.com/api", false},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := gitlab.NewWithURL("gitlab", "test-token", "test/project", tc.url)
			if tc.shouldErr && err == nil {
				t.Errorf("expected error for %q, got none", tc.url)
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tc.url, err)
			}
		})
	}
}
