package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"erp/services/purchasing-service/internal/domain"
)

type ctxKey string

const AuthHeaderKey ctxKey = "authorization"

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"),
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) EnsureSupplier(ctx context.Context, id string) error {
	return c.get(ctx, "/suppliers/"+id)
}

func (c *Client) QuoteRequired(ctx context.Context) (bool, error) {
	req, err := c.request(ctx, "/settings/purchase_quote_required")
	if err != nil {
		return false, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("config: %s", strings.TrimSpace(string(b)))
	}
	var st struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return false, err
	}
	return st.Value == "true" || st.Value == "1", nil
}

func (c *Client) DefaultWarehouse(ctx context.Context) (string, error) {
	req, err := c.request(ctx, "/settings/default_warehouse_id")
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("config: %s", strings.TrimSpace(string(b)))
	}
	var st struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return "", err
	}
	return strings.TrimSpace(st.Value), nil
}

func (c *Client) get(ctx context.Context, path string) error {
	req, err := c.request(ctx, path)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return domain.ErrNotFound
	}
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("config: %s", strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *Client) request(ctx context.Context, path string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	if tok, ok := ctx.Value(AuthHeaderKey).(string); ok && tok != "" {
		req.Header.Set("Authorization", tok)
	}
	return req, nil
}
