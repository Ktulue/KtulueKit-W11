package installer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ktulue/KtulueKit-W11/internal/config"
	"github.com/Ktulue/KtulueKit-W11/internal/reporter"
)

// makeCmd builds a minimal scrape-type Command pointing at the given servers.
func makeCmd(pageURL, pattern string) config.Command {
	return config.Command{
		ID:             "test-tool",
		Name:           "Test Tool",
		Phase:          5,
		Check:          "echo skip",
		ScrapeURL:      pageURL,
		URLPattern:     pattern,
		InstallArgs:    "",
		TimeoutSeconds: 30,
	}
}

// TestScrapeAndInstall_DryRun verifies no network calls happen and StatusDryRun is returned.
func TestScrapeAndInstall_DryRun(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	cmd := makeCmd(srv.URL, `http://.*\.exe`)
	res := ScrapeAndInstall(cmd, true)

	if called {
		t.Error("dry-run: server was called, want no network requests")
	}
	if res.Status != reporter.StatusDryRun {
		t.Errorf("dry-run: want StatusDryRun, got %q", res.Status)
	}
}

// TestScrapeAndInstall_AlreadyInstalled verifies the check short-circuits without network calls.
func TestScrapeAndInstall_AlreadyInstalled(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	cmd := makeCmd(srv.URL, `http://.*\.exe`)
	cmd.Check = "cmd /C exit 0" // always passes — simulates already installed
	res := ScrapeAndInstall(cmd, false)

	if called {
		t.Error("already-installed: server was called, want no network requests")
	}
	if res.Status != reporter.StatusAlready {
		t.Errorf("already-installed: want StatusAlready, got %q", res.Status)
	}
}

// TestScrapeAndInstall_PageFetchFailure verifies failure when the page server returns 500.
func TestScrapeAndInstall_PageFetchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cmd := makeCmd(srv.URL, `http://.*\.exe`)
	cmd.Check = "cmd /C exit 1" // not installed
	res := ScrapeAndInstall(cmd, false)

	if res.Status != reporter.StatusFailed {
		t.Errorf("page fetch failure: want StatusFailed, got %q", res.Status)
	}
	if !strings.Contains(res.Detail, "page") && !strings.Contains(res.Detail, "fetch") && !strings.Contains(res.Detail, "500") {
		t.Errorf("page fetch failure: detail %q should mention fetch/page/500", res.Detail)
	}
}

// TestScrapeAndInstall_NoURLMatch verifies failure when regex finds nothing in the HTML.
func TestScrapeAndInstall_NoURLMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "<html><body>No download link here</body></html>")
	}))
	defer srv.Close()

	cmd := makeCmd(srv.URL, `https://files\.example\.com/tool-[\d]+\.exe`)
	cmd.Check = "cmd /C exit 1"
	res := ScrapeAndInstall(cmd, false)

	if res.Status != reporter.StatusFailed {
		t.Errorf("no URL match: want StatusFailed, got %q", res.Status)
	}
	if !strings.Contains(res.Detail, "no download URL") {
		t.Errorf("no URL match: detail %q should mention 'no download URL'", res.Detail)
	}
}

// TestScrapeAndInstall_DownloadFailure verifies failure when the download server returns 404.
func TestScrapeAndInstall_DownloadFailure(t *testing.T) {
	// Download server — always 404.
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer dlSrv.Close()

	dlURL := dlSrv.URL + "/tool.exe"
	// Page server — embeds the download URL.
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<a href="%s">Download</a>`, dlURL)
	}))
	defer pageSrv.Close()

	pattern := strings.ReplaceAll(dlURL, ".", `\.`) // escape dots for regex
	cmd := makeCmd(pageSrv.URL, pattern)
	cmd.Check = "cmd /C exit 1"
	res := ScrapeAndInstall(cmd, false)

	if res.Status != reporter.StatusFailed {
		t.Errorf("download failure: want StatusFailed, got %q", res.Status)
	}
}

// TestScrapeAndInstall_TempFileCleanup verifies the temp file is removed after a failed download.
func TestScrapeAndInstall_TempFileCleanup(t *testing.T) {
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer dlSrv.Close()

	dlURL := dlSrv.URL + "/tool.exe"
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<a href="%s">Download</a>`, dlURL)
	}))
	defer pageSrv.Close()

	pattern := strings.ReplaceAll(dlURL, ".", `\.`)
	cmd := makeCmd(pageSrv.URL, pattern)
	cmd.Check = "cmd /C exit 1"
	ScrapeAndInstall(cmd, false)

	tempPath := filepath.Join(os.TempDir(), cmd.ID+"-setup.exe")
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Errorf("temp file %q should have been removed after install", tempPath)
		os.Remove(tempPath) // clean up if test fails
	}
}
