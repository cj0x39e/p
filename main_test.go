package main

import (
	"os"
	"strings"
	"testing"
)

func TestDetectFromEnv(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:8080")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:8443")
	t.Setenv("ALL_PROXY", "socks5://127.0.0.1:1080")

	cfg, ok := detectFromEnv()
	if !ok {
		t.Fatalf("expected env detection")
	}
	if cfg.HTTP != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected HTTP: %s", cfg.HTTP)
	}
	if cfg.HTTPS != "http://127.0.0.1:8443" {
		t.Fatalf("unexpected HTTPS: %s", cfg.HTTPS)
	}
	if cfg.ALL != "socks5://127.0.0.1:1080" {
		t.Fatalf("unexpected ALL: %s", cfg.ALL)
	}
}

func TestDetectFromEnvEmpty(t *testing.T) {
	clearProxyEnv(t)
	_, ok := detectFromEnv()
	if ok {
		t.Fatalf("expected no env detection")
	}
}

func TestParseKeyValues(t *testing.T) {
	input := "HTTPEnable : 1\nHTTPProxy : 127.0.0.1\nHTTPPort : 7890\n"
	vals := parseKeyValues(input)
	if vals["HTTPEnable"] != "1" {
		t.Fatalf("unexpected HTTPEnable: %s", vals["HTTPEnable"])
	}
	if vals["HTTPProxy"] != "127.0.0.1" {
		t.Fatalf("unexpected HTTPProxy: %s", vals["HTTPProxy"])
	}
	if vals["HTTPPort"] != "7890" {
		t.Fatalf("unexpected HTTPPort: %s", vals["HTTPPort"])
	}
}

func TestParseWindowsProxy(t *testing.T) {
	var cfg ProxyConfig
	parseWindowsProxy("http=127.0.0.1:8080;https=127.0.0.1:8443;socks=127.0.0.1:1080", &cfg)
	if cfg.HTTP != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected HTTP: %s", cfg.HTTP)
	}
	if cfg.HTTPS != "http://127.0.0.1:8443" {
		t.Fatalf("unexpected HTTPS: %s", cfg.HTTPS)
	}
	if cfg.ALL != "socks5://127.0.0.1:1080" {
		t.Fatalf("unexpected ALL: %s", cfg.ALL)
	}
}

func TestParseClashPorts(t *testing.T) {
	input := `
mixed-port: 7890
port: 7892
socks-port: 7891
bind-address: 0.0.0.0
`
	ports := parseClashPorts([]byte(input))
	if ports.MixedPort != 7890 {
		t.Fatalf("unexpected mixed port: %d", ports.MixedPort)
	}
	if ports.HTTPPort != 7892 {
		t.Fatalf("unexpected http port: %d", ports.HTTPPort)
	}
	if ports.SOCKSPort != 7891 {
		t.Fatalf("unexpected socks port: %d", ports.SOCKSPort)
	}
	if ports.BindAddress != "0.0.0.0" {
		t.Fatalf("unexpected bind address: %s", ports.BindAddress)
	}
}

func TestParseSurgePorts(t *testing.T) {
	input := `
[General]
http-listen = 127.0.0.1:6152
socks5-listen = 127.0.0.1:6153
`
	ports := parseSurgePorts([]byte(input))
	if ports.HTTPPort != 6152 {
		t.Fatalf("unexpected http port: %d", ports.HTTPPort)
	}
	if ports.SOCKSPort != 6153 {
		t.Fatalf("unexpected socks port: %d", ports.SOCKSPort)
	}
}

func TestParseJSONPorts(t *testing.T) {
	input := `{"localPort":1080,"localHttpPort":1087}`
	ports := parseJSONPorts([]byte(input))
	if ports.SOCKSPort != 1080 {
		t.Fatalf("unexpected socks port: %d", ports.SOCKSPort)
	}
	if ports.HTTPPort != 1087 {
		t.Fatalf("unexpected http port: %d", ports.HTTPPort)
	}
}

func TestParseInboundsPorts(t *testing.T) {
	input := `{"inbounds":[{"protocol":"socks","port":1080},{"protocol":"http","port":10809}]}`
	ports := parseInboundsPorts([]byte(input))
	if ports.SOCKSPort != 1080 {
		t.Fatalf("unexpected socks port: %d", ports.SOCKSPort)
	}
	if ports.HTTPPort != 10809 {
		t.Fatalf("unexpected http port: %d", ports.HTTPPort)
	}
}

func TestRenderShSet(t *testing.T) {
	cfg := ProxyConfig{
		HTTP:  "http://127.0.0.1:8080",
		HTTPS: "http://127.0.0.1:8443",
		ALL:   "socks5://127.0.0.1:1080",
	}
	out := renderOn("sh", cfg)
	assertContains(t, out, "export HTTP_PROXY='http://127.0.0.1:8080'")
	assertContains(t, out, "export HTTPS_PROXY='http://127.0.0.1:8443'")
	assertContains(t, out, "export ALL_PROXY='socks5://127.0.0.1:1080'")
	assertContains(t, out, "export http_proxy='http://127.0.0.1:8080'")
	assertContains(t, out, "export https_proxy='http://127.0.0.1:8443'")
	assertContains(t, out, "export all_proxy='socks5://127.0.0.1:1080'")
}

func TestRenderOff(t *testing.T) {
	out := renderOff("sh")
	assertContains(t, out, "unset HTTP_PROXY")
	assertContains(t, out, "unset HTTPS_PROXY")
	assertContains(t, out, "unset ALL_PROXY")
	assertContains(t, out, "unset NO_PROXY")
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("a'b")
	if got != "'a'\\''b'" {
		t.Fatalf("unexpected shellQuote: %s", got)
	}
}

func TestPSQuote(t *testing.T) {
	got := psQuote(`a"b`)
	if got != "\"a`\"b\"" {
		t.Fatalf("unexpected psQuote: %s", got)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected %q to contain %q", s, substr)
	}
}

func clearProxyEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"HTTP_PROXY", "http_proxy",
		"HTTPS_PROXY", "https_proxy",
		"ALL_PROXY", "all_proxy",
		"NO_PROXY", "no_proxy",
	} {
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("failed to unset %s: %v", k, err)
		}
	}
}
