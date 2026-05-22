package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	GatewayEndpoint string `toml:"endpoint" json:"gateway_endpoint" yaml:"endpoint"`
	AgentID         string `toml:"agent_id" json:"agent_id" yaml:"agent_id"`
	AgentName       string `toml:"name" json:"agent_name" yaml:"name"`
	Owner           string `toml:"owner" json:"owner" yaml:"owner"`
	KeyPath         string `toml:"key_path" json:"key_path" yaml:"key_path"`
	TimeoutSecs     int    `toml:"timeout_secs" json:"timeout_secs" yaml:"timeout_secs"`
	APIKey          string `toml:"api_key" json:"api_key" yaml:"api_key"`
	JWTToken        string `toml:"jwt_token" json:"jwt_token" yaml:"jwt_token"`
}

func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".eyevesa")
}

func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.toml")
}

func DefaultKeysDir() string {
	return filepath.Join(DefaultConfigDir(), "keys")
}

func DefaultAgentsDir() string {
	return filepath.Join(DefaultConfigDir(), "agents")
}

func EnsureDirs() error {
	dirs := []string{
		DefaultConfigDir(),
		DefaultKeysDir(),
		DefaultAgentsDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{
			GatewayEndpoint: "http://localhost:8080",
			TimeoutSecs:     30,
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := parseTOML(data, cfg); err != nil {
		return nil, err
	}

	if cfg.GatewayEndpoint == "" {
		cfg.GatewayEndpoint = "http://localhost:8080"
	}
	if cfg.TimeoutSecs == 0 {
		cfg.TimeoutSecs = 30
	}

	return cfg, nil
}

func (c *Config) Save(path string) error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	data, err := formatTOML(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func parseTOML(data []byte, cfg *Config) error {
	lines := splitLines(string(data))
	for _, line := range lines {
		line = trimSpace(line)
		if line == "" || line[0] == '#' || line[0] == '[' {
			continue
		}
		key, val, ok := splitKV(line)
		if !ok {
			continue
		}
		switch key {
		case "endpoint":
			cfg.GatewayEndpoint = unquote(val)
		case "agent_id":
			cfg.AgentID = unquote(val)
		case "name":
			cfg.AgentName = unquote(val)
		case "owner":
			cfg.Owner = unquote(val)
		case "key_path":
			cfg.KeyPath = unquote(val)
		case "timeout_secs":
			cfg.TimeoutSecs = atoi(val)
		case "api_key":
			cfg.APIKey = unquote(val)
		case "jwt_token":
			cfg.JWTToken = unquote(val)
		}
	}
	return nil
}

func formatTOML(cfg *Config) ([]byte, error) {
	var b []byte
	b = append(b, "[agent]\n"...)
	b = append(b, "name = "+quote(cfg.AgentName)+"\n"...)
	b = append(b, "agent_id = "+quote(cfg.AgentID)+"\n"...)
	b = append(b, "owner = "+quote(cfg.Owner)+"\n"...)
	b = append(b, "\n[gateway]\n"...)
	b = append(b, "endpoint = "+quote(cfg.GatewayEndpoint)+"\n"...)
	b = append(b, "timeout_secs = "+itoa(cfg.TimeoutSecs)+"\n"...)
	b = append(b, "\n[identity]\n"...)
	b = append(b, "key_path = "+quote(cfg.KeyPath)+"\n"...)
	if cfg.APIKey != "" || cfg.JWTToken != "" {
		b = append(b, "\n[auth]\n"...)
		if cfg.APIKey != "" {
			b = append(b, "api_key = "+quote(cfg.APIKey)+"\n"...)
		}
		if cfg.JWTToken != "" {
			b = append(b, "jwt_token = "+quote(cfg.JWTToken)+"\n"...)
		}
	}
	return b, nil
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range splitByNewline(s) {
		lines = append(lines, line)
	}
	return lines
}

func splitByNewline(s string) []string {
	var result []string
	current := ""
	for _, ch := range s {
		if ch == '\n' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func splitKV(line string) (string, string, bool) {
	for i := 0; i < len(line); i++ {
		if line[i] == '=' {
			return trimSpace(line[:i]), trimSpace(line[i+1:]), true
		}
	}
	return "", "", false
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func quote(s string) string {
	return `"` + s + `"`
}

func atoi(s string) int {
	n := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
