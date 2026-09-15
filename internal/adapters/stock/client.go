package stock

import (
	"bytes"
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

func (c *Client) RegisterPurchasePrice(ctx context.Context, productID string, price float64, referenceDocID string) error {
	body, err := json.Marshal(map[string]any{
		"product_id":       productID,
		"new_price":        price,
		"reference_doc_id": referenceDocID,
	})
	if err != nil {
		return err
	}
	return c.post(ctx, "/prices/purchase", body)
}

func (c *Client) Receive(ctx context.Context, orderID, warehouseID string, items []domain.OrderItem) error {
	body, err := json.Marshal(map[string]any{
		"order_id":     orderID,
		"warehouse_id": warehouseID,
		"items":        items,
	})
	if err != nil {
		return err
	}
	return c.post(ctx, "/purchases/receive", body)
}

func (c *Client) post(ctx context.Context, path string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(body))
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
		return fmt.Errorf("stock: %s", strings.TrimSpace(string(b)))
	}
	return nil
}
