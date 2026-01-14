package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var execCommand = exec.Command
var execLookPath = exec.LookPath

type ProxyConfig struct {
	HTTP   string
	HTTPS  string
	ALL    string
	Source string
	Notes  []string
}

func main() {
	args := os.Args[1:]
	cmd := "copy"
	if len(args) > 0 {
		cmd = strings.ToLower(args[0])
	}

	shell := detectShell()
	if len(args) > 1 {
		for i := 1; i < len(args); i++ {
			if args[i] == "--shell" && i+1 < len(args) {
				shell = strings.ToLower(args[i+1])
				i++
			}
		}
	}

	if isHelpArgs(args) {
		printHelp(os.Stderr)
		return
	}

	switch cmd {
	case "copy", "":
		cfg := detectProxy()
		script := renderOn(shell, cfg)
		if err := copyToClipboard(script); err != nil {
			fmt.Print(script)
		} else {
			fmt.Fprintln(os.Stderr, "Copied proxy env to clipboard. Paste and run to apply.")
		}
	case "on":
		cfg := detectProxy()
		fmt.Print(renderOn(shell, cfg))
	case "off":
		script := renderOff(shell)
		if err := copyToClipboard(script); err != nil {
			fmt.Print(script)
		} else {
			fmt.Fprintln(os.Stderr, "Copied proxy unset to clipboard. Paste and run to apply.")
		}
	case "status":
		cfg := detectProxy()
		printStatus(cfg)
	case "detect":
		cfg := detectProxy()
		printDetect(cfg)
	case "test":
		cfg := detectProxy()
		if err := testProxy(cfg); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printHelp(os.Stderr)
		os.Exit(1)
	}
}

func printHelp(out *os.File) {
	fmt.Fprintln(out, "p - proxy helper")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  p                # copy shell commands to clipboard")
	fmt.Fprintln(out, "  p on             # output shell commands to enable proxy")
	fmt.Fprintln(out, "  p off            # output shell commands to disable proxy")
	fmt.Fprintln(out, "  p status         # show detected proxy")
	fmt.Fprintln(out, "  p detect         # show detection details")
	fmt.Fprintln(out, "  p test           # test proxy with curl to google.com")
	fmt.Fprintln(out, "  p --shell sh     # force output shell (sh|fish|ps)")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Example:")
	fmt.Fprintln(out, "  p")
	fmt.Fprintln(out, "  eval \"$(p on)\"")
}

func detectShell() string {
	if runtime.GOOS == "windows" {
		return "ps"
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		base := filepath.Base(sh)
		switch base {
		case "fish":
			return "fish"
		case "bash", "zsh", "sh":
			return "sh"
		}
	}
	return "sh"
}

func detectProxy() ProxyConfig {
	if cfg, ok := detectFromEnv(); ok {
		return cfg
	}
	if cfg, ok := detectFromSystem(); ok {
		return cfg
	}
	if cfg, ok := detectFromApps(); ok {
		if cfg.HTTP == "" && cfg.HTTPS == "" {
			cfg = mergeProxyConfig(cfg, detectFromPorts())
		}
		return cfg
	}
	return detectFromPorts()
}

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		if path, _ := exec.LookPath("wl-copy"); path != "" {
			cmd = exec.Command("wl-copy")
		} else if path, _ := exec.LookPath("xclip"); path != "" {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			return fmt.Errorf("no clipboard tool found")
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte(text)); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return err
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func detectFromEnv() (ProxyConfig, bool) {
	cfg := ProxyConfig{Source: "env"}
	cfg.HTTP = firstEnv("HTTP_PROXY", "http_proxy")
	cfg.HTTPS = firstEnv("HTTPS_PROXY", "https_proxy")
	cfg.ALL = firstEnv("ALL_PROXY", "all_proxy")
	if cfg.HTTP == "" && cfg.HTTPS == "" && cfg.ALL == "" {
		return ProxyConfig{}, false
	}
	cfg.Notes = append(cfg.Notes, "env vars detected")
	return cfg, true
}

func detectFromSystem() (ProxyConfig, bool) {
	switch runtime.GOOS {
	case "darwin":
		return detectFromMacOS()
	case "windows":
		return detectFromWindows()
	case "linux":
		return detectFromGSettings()
	default:
		return ProxyConfig{}, false
	}
}

func detectFromMacOS() (ProxyConfig, bool) {
	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return ProxyConfig{}, false
	}

	cfg := ProxyConfig{Source: "macos"}
	vals := parseKeyValues(string(out))

	httpEnabled := vals["HTTPEnable"] == "1"
	httpsEnabled := vals["HTTPSEnable"] == "1"
	socksEnabled := vals["SOCKSEnable"] == "1"

	if httpEnabled {
		cfg.HTTP = hostPort(vals["HTTPProxy"], vals["HTTPPort"], "http")
	}
	if httpsEnabled {
		cfg.HTTPS = hostPort(vals["HTTPSProxy"], vals["HTTPSPort"], "http")
	}
	if socksEnabled {
		cfg.ALL = hostPort(vals["SOCKSProxy"], vals["SOCKSPort"], "socks5")
	}

	if cfg.HTTP == "" && cfg.HTTPS == "" && cfg.ALL == "" {
		return ProxyConfig{}, false
	}
	cfg.Notes = append(cfg.Notes, "scutil --proxy")
	return cfg, true
}

func detectFromWindows() (ProxyConfig, bool) {
	out, err := exec.Command("netsh", "winhttp", "show", "proxy").Output()
	if err != nil {
		return ProxyConfig{}, false
	}
	text := string(out)
	if strings.Contains(text, "Direct access") {
		return ProxyConfig{}, false
	}

	cfg := ProxyConfig{Source: "windows"}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Proxy Server(s)") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			parseWindowsProxy(parts[1], &cfg)
		}
	}

	if cfg.HTTP == "" && cfg.HTTPS == "" && cfg.ALL == "" {
		return ProxyConfig{}, false
	}
	cfg.Notes = append(cfg.Notes, "netsh winhttp show proxy")
	return cfg, true
}

func detectFromGSettings() (ProxyConfig, bool) {
	modeOut, err := exec.Command("gsettings", "get", "org.gnome.system.proxy", "mode").Output()
	if err != nil {
		return ProxyConfig{}, false
	}
	mode := strings.Trim(strings.TrimSpace(string(modeOut)), "'")
	if mode != "manual" {
		return ProxyConfig{}, false
	}

	cfg := ProxyConfig{Source: "gnome"}
	if hp := gsettingsHostPort("org.gnome.system.proxy.http", "host", "port"); hp != "" {
		cfg.HTTP = "http://" + hp
	}
	if hp := gsettingsHostPort("org.gnome.system.proxy.https", "host", "port"); hp != "" {
		cfg.HTTPS = "http://" + hp
	}
	if hp := gsettingsHostPort("org.gnome.system.proxy.socks", "host", "port"); hp != "" {
		cfg.ALL = "socks5://" + hp
	}

	if cfg.HTTP == "" && cfg.HTTPS == "" && cfg.ALL == "" {
		return ProxyConfig{}, false
	}
	cfg.Notes = append(cfg.Notes, "gsettings org.gnome.system.proxy")
	return cfg, true
}

func detectFromPorts() ProxyConfig {
	cfg := ProxyConfig{Source: "ports"}
	candidatesHTTP := []int{7890, 7891, 7892, 8001, 8080, 8118, 8888, 2080, 3128, 6152, 1087, 10809}
	candidatesSOCKS := []int{1080, 1081, 10808, 7892, 6153}

	for _, port := range candidatesHTTP {
		if portOpen("127.0.0.1", port) {
			cfg.HTTP = fmt.Sprintf("http://127.0.0.1:%d", port)
			cfg.HTTPS = cfg.HTTP
			cfg.Notes = append(cfg.Notes, fmt.Sprintf("port %d open", port))
			break
		}
	}
	for _, port := range candidatesSOCKS {
		if portOpen("127.0.0.1", port) {
			cfg.ALL = fmt.Sprintf("socks5://127.0.0.1:%d", port)
			cfg.Notes = append(cfg.Notes, fmt.Sprintf("port %d open", port))
			break
		}
	}

	return cfg
}

func mergeProxyConfig(base ProxyConfig, extra ProxyConfig) ProxyConfig {
	if base.HTTP == "" && extra.HTTP != "" {
		base.HTTP = extra.HTTP
	}
	if base.HTTPS == "" && extra.HTTPS != "" {
		base.HTTPS = extra.HTTPS
	}
	if base.ALL == "" && extra.ALL != "" {
		base.ALL = extra.ALL
	}
	if len(extra.Notes) > 0 {
		base.Notes = append(base.Notes, "ports fallback")
	}
	return base
}

func detectFromApps() (ProxyConfig, bool) {
	detectors := []func() (ProxyConfig, bool){
		detectFromClashFamily,
		detectFromSurge,
		detectFromShadowsocks,
		detectFromShadowsocksR,
		detectFromV2RayN,
		detectFromTrojan,
		detectFromSingBox,
		detectFromV2RayCore,
	}

	for _, detect := range detectors {
		if cfg, ok := detect(); ok {
			return cfg, true
		}
	}
	return ProxyConfig{}, false
}

func detectFromClashFamily() (ProxyConfig, bool) {
	paths := clashConfigPaths()
	return detectFromClashConfigs(paths, "app:clash")
}

func detectFromSurge() (ProxyConfig, bool) {
	if runtime.GOOS != "darwin" {
		return ProxyConfig{}, false
	}
	paths := surgeConfigPaths()
	return detectFromSurgeConfigs(paths)
}

func detectFromShadowsocks() (ProxyConfig, bool) {
	paths := shadowsocksConfigPaths()
	return detectFromShadowsocksConfigs(paths)
}

func detectFromShadowsocksR() (ProxyConfig, bool) {
	paths := shadowsocksRConfigPaths()
	return detectFromShadowsocksRConfigs(paths)
}

func detectFromV2RayN() (ProxyConfig, bool) {
	if runtime.GOOS != "windows" {
		return ProxyConfig{}, false
	}
	paths := v2rayNConfigPaths()
	return detectFromV2RayNConfigs(paths)
}

func detectFromSingBox() (ProxyConfig, bool) {
	paths := singBoxConfigPaths()
	return detectFromSingBoxConfigs(paths)
}

func detectFromTrojan() (ProxyConfig, bool) {
	paths := trojanConfigPaths()
	return detectFromTrojanConfigs(paths)
}

func detectFromV2RayCore() (ProxyConfig, bool) {
	paths := v2rayCoreConfigPaths()
	return detectFromV2RayCoreConfigs(paths)
}

func detectFromClashConfigs(paths []string, source string) (ProxyConfig, bool) {
	for _, path := range expandPaths(paths) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ports := parseClashPorts(data)
		if !ports.hasAny() {
			continue
		}
		host := normalizeListenHost(ports.BindAddress)
		cfg := ProxyConfig{Source: source}
		if ports.MixedPort > 0 {
			cfg.HTTP = fmt.Sprintf("http://%s:%d", host, ports.MixedPort)
			cfg.HTTPS = cfg.HTTP
			cfg.ALL = fmt.Sprintf("socks5://%s:%d", host, ports.MixedPort)
			cfg.Notes = append(cfg.Notes, "mixed-port from "+filepath.Base(path))
		} else {
			if ports.HTTPPort > 0 {
				cfg.HTTP = fmt.Sprintf("http://%s:%d", host, ports.HTTPPort)
				cfg.HTTPS = cfg.HTTP
				cfg.Notes = append(cfg.Notes, "port from "+filepath.Base(path))
			}
			if ports.SOCKSPort > 0 {
				cfg.ALL = fmt.Sprintf("socks5://%s:%d", host, ports.SOCKSPort)
				cfg.Notes = append(cfg.Notes, "socks-port from "+filepath.Base(path))
			}
		}
		return cfg, true
	}
	return ProxyConfig{}, false
}

func detectFromSurgeConfigs(paths []string) (ProxyConfig, bool) {
	for _, path := range expandPaths(paths) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ports := parseSurgePorts(data)
		if !ports.hasAny() {
			continue
		}
		host := normalizeListenHost(ports.BindAddress)
		cfg := ProxyConfig{Source: "app:surge"}
		if ports.MixedPort > 0 {
			cfg.HTTP = fmt.Sprintf("http://%s:%d", host, ports.MixedPort)
			cfg.HTTPS = cfg.HTTP
			cfg.ALL = fmt.Sprintf("socks5://%s:%d", host, ports.MixedPort)
			cfg.Notes = append(cfg.Notes, "mixed-listen from "+filepath.Base(path))
		} else {
			if ports.HTTPPort > 0 {
				cfg.HTTP = fmt.Sprintf("http://%s:%d", host, ports.HTTPPort)
				cfg.HTTPS = cfg.HTTP
				cfg.Notes = append(cfg.Notes, "http-listen from "+filepath.Base(path))
			}
			if ports.SOCKSPort > 0 {
				cfg.ALL = fmt.Sprintf("socks5://%s:%d", host, ports.SOCKSPort)
				cfg.Notes = append(cfg.Notes, "socks5-listen from "+filepath.Base(path))
			}
		}
		return cfg, true
	}
	return ProxyConfig{}, false
}

func detectFromShadowsocksConfigs(paths []string) (ProxyConfig, bool) {
	for _, path := range expandPaths(paths) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ports := parseJSONPorts(data)
		if !ports.hasAny() {
			continue
		}
		cfg := ProxyConfig{Source: "app:shadowsocks"}
		if ports.HTTPPort > 0 {
			cfg.HTTP = fmt.Sprintf("http://127.0.0.1:%d", ports.HTTPPort)
			cfg.HTTPS = cfg.HTTP
		}
		if ports.SOCKSPort > 0 {
			cfg.ALL = fmt.Sprintf("socks5://127.0.0.1:%d", ports.SOCKSPort)
		}
		cfg.Notes = append(cfg.Notes, "config "+filepath.Base(path))
		return cfg, true
	}
	return ProxyConfig{}, false
}

func detectFromV2RayNConfigs(paths []string) (ProxyConfig, bool) {
	for _, path := range expandPaths(paths) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ports := parseJSONPorts(data)
		if !ports.hasAny() {
			continue
		}
		cfg := ProxyConfig{Source: "app:v2rayn"}
		if ports.HTTPPort > 0 {
			cfg.HTTP = fmt.Sprintf("http://127.0.0.1:%d", ports.HTTPPort)
			cfg.HTTPS = cfg.HTTP
		}
		if ports.SOCKSPort > 0 {
			cfg.ALL = fmt.Sprintf("socks5://127.0.0.1:%d", ports.SOCKSPort)
		}
		cfg.Notes = append(cfg.Notes, "config "+filepath.Base(path))
		return cfg, true
	}
	return ProxyConfig{}, false
}

func detectFromShadowsocksRConfigs(paths []string) (ProxyConfig, bool) {
	for _, path := range expandPaths(paths) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ports := parseJSONPorts(data)
		if !ports.hasAny() {
			continue
		}
		cfg := ProxyConfig{Source: "app:shadowsocksr"}
		if ports.HTTPPort > 0 {
			cfg.HTTP = fmt.Sprintf("http://127.0.0.1:%d", ports.HTTPPort)
			cfg.HTTPS = cfg.HTTP
		}
		if ports.SOCKSPort > 0 {
			cfg.ALL = fmt.Sprintf("socks5://127.0.0.1:%d", ports.SOCKSPort)
		}
		cfg.Notes = append(cfg.Notes, "config "+filepath.Base(path))
		return cfg, true
	}
	return ProxyConfig{}, false
}

func detectFromTrojanConfigs(paths []string) (ProxyConfig, bool) {
	for _, path := range expandPaths(paths) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ports := parseJSONPorts(data)
		if !ports.hasAny() {
			continue
		}
		cfg := ProxyConfig{Source: "app:trojan"}
		if ports.HTTPPort > 0 {
			cfg.HTTP = fmt.Sprintf("http://127.0.0.1:%d", ports.HTTPPort)
			cfg.HTTPS = cfg.HTTP
		}
		if ports.SOCKSPort > 0 {
			cfg.ALL = fmt.Sprintf("socks5://127.0.0.1:%d", ports.SOCKSPort)
		}
		cfg.Notes = append(cfg.Notes, "config "+filepath.Base(path))
		return cfg, true
	}
	return ProxyConfig{}, false
}

func detectFromSingBoxConfigs(paths []string) (ProxyConfig, bool) {
	for _, path := range expandPaths(paths) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ports := parseInboundsPorts(data)
		if !ports.hasAny() {
			continue
		}
		cfg := ProxyConfig{Source: "app:sing-box"}
		if ports.HTTPPort > 0 {
			cfg.HTTP = fmt.Sprintf("http://127.0.0.1:%d", ports.HTTPPort)
			cfg.HTTPS = cfg.HTTP
		}
		if ports.SOCKSPort > 0 {
			cfg.ALL = fmt.Sprintf("socks5://127.0.0.1:%d", ports.SOCKSPort)
		}
		cfg.Notes = append(cfg.Notes, "config "+filepath.Base(path))
		return cfg, true
	}
	return ProxyConfig{}, false
}

func detectFromV2RayCoreConfigs(paths []string) (ProxyConfig, bool) {
	for _, path := range expandPaths(paths) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		ports := parseInboundsPorts(data)
		if !ports.hasAny() {
			continue
		}
		cfg := ProxyConfig{Source: "app:v2ray"}
		if ports.HTTPPort > 0 {
			cfg.HTTP = fmt.Sprintf("http://127.0.0.1:%d", ports.HTTPPort)
			cfg.HTTPS = cfg.HTTP
		}
		if ports.SOCKSPort > 0 {
			cfg.ALL = fmt.Sprintf("socks5://127.0.0.1:%d", ports.SOCKSPort)
		}
		cfg.Notes = append(cfg.Notes, "config "+filepath.Base(path))
		return cfg, true
	}
	return ProxyConfig{}, false
}

func portOpen(host string, port int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func parseKeyValues(input string) map[string]string {
	vals := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		vals[key] = val
	}
	return vals
}

func hostPort(host string, port string, scheme string) string {
	if host == "" || port == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

func parseWindowsProxy(value string, cfg *ProxyConfig) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	parts := strings.Split(value, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			proto := strings.ToLower(strings.TrimSpace(kv[0]))
			addr := strings.TrimSpace(kv[1])
			switch proto {
			case "http":
				cfg.HTTP = normalizeAddr("http", addr)
			case "https":
				cfg.HTTPS = normalizeAddr("http", addr)
			case "socks":
				cfg.ALL = normalizeAddr("socks5", addr)
			}
		} else {
			cfg.HTTP = normalizeAddr("http", part)
			cfg.HTTPS = normalizeAddr("http", part)
		}
	}
}

func normalizeAddr(scheme, addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") || strings.HasPrefix(addr, "socks5://") {
		return addr
	}
	return scheme + "://" + addr
}

func gsettingsHostPort(schema, hostKey, portKey string) string {
	hostOut, err := exec.Command("gsettings", "get", schema, hostKey).Output()
	if err != nil {
		return ""
	}
	portOut, err := exec.Command("gsettings", "get", schema, portKey).Output()
	if err != nil {
		return ""
	}

	host := strings.Trim(strings.TrimSpace(string(hostOut)), "'")
	portStr := strings.TrimSpace(string(portOut))
	if host == "" || host == "none" {
		return ""
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func renderOn(shell string, cfg ProxyConfig) string {
	if cfg.HTTP == "" && cfg.HTTPS == "" && cfg.ALL == "" {
		return renderOff(shell)
	}
	set := map[string]string{}
	if cfg.HTTP != "" {
		set["HTTP_PROXY"] = cfg.HTTP
		set["http_proxy"] = cfg.HTTP
	}
	if cfg.HTTPS != "" {
		set["HTTPS_PROXY"] = cfg.HTTPS
		set["https_proxy"] = cfg.HTTPS
	}
	if cfg.ALL != "" {
		set["ALL_PROXY"] = cfg.ALL
		set["all_proxy"] = cfg.ALL
	}

	switch shell {
	case "fish":
		return renderFishSet(set)
	case "ps":
		return renderPSSet(set)
	default:
		return renderShSet(set)
	}
}

func renderOff(shell string) string {
	unset := []string{
		"HTTP_PROXY", "http_proxy",
		"HTTPS_PROXY", "https_proxy",
		"ALL_PROXY", "all_proxy",
		"NO_PROXY", "no_proxy",
	}

	switch shell {
	case "fish":
		return renderFishUnset(unset)
	case "ps":
		return renderPSUnset(unset)
	default:
		return renderShUnset(unset)
	}
}

func testProxy(cfg ProxyConfig) error {
	if cfg.HTTP == "" && cfg.HTTPS == "" && cfg.ALL == "" {
		return fmt.Errorf("no proxy detected")
	}
	if _, err := execLookPath("curl"); err != nil {
		return fmt.Errorf("curl not found in PATH")
	}
	url := "https://www.google.com/generate_204"
	fmt.Fprintf(os.Stderr, "Testing proxy with curl: %s\n", url)
	cmd := execCommand("curl", "-I", "-L", "--connect-timeout", "5", "--max-time", "10", url)
	cmd.Env = mergeEnv(os.Environ(), proxyEnv(cfg))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func proxyEnv(cfg ProxyConfig) []string {
	var env []string
	if cfg.HTTP != "" {
		env = append(env, "HTTP_PROXY="+cfg.HTTP, "http_proxy="+cfg.HTTP)
	}
	if cfg.HTTPS != "" {
		env = append(env, "HTTPS_PROXY="+cfg.HTTPS, "https_proxy="+cfg.HTTPS)
	}
	if cfg.ALL != "" {
		env = append(env, "ALL_PROXY="+cfg.ALL, "all_proxy="+cfg.ALL)
	}
	env = append(env, "NO_PROXY=", "no_proxy=")
	return env
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

func renderShSet(values map[string]string) string {
	var b strings.Builder
	for k, v := range values {
		b.WriteString(fmt.Sprintf("export %s=%s\n", k, shellQuote(v)))
	}
	return b.String()
}

func renderShUnset(keys []string) string {
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("unset %s\n", k))
	}
	return b.String()
}

func renderFishSet(values map[string]string) string {
	var b strings.Builder
	for k, v := range values {
		b.WriteString(fmt.Sprintf("set -gx %s %s;\n", k, shellQuote(v)))
	}
	return b.String()
}

func renderFishUnset(keys []string) string {
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("set -e %s;\n", k))
	}
	return b.String()
}

func renderPSSet(values map[string]string) string {
	var b strings.Builder
	for k, v := range values {
		b.WriteString(fmt.Sprintf("$env:%s=%s\n", k, psQuote(v)))
	}
	return b.String()
}

func renderPSUnset(keys []string) string {
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("Remove-Item Env:%s -ErrorAction SilentlyContinue\n", k))
	}
	return b.String()
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func psQuote(value string) string {
	return "\"" + strings.ReplaceAll(value, "\"", "`\"") + "\""
}

func printStatus(cfg ProxyConfig) {
	if cfg.HTTP == "" && cfg.HTTPS == "" && cfg.ALL == "" {
		fmt.Println("No proxy detected.")
		fmt.Println("Hint: eval \"$(p)\" will output proxy envs when detected.")
		return
	}
	printDetect(cfg)
	fmt.Println("")
	fmt.Println("Enable in current shell:")
	fmt.Println("  eval \"$(p)\"")
}

func printDetect(cfg ProxyConfig) {
	fmt.Printf("Source: %s\n", cfg.Source)
	if cfg.HTTP != "" {
		fmt.Printf("HTTP: %s\n", cfg.HTTP)
	}
	if cfg.HTTPS != "" {
		fmt.Printf("HTTPS: %s\n", cfg.HTTPS)
	}
	if cfg.ALL != "" {
		fmt.Printf("ALL: %s\n", cfg.ALL)
	}
	if len(cfg.Notes) > 0 {
		fmt.Printf("Notes: %s\n", strings.Join(cfg.Notes, ", "))
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return ""
}

func isHelpArgs(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-h", "--help", "help":
			return true
		}
	}
	return false
}

type ProxyPorts struct {
	HTTPPort    int
	SOCKSPort   int
	MixedPort   int
	BindAddress string
}

func (p ProxyPorts) hasAny() bool {
	return p.HTTPPort > 0 || p.SOCKSPort > 0 || p.MixedPort > 0
}

func parseClashPorts(data []byte) ProxyPorts {
	var ports ProxyPorts
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := parseKeyValueLine(line, ":")
		if !ok {
			continue
		}
		switch key {
		case "mixed-port":
			ports.MixedPort = parsePortValue(val)
		case "port":
			ports.HTTPPort = parsePortValue(val)
		case "socks-port":
			ports.SOCKSPort = parsePortValue(val)
		case "bind-address":
			ports.BindAddress = cleanValue(val)
		}
	}
	return ports
}

func parseSurgePorts(data []byte) ProxyPorts {
	var ports ProxyPorts
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, val, ok := parseKeyValueLine(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "mixed-listen":
			host, port := parseListenAddr(val)
			if port > 0 {
				ports.MixedPort = port
				ports.BindAddress = host
			}
		case "http-listen":
			host, port := parseListenAddr(val)
			if port > 0 {
				ports.HTTPPort = port
				ports.BindAddress = host
			}
		case "socks5-listen":
			host, port := parseListenAddr(val)
			if port > 0 {
				ports.SOCKSPort = port
				ports.BindAddress = host
			}
		}
	}
	return ports
}

func parseJSONPorts(data []byte) ProxyPorts {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return ProxyPorts{}
	}

	var ports ProxyPorts
	if v := intFromMap(root, "localHttpPort", "httpPort"); v > 0 {
		ports.HTTPPort = v
	}
	if v := intFromMap(root, "localSocksPort", "socksPort", "localPort"); v > 0 {
		ports.SOCKSPort = v
	}
	if ports.hasAny() {
		return ports
	}
	return parseInboundsPortsFromMap(root)
}

func parseInboundsPorts(data []byte) ProxyPorts {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return ProxyPorts{}
	}
	return parseInboundsPortsFromMap(root)
}

func parseInboundsPortsFromMap(root map[string]any) ProxyPorts {
	var ports ProxyPorts
	inbounds, ok := root["inbounds"].([]any)
	if !ok {
		return ports
	}
	for _, item := range inbounds {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		proto := stringFromMap(entry, "protocol", "type")
		port := intFromMap(entry, "port", "listen_port")
		switch strings.ToLower(proto) {
		case "http":
			if ports.HTTPPort == 0 && port > 0 {
				ports.HTTPPort = port
			}
		case "socks", "socks5":
			if ports.SOCKSPort == 0 && port > 0 {
				ports.SOCKSPort = port
			}
		}
	}
	return ports
}

func parseKeyValueLine(line string, sep string) (string, string, bool) {
	parts := strings.SplitN(line, sep, 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	return key, val, true
}

func cleanValue(val string) string {
	if idx := strings.Index(val, "#"); idx >= 0 {
		val = val[:idx]
	}
	val = strings.TrimSpace(val)
	val = strings.Trim(val, `"'`)
	return val
}

func parsePortValue(val string) int {
	val = cleanValue(val)
	if val == "" {
		return 0
	}
	if n, err := strconv.Atoi(val); err == nil {
		return n
	}
	return parseTrailingPort(val)
}

func parseListenAddr(val string) (string, int) {
	val = cleanValue(val)
	if val == "" {
		return "", 0
	}
	if strings.Contains(val, "://") {
		if _, rest, ok := strings.Cut(val, "://"); ok {
			val = rest
		}
	}
	if strings.Contains(val, ":") {
		host, portStr := splitHostPort(val)
		if portStr == "" {
			return "", 0
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return "", 0
		}
		return normalizeListenHost(host), port
	}
	port, err := strconv.Atoi(val)
	if err != nil {
		return "", 0
	}
	return "127.0.0.1", port
}

func splitHostPort(val string) (string, string) {
	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "[") && strings.Contains(val, "]") {
		host, port, err := net.SplitHostPort(val)
		if err == nil {
			return strings.Trim(host, "[]"), port
		}
	}
	if idx := strings.LastIndex(val, ":"); idx >= 0 {
		return strings.TrimSpace(val[:idx]), strings.TrimSpace(val[idx+1:])
	}
	return val, ""
}

func normalizeListenHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" || host == "*" {
		return "127.0.0.1"
	}
	return host
}

func parseTrailingPort(val string) int {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	idx := strings.LastIndex(val, ":")
	if idx < 0 || idx+1 >= len(val) {
		return 0
	}
	portStr := strings.TrimSpace(val[idx+1:])
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}

func intFromMap(m map[string]any, keys ...string) int {
	for _, key := range keys {
		val, ok := m[key]
		if !ok {
			continue
		}
		switch v := val.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
	}
	return 0
}

func stringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		val, ok := m[key]
		if !ok {
			continue
		}
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func expandPaths(paths []string) []string {
	var out []string
	for _, raw := range paths {
		expanded := expandUser(raw)
		if hasGlob(expanded) {
			matches, err := filepath.Glob(expanded)
			if err != nil || len(matches) == 0 {
				continue
			}
			out = append(out, matches...)
			continue
		}
		out = append(out, expanded)
	}
	return out
}

func expandUser(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

func hasGlob(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func clashConfigPaths() []string {
	var paths []string
	home := userHome()
	if home != "" {
		paths = append(paths,
			filepath.Join(home, ".config", "clash", "config.yaml"),
			filepath.Join(home, ".config", "clash", "config.yml"),
			filepath.Join(home, ".config", "mihomo", "config.yaml"),
			filepath.Join(home, ".config", "mihomo", "config.yml"),
			filepath.Join(home, ".config", "clash-verge", "config.yaml"),
			filepath.Join(home, ".config", "clash-verge", "config.yml"),
			filepath.Join(home, ".config", "clash-verge-rev", "config.yaml"),
			filepath.Join(home, ".config", "clash-verge-rev", "config.yml"),
		)
	}
	switch runtime.GOOS {
	case "darwin":
		if home != "" {
			paths = append(paths,
				filepath.Join(home, "Library", "Application Support", "ClashX", "config.yaml"),
				filepath.Join(home, "Library", "Application Support", "ClashX", "config.yml"),
				filepath.Join(home, "Library", "Application Support", "ClashX", "Profiles", "*.yaml"),
				filepath.Join(home, "Library", "Application Support", "ClashX", "Profiles", "*.yml"),
				filepath.Join(home, "Library", "Application Support", "Clash for Windows", "config.yaml"),
				filepath.Join(home, "Library", "Application Support", "Clash for Windows", "config.yml"),
				filepath.Join(home, "Library", "Application Support", "Clash Verge", "config.yaml"),
				filepath.Join(home, "Library", "Application Support", "Clash Verge", "config.yml"),
				filepath.Join(home, "Library", "Application Support", "Clash Verge Rev", "config.yaml"),
				filepath.Join(home, "Library", "Application Support", "Clash Verge Rev", "config.yml"),
			)
		}
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			paths = append(paths,
				filepath.Join(appdata, "Clash for Windows", "config.yaml"),
				filepath.Join(appdata, "Clash for Windows", "config.yml"),
				filepath.Join(appdata, "Clash Verge", "config.yaml"),
				filepath.Join(appdata, "Clash Verge", "config.yml"),
				filepath.Join(appdata, "clash-verge", "config.yaml"),
				filepath.Join(appdata, "clash-verge-rev", "config.yaml"),
				filepath.Join(appdata, "Clash", "config.yaml"),
				filepath.Join(appdata, "Clash", "config.yml"),
			)
		}
		if home != "" {
			paths = append(paths,
				filepath.Join(home, ".config", "clash", "config.yaml"),
				filepath.Join(home, ".config", "clash", "config.yml"),
			)
		}
	case "linux":
		paths = append(paths,
			"/etc/clash/config.yaml",
			"/etc/clash/config.yml",
		)
	}
	return paths
}

func surgeConfigPaths() []string {
	home := userHome()
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, "Library", "Application Support", "Surge", "Profiles", "*.conf"),
		filepath.Join(home, "Library", "Application Support", "Surge", "Profiles", "*.txt"),
	}
}

func shadowsocksConfigPaths() []string {
	var paths []string
	home := userHome()
	if home != "" {
		paths = append(paths,
			filepath.Join(home, "Library", "Application Support", "ShadowsocksX-NG", "config.json"),
			filepath.Join(home, "Library", "Application Support", "ShadowsocksX-NG", "gui-config.json"),
		)
	}
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			paths = append(paths,
				filepath.Join(appdata, "Shadowsocks", "gui-config.json"),
				filepath.Join(appdata, "Shadowsocks", "config.json"),
			)
		}
	}
	if runtime.GOOS == "linux" {
		paths = append(paths,
			"/etc/shadowsocks/config.json",
			"/etc/shadowsocks.json",
		)
	}
	return paths
}

func shadowsocksRConfigPaths() []string {
	var paths []string
	home := userHome()
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			paths = append(paths,
				filepath.Join(appdata, "ShadowsocksR", "gui-config.json"),
				filepath.Join(appdata, "ShadowsocksR", "config.json"),
			)
		}
	}
	if runtime.GOOS == "linux" {
		paths = append(paths,
			"/etc/shadowsocksr/config.json",
			"/etc/ssr/config.json",
		)
	}
	if home != "" {
		paths = append(paths,
			filepath.Join(home, ".config", "shadowsocksr", "config.json"),
		)
	}
	return paths
}

func v2rayNConfigPaths() []string {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return nil
	}
	return []string{
		filepath.Join(appdata, "v2rayN", "guiNConfig.json"),
		filepath.Join(appdata, "v2rayN", "gui-config.json"),
		filepath.Join(appdata, "v2rayN", "config.json"),
	}
}

func singBoxConfigPaths() []string {
	var paths []string
	home := userHome()
	if home != "" {
		paths = append(paths,
			filepath.Join(home, ".config", "sing-box", "config.json"),
			filepath.Join(home, ".config", "sing-box", "sing-box.json"),
			filepath.Join(home, "Library", "Application Support", "sing-box", "config.json"),
		)
	}
	switch runtime.GOOS {
	case "linux":
		paths = append(paths, "/etc/sing-box/config.json")
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			paths = append(paths, filepath.Join(appdata, "sing-box", "config.json"))
		}
	}
	return paths
}

func trojanConfigPaths() []string {
	var paths []string
	home := userHome()
	if home != "" {
		paths = append(paths,
			filepath.Join(home, ".config", "trojan", "config.json"),
		)
	}
	switch runtime.GOOS {
	case "linux":
		paths = append(paths, "/etc/trojan/config.json")
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			paths = append(paths, filepath.Join(appdata, "trojan", "config.json"))
		}
	}
	return paths
}

func v2rayCoreConfigPaths() []string {
	var paths []string
	home := userHome()
	if home != "" {
		paths = append(paths,
			filepath.Join(home, ".config", "v2ray", "config.json"),
			filepath.Join(home, ".config", "xray", "config.json"),
		)
	}
	switch runtime.GOOS {
	case "linux":
		paths = append(paths,
			"/etc/v2ray/config.json",
			"/etc/xray/config.json",
			"/usr/local/etc/v2ray/config.json",
			"/usr/local/etc/xray/config.json",
		)
	}
	return paths
}

func userHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
