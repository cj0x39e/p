package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var testBinaryPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "p-cli-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create temp dir:", err)
		os.Exit(1)
	}
	binName := "p"
	if runtime.GOOS == "windows" {
		binName = "p.exe"
	}
	testBinaryPath = filepath.Join(tmpDir, binName)

	cmd := exec.Command("go", "build", "-o", testBinaryPath, ".")
	cmd.Env = append(os.Environ(), "GOCACHE="+filepath.Join(tmpDir, "gocache"))
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to build test binary:", err)
		_ = os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

func runCLI(t *testing.T, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(testBinaryPath, args...)
	cmd.Env = mergeEnv(os.Environ(), env)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cli failed: %v\n%s", err, string(out))
	}
	return string(out)
}

func mergeEnv(base []string, overrides []string) []string {
	merged := make(map[string]string, len(base))
	order := make([]string, 0, len(base))
	for _, entry := range base {
		if entry == "" {
			continue
		}
		key, val, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, exists := merged[key]; !exists {
			order = append(order, key)
		}
		merged[key] = val
	}
	for _, entry := range overrides {
		if entry == "" {
			continue
		}
		key, val, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, exists := merged[key]; !exists {
			order = append(order, key)
		}
		merged[key] = val
	}
	out := make([]string, 0, len(merged))
	for _, key := range order {
		out = append(out, key+"="+merged[key])
	}
	return out
}

func TestCLIOnShFromEnv(t *testing.T) {
	env := []string{
		"HTTP_PROXY=http://127.0.0.1:8080",
		"HTTPS_PROXY=http://127.0.0.1:8443",
		"ALL_PROXY=socks5://127.0.0.1:1080",
	}
	out := runCLI(t, env, "on", "--shell", "sh")
	assertContains(t, out, "export HTTP_PROXY='http://127.0.0.1:8080'")
	assertContains(t, out, "export HTTPS_PROXY='http://127.0.0.1:8443'")
	assertContains(t, out, "export ALL_PROXY='socks5://127.0.0.1:1080'")
}

func TestCLIDetectFromEnv(t *testing.T) {
	env := []string{
		"HTTP_PROXY=http://127.0.0.1:8080",
	}
	out := runCLI(t, env, "detect")
	assertContains(t, out, "Source: env")
	assertContains(t, out, "HTTP: http://127.0.0.1:8080")
}

func TestCLIOnFishFromEnv(t *testing.T) {
	env := []string{
		"HTTP_PROXY=http://127.0.0.1:8080",
	}
	out := runCLI(t, env, "on", "--shell", "fish")
	assertContains(t, out, "set -gx HTTP_PROXY 'http://127.0.0.1:8080';")
}
