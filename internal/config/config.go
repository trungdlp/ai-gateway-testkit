package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Config contains the validated settings for a single test run.
type Config struct {
	BaseURL  string
	Model    string
	APIKey   string
	Timeout  time.Duration
	Protocol Protocol
}

type Protocol string

const (
	ProtocolOpenAI    Protocol = "openai"
	ProtocolAnthropic Protocol = "anthropic"
	ProtocolBoth      Protocol = "both"
)

func ParseProtocol(value string) (Protocol, error) {
	switch Protocol(strings.ToLower(strings.TrimSpace(value))) {
	case ProtocolOpenAI:
		return ProtocolOpenAI, nil
	case ProtocolAnthropic:
		return ProtocolAnthropic, nil
	case ProtocolBoth:
		return ProtocolBoth, nil
	default:
		return "", fmt.Errorf("protocol must be openai, anthropic, or both")
	}
}

// New validates and normalizes gateway settings. Plain HTTP is limited to
// loopback addresses unless explicitly allowed.
func New(baseURL, model, apiKey string, timeout time.Duration, protocol Protocol, allowInsecureHTTP bool) (Config, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return Config{}, fmt.Errorf("base URL is required")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return Config{}, fmt.Errorf("parse base URL: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return Config{}, fmt.Errorf("base URL scheme must be https or http")
	}
	if u.Host == "" {
		return Config{}, fmt.Errorf("base URL must include a host")
	}
	if u.User != nil {
		return Config{}, fmt.Errorf("base URL must not include user information")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return Config{}, fmt.Errorf("base URL must not include a query or fragment")
	}
	if u.Scheme == "http" && !allowInsecureHTTP && !isLoopback(u.Hostname()) {
		return Config{}, fmt.Errorf("plain HTTP is only allowed for loopback addresses; use --allow-insecure-http to override")
	}

	model = strings.TrimSpace(model)
	if model == "" {
		return Config{}, fmt.Errorf("model is required")
	}
	if strings.TrimSpace(apiKey) == "" {
		return Config{}, fmt.Errorf("API key is required")
	}
	if timeout <= 0 {
		return Config{}, fmt.Errorf("timeout must be greater than zero")
	}
	if protocol != ProtocolOpenAI && protocol != ProtocolAnthropic && protocol != ProtocolBoth {
		return Config{}, fmt.Errorf("protocol must be openai, anthropic, or both")
	}

	u.Path = strings.TrimRight(u.Path, "/")
	return Config{
		BaseURL:  strings.TrimRight(u.String(), "/"),
		Model:    model,
		APIKey:   apiKey,
		Timeout:  timeout,
		Protocol: protocol,
	}, nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
