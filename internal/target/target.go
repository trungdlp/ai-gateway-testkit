package target

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/trungdlp/ai-gateway-testkit/internal/config"
	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

type Endpoint struct {
	BaseURL       string `json:"base_url" yaml:"base_url"`
	Model         string `json:"model" yaml:"model"`
	CredentialEnv string `json:"credential_env" yaml:"credential_env"`
	APIVersion    string `json:"api_version,omitempty" yaml:"api_version,omitempty"`
}

type Manifest struct {
	Name      string    `json:"name" yaml:"name"`
	OpenAI    *Endpoint `json:"openai,omitempty" yaml:"openai,omitempty"`
	Anthropic *Endpoint `json:"anthropic,omitempty" yaml:"anthropic,omitempty"`
}

type Resolved struct {
	Name      string
	Endpoints map[string]testcase.Target
}

type Environment func(string) (string, bool)

func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read target manifest: %w", err)
	}
	var manifest Manifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse target manifest: %w", err)
	}
	return manifest, nil
}

func Legacy(name string, protocol config.Protocol, baseURL, model, credentialEnv string) Manifest {
	endpoint := func() *Endpoint {
		return &Endpoint{BaseURL: baseURL, Model: model, CredentialEnv: credentialEnv}
	}
	manifest := Manifest{Name: name}
	if protocol == config.ProtocolOpenAI || protocol == config.ProtocolBoth {
		manifest.OpenAI = endpoint()
	}
	if protocol == config.ProtocolAnthropic || protocol == config.ProtocolBoth {
		manifest.Anthropic = endpoint()
		manifest.Anthropic.APIVersion = "2023-06-01"
	}
	return manifest
}

func Resolve(manifest Manifest, getenv Environment, timeout time.Duration, allowInsecureHTTP bool, retry gateway.RetryPolicy) (Resolved, error) {
	if retry.MaxRetries < 0 || retry.MaxRetries > 10 {
		return Resolved{}, fmt.Errorf("retries must be between 0 and 10")
	}
	if retry.InitialBackoff <= 0 || retry.MaxBackoff <= 0 || retry.InitialBackoff > retry.MaxBackoff {
		return Resolved{}, fmt.Errorf("retry backoff must be positive and not exceed retry max wait")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		manifest.Name = "gateway"
	}
	resolved := Resolved{Name: manifest.Name, Endpoints: map[string]testcase.Target{}}
	entries := []struct {
		protocol config.Protocol
		endpoint *Endpoint
	}{
		{config.ProtocolOpenAI, manifest.OpenAI},
		{config.ProtocolAnthropic, manifest.Anthropic},
	}
	for _, entry := range entries {
		if entry.endpoint == nil {
			continue
		}
		if strings.TrimSpace(entry.endpoint.CredentialEnv) == "" {
			return Resolved{}, fmt.Errorf("%s credential_env is required", entry.protocol)
		}
		credential, ok := getenv(entry.endpoint.CredentialEnv)
		if !ok || strings.TrimSpace(credential) == "" {
			return Resolved{}, fmt.Errorf("environment variable %s is not set or is empty", entry.endpoint.CredentialEnv)
		}
		validated, err := config.New(entry.endpoint.BaseURL, entry.endpoint.Model, credential, timeout, entry.protocol, allowInsecureHTTP)
		if err != nil {
			return Resolved{}, fmt.Errorf("%s target: %w", entry.protocol, err)
		}
		apiVersion := entry.endpoint.APIVersion
		if entry.protocol == config.ProtocolAnthropic && apiVersion == "" {
			apiVersion = "2023-06-01"
		}
		resolved.Endpoints[string(entry.protocol)] = testcase.Target{
			Protocol: string(entry.protocol), BaseURL: validated.BaseURL, Model: validated.Model,
			Credential: credential, Timeout: timeout, APIVersion: apiVersion, Retry: retry,
		}
	}
	if len(resolved.Endpoints) == 0 {
		return Resolved{}, fmt.Errorf("target manifest must configure openai, anthropic, or both")
	}
	return resolved, nil
}

func (r Resolved) Fingerprint() string {
	protocols := make([]string, 0, len(r.Endpoints))
	for protocol := range r.Endpoints {
		protocols = append(protocols, protocol)
	}
	sort.Strings(protocols)
	public := make([]map[string]string, 0, len(protocols))
	for _, protocol := range protocols {
		endpoint := r.Endpoints[protocol]
		public = append(public, map[string]string{
			"protocol": protocol, "base_url": endpoint.BaseURL, "model": endpoint.Model, "api_version": endpoint.APIVersion,
		})
	}
	data, _ := json.Marshal(public)
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
