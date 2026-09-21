package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrInvalid            = errors.New("invalid status")
	ErrQuoteRequired      = errors.New("quote required")
	ErrWarehouseRequired  = errors.New("warehouse required")
)

const (
	QuoteOpen      = "OPEN"
	QuoteConverted = "CONVERTED"
	QuoteCancelled = "CANCELLED"
	OrderApproved  = "APPROVED"
	OrderReceived  = "RECEIVED"
	OrderConferred = "CONFERRED"
	OrderCancelled = "CANCELLED"

	// Eixo financeiro do pedido (independente do status de entrega acima).
	PaymentPending = "PENDING"
	PaymentPaid    = "PAID"
)

// Status de entrega, como aparecem na tela: APPROVED = "pendente entrega", RECEIVED = recebido
// (entrada em estoque feita, falta conferir), CONFERRED = "finalizado", CANCELLED = "cancelado".
// Os valores técnicos são mantidos porque outros serviços (bi) já filtram por APPROVED.

type OrderItem struct {
	ID         string  `json:"id,omitempty"`
	ProductID  string  `json:"product_id"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	TotalPrice float64 `json:"total_price"`
}

type Quote struct {
	ID              string      `json:"id"`
	SupplierID      string      `json:"supplier_id"`
	Status          string      `json:"status"`
	SubtotalAmount  float64     `json:"subtotal_amount"`
	DiscountAmount  float64     `json:"discount_amount"`
	DeliveryAmount  float64     `json:"delivery_amount"`
	TotalAmount     float64     `json:"total_amount"`
	Notes           string      `json:"notes"`
	Items           []OrderItem `json:"items"`
	CreatedAt       time.Time   `json:"created_at"`
}

type PurchaseOrder struct {
	ID                   string      `json:"id"`
	SupplierID           string      `json:"supplier_id"`
	QuoteID              *string     `json:"quote_id"`
	PaymentMethodID      string      `json:"payment_method_id"`
	PaymentTermID        string      `json:"payment_term_id"`
	Status               string      `json:"status"`
	PaymentStatus        string      `json:"payment_status"`
	StockReceived        bool        `json:"stock_received"`
	TotalAmount          float64     `json:"total_amount"`
	ExpectedDeliveryDate *time.Time  `json:"expected_delivery_date"`
	Items                []OrderItem `json:"items"`
	CreatedAt            time.Time   `json:"created_at"`
}

type QuoteLine struct {
	QuoteID    string  `json:"quote_id"`
	SupplierID string  `json:"supplier_id"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	TotalPrice float64 `json:"total_price"`
}

type ProductCompare struct {
	ProductID     string      `json:"product_id"`
	Lines         []QuoteLine `json:"lines"`
	BestQuoteID   string      `json:"best_quote_id"`
	BestUnitPrice float64     `json:"best_unit_price"`
}

type QuoteComparison struct {
	Quotes        []Quote          `json:"quotes"`
	Products      []ProductCompare `json:"products"`
	LowestTotalID string           `json:"lowest_total_id"`
	LowestTotal   float64          `json:"lowest_total"`
}

type Directory interface {
	EnsureSupplier(ctx context.Context, id string) error
}

type Policy interface {
	QuoteRequired(ctx context.Context) (bool, error)
	DefaultWarehouse(ctx context.Context) (string, error)
}

type QuoteRepository interface {
	Create(ctx context.Context, q Quote) (Quote, error)
	Get(ctx context.Context, id string) (Quote, error)
	List(ctx context.Context) ([]Quote, error)
	Update(ctx context.Context, q Quote) (Quote, error)
	Delete(ctx context.Context, id string) error
	Convert(ctx context.Context, q Quote, methodID, termID string) (PurchaseOrder, error)
}

type OrderRepository interface {
	Create(ctx context.Context, o PurchaseOrder) (PurchaseOrder, error)
	Get(ctx context.Context, id string) (PurchaseOrder, error)
	List(ctx context.Context) ([]PurchaseOrder, error)
	// UpdateStatus sets the delivery status; moving to RECEIVED also flags stock_received.
	UpdateStatus(ctx context.Context, id, status string) error
	SetPaymentStatus(ctx context.Context, id, status string) error
	// Update replaces supplier, payment, expected date, total and items of an order that is
	// APPROVED (pending delivery) and PENDING payment; ErrInvalid otherwise.
	Update(ctx context.Context, o PurchaseOrder) (PurchaseOrder, error)
	// Delete removes an order under the same conditions as Update (items cascade).
	Delete(ctx context.Context, id string) error
}

type Catalog interface {
	RegisterPurchasePrice(ctx context.Context, productID string, price float64, referenceDocID string) error
	Receive(ctx context.Context, orderID, warehouseID string, items []OrderItem) error
}

type Cashflow interface {
	SchedulePurchase(ctx context.Context, orderID, supplierID, methodID, termID string, amount float64, at time.Time) error
	CancelPurchase(ctx context.Context, orderID string) error
}

func Totals(items []OrderItem) ([]OrderItem, float64) {
	var total float64
	out := make([]OrderItem, 0, len(items))
	for _, it := range items {
		if it.ProductID == "" || it.Quantity <= 0 {
			continue
		}
		it.TotalPrice = it.Quantity * it.UnitPrice
		total += it.TotalPrice
		out = append(out, it)
	}
	return out, total
}
