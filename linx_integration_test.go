package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const integrationConfigPath = "linx-client.test.conf"

func requireIntegrationConfig(t *testing.T) configFileData {
	t.Helper()

	data, err := os.ReadFile(integrationConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("create %s with siteurl and api key to run integration tests", integrationConfigPath)
		}
		t.Fatalf("read integration config: %v", err)
	}

	var cfg configFileData
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode integration config: %v", err)
	}

	if strings.TrimSpace(cfg.Siteurl) == "" {
		t.Skipf("integration config %s needs siteurl", integrationConfigPath)
	}

	return cfg
}

func setupIntegrationEnv(t *testing.T) {
	t.Helper()

	cfg := requireIntegrationConfig(t)
	Config.siteurl = ensureTrailingSlash(strings.TrimSpace(cfg.Siteurl))
	Config.apikey = strings.TrimSpace(cfg.Apikey)
	keys = make(map[string]string)

	tempDir := t.TempDir()
	Config.logfile = filepath.Join(tempDir, "linx-client-test.log")
	if err := os.MkdirAll(filepath.Dir(Config.logfile), 0o700); err != nil {
		t.Fatalf("prepare log dir: %v", err)
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func requireSingleEntry(t *testing.T) string {
	t.Helper()
	if len(keys) != 1 {
		t.Fatalf("expected single log entry, got %d", len(keys))
	}
	for url := range keys {
		return url
	}
	t.Fatalf("no entry found")
	return ""
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}

	os.Stdout = w
	outputCh := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		r.Close()
		outputCh <- buf.String()
	}()

	fn()

	w.Close()
	os.Stdout = old

	return <-outputCh
}

func TestUploadOverwriteDelete(t *testing.T) {
	setupIntegrationEnv(t)

	file := writeTempFile(t, "an-upload.txt", "integration upload")
	upload(file, "", "", false, 0, false, "", true, false)

	url := requireSingleEntry(t)
	listOut := captureOutput(t, listLogEntries)
	if !strings.Contains(listOut, url) {
		t.Fatalf("list output missing url: %s", url)
	}

	upload(file, "", "", false, 0, true, "", true, false)
	deleteUrl(url)

	if len(keys) != 0 {
		t.Fatalf("expected no keys after delete, got %d", len(keys))
	}
}

func TestUploadWithCustomKeys(t *testing.T) {
	setupIntegrationEnv(t)

	file := writeTempFile(t, "custom.txt", "custom keys")
	upload(file, "custom-delete", "custom-access", false, 0, false, "", true, false)

	url := requireSingleEntry(t)
	if keys[url] == "" {
		t.Fatalf("expected delete key set for %s", url)
	}

	deleteUrl(url)
}

func TestCleanupAndList(t *testing.T) {
	setupIntegrationEnv(t)

	deadURL := fmt.Sprintf("https://up.gn.gy/linx-client-test-cleanup-%d", time.Now().UnixNano())
	keys = map[string]string{deadURL: "deadkey"}

	listOut := captureOutput(t, listLogEntries)
	if !strings.Contains(listOut, deadURL) {
		t.Fatalf("list output missing dead url")
	}

	cleanupOut := captureOutput(t, cleanLogfile)
	if !strings.Contains(cleanupOut, "Removed stale entries") {
		t.Fatalf("expected cleanup to report stale entries, got: %s", cleanupOut)
	}

	if len(keys) != 0 {
		t.Fatalf("expected keys emptied after cleanup, got %d", len(keys))
	}
}
