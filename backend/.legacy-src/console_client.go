package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ConsoleClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

type consoleConsumer struct {
	Name        string              `json:"name"`
	Department  string              `json:"department,omitempty"`
	Credentials []consoleCredential `json:"credentials"`
}

type consoleCredential struct {
	Type   string   `json:"type"`
	Source string   `json:"source"`
	Values []string `json:"values"`
}

type usageStatsResponse struct {
	Items []ConsumerUsageStat `json:"items"`
}

func NewConsoleClient(cfg Config) *ConsoleClient {
	base := strings.TrimRight(cfg.ConsoleBaseURL, "/")
	return &ConsoleClient{
		baseURL:  base,
		username: cfg.ConsoleUsername,
		password: cfg.ConsolePassword,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *ConsoleClient) Enabled() bool {
	return c != nil && c.baseURL != ""
}

func (c *ConsoleClient) UpsertConsumer(ctx context.Context, consumerName string, department string, keys []string) error {
	if !c.Enabled() {
		return nil
	}
	if len(keys) == 0 {
		return fmt.Errorf("consumer keys cannot be empty")
	}

	payload := consoleConsumer{
		Name:       consumerName,
		Department: department,
		Credentials: []consoleCredential{{
			Type:   "key-auth",
			Source: "BEARER",
			Values: keys,
		}},
	}

	_, err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/v1/consumers/%s", url.PathEscape(consumerName)), payload)
	return err
}

func (c *ConsoleClient) FetchUsageStats(ctx context.Context, from time.Time, to time.Time) ([]ConsumerUsageStat, error) {
	if !c.Enabled() {
		return nil, nil
	}
	queryPath := fmt.Sprintf("/v1/portal/stats/usage?from=%d&to=%d", from.UnixMilli(), to.UnixMilli())
	body, err := c.doJSON(ctx, http.MethodGet, queryPath, nil)
	if err != nil {
		return nil, err
	}

	var direct usageStatsResponse
	if err := decodeFlexible(body, &direct); err == nil && len(direct.Items) > 0 {
		return direct.Items, nil
	}

	var list []ConsumerUsageStat
	if err := decodeFlexible(body, &list); err == nil {
		return list, nil
	}

	var wrapped struct {
		Data []ConsumerUsageStat `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil {
		return wrapped.Data, nil
	}

	return nil, fmt.Errorf("failed to decode usage stats response")
}

func (c *ConsoleClient) doJSON(ctx context.Context, method string, path string, reqBody any) ([]byte, error) {
	if !c.Enabled() {
		return nil, nil
	}

	fullURL := c.baseURL + path
	var reader io.Reader
	if reqBody != nil {
		raw, err := json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.username != "" {
		token := base64.StdEncoding.EncodeToString([]byte(c.username + ":" + c.password))
		req.Header.Set("Authorization", "Basic "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("console request failed: %s, body=%s", resp.Status, string(body))
	}
	return body, nil
}

func decodeFlexible[T any](body []byte, out *T) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err == nil {
			return nil
		}
	}
	return json.Unmarshal(body, out)
}
