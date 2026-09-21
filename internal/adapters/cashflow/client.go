package cashflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	// A zero-value order has no cash to schedule, and cashflow rejects amount <= 0.
	// (On edit, the service calls CancelPurchase instead so a stale schedule doesn't linger.)
	if amount <= 0 {
		return nil
	}
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
	return c.do(ctx, req)
}

// CancelPurchase clears the order's whole cash schedule (order edited to zero, or deleted).
func (c *Client) CancelPurchase(ctx context.Context, orderID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+"/schedule/PURCHASE/"+url.PathEscape(orderID), nil)
	if err != nil {
		return err
	}
	return c.do(ctx, req)
}

func (c *Client) do(ctx context.Context, req *http.Request) error {
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
