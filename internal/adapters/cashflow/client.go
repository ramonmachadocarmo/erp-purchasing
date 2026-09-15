package cashflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ctxKey string

const AuthHeaderKey ctxKey = "authorization"

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) SchedulePurchase(ctx context.Context, orderID, supplierID, methodID, termID string, amount float64, at time.Time) error {
	raw, err := json.Marshal(map[string]any{
		"direction":         "OUT",
		"amount":            amount,
		"payment_method_id": methodID,
		"payment_term_id":   termID,
		"reference_type":    "PURCHASE",
		"reference_id":      orderID,
		"party_id":          supplierID,
		"occurred_at":       at.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/schedule", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if tok, ok := ctx.Value(AuthHeaderKey).(string); ok && tok != "" {
		req.Header.Set("Authorization", tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cashflow: %s", strings.TrimSpace(string(b)))
	}
	return nil
}
