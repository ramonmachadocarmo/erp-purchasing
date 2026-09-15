package application

import (
	"context"
	"testing"
	"time"

	"erp/services/purchasing-service/internal/domain"
)

type memOrders struct{ byID map[string]domain.PurchaseOrder }

func (m *memOrders) Create(_ context.Context, o domain.PurchaseOrder) (domain.PurchaseOrder, error) {
	o.ID = "po1"
	o.CreatedAt = time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	if m.byID == nil {
		m.byID = map[string]domain.PurchaseOrder{}
	}
	m.byID[o.ID] = o
	return o, nil
}

func (m *memOrders) Get(_ context.Context, id string) (domain.PurchaseOrder, error) {
	o, ok := m.byID[id]
	if !ok {
		return domain.PurchaseOrder{}, domain.ErrNotFound
	}
	return o, nil
}

func (m *memOrders) List(context.Context) ([]domain.PurchaseOrder, error) { return nil, nil }

func (m *memOrders) UpdateStatus(_ context.Context, id, status string) error {
	o, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	o.Status = status
	m.byID[id] = o
	return nil
}

type memQuotes struct{ byID map[string]domain.Quote }

func (m *memQuotes) Create(_ context.Context, q domain.Quote) (domain.Quote, error) {
	q.ID = "q1"
	if m.byID == nil {
		m.byID = map[string]domain.Quote{}
	}
	m.byID[q.ID] = q
	return q, nil
}

func (m *memQuotes) Get(_ context.Context, id string) (domain.Quote, error) {
	q, ok := m.byID[id]
	if !ok {
		return domain.Quote{}, domain.ErrNotFound
	}
	return q, nil
}

func (m *memQuotes) List(context.Context) ([]domain.Quote, error) { return nil, nil }

func (m *memQuotes) Update(_ context.Context, q domain.Quote) (domain.Quote, error) {
	cur, ok := m.byID[q.ID]
	if !ok {
		return domain.Quote{}, domain.ErrNotFound
	}
	if cur.Status != domain.QuoteOpen {
		return domain.Quote{}, domain.ErrInvalid
	}
	m.byID[q.ID] = q
	return q, nil
}

func (m *memQuotes) Delete(_ context.Context, id string) error {
	cur, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if cur.Status != domain.QuoteOpen {
		return domain.ErrInvalid
	}
	delete(m.byID, id)
	return nil
}

func (m *memQuotes) Convert(_ context.Context, q domain.Quote, methodID, termID string) (domain.PurchaseOrder, error) {
	return domain.PurchaseOrder{
		ID: "po1", SupplierID: q.SupplierID, PaymentMethodID: methodID, PaymentTermID: termID,
		TotalAmount: q.TotalAmount, Status: domain.OrderApproved, Items: q.Items,
		CreatedAt: time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC),
	}, nil
}

type okDir struct{}

func (okDir) EnsureSupplier(context.Context, string) error { return nil }

type policy struct {
	required  bool
	warehouse string
}

func (p policy) QuoteRequired(context.Context) (bool, error) { return p.required, nil }

func (p policy) DefaultWarehouse(context.Context) (string, error) { return p.warehouse, nil }

type nopCatalog struct {
	receivedWH string
}

func (nopCatalog) RegisterPurchasePrice(context.Context, string, float64, string) error { return nil }

func (c *nopCatalog) Receive(_ context.Context, _, warehouseID string, _ []domain.OrderItem) error {
	c.receivedWH = warehouseID
	return nil
}

type cashSpy struct {
	n      int
	amount float64
}

func (c *cashSpy) SchedulePurchase(_ context.Context, _, _, _, _ string, amount float64, _ time.Time) error {
	c.n++
	c.amount = amount
	return nil
}

func TestCreateOrderRequiresPayment(t *testing.T) {
	svc := New(&memOrders{}, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	_, err := svc.CreateOrder(context.Background(), domain.PurchaseOrder{
		SupplierID: "s1",
		Items:      []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
	})
	if err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestCreateOrderSchedulesCashflow(t *testing.T) {
	cash := &cashSpy{}
	svc := New(&memOrders{}, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, cash)
	got, err := svc.CreateOrder(context.Background(), domain.PurchaseOrder{
		SupplierID: "s1", PaymentMethodID: "m", PaymentTermID: "t",
		Items: []domain.OrderItem{{ProductID: "p", Quantity: 2, UnitPrice: 500}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalAmount != 1000 || cash.n != 1 || cash.amount != 1000 {
		t.Fatalf("%+v %+v", got, cash)
	}
}

func TestConvertQuoteRequiresPayment(t *testing.T) {
	q := &memQuotes{byID: map[string]domain.Quote{"q1": {ID: "q1", Status: domain.QuoteOpen, SupplierID: "s1", TotalAmount: 10}}}
	svc := New(&memOrders{}, q, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	_, err := svc.ConvertQuote(context.Background(), "q1", "", "")
	if err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestConvertQuoteSchedulesCashflow(t *testing.T) {
	cash := &cashSpy{}
	q := &memQuotes{byID: map[string]domain.Quote{
		"q1": {ID: "q1", Status: domain.QuoteOpen, SupplierID: "s1", TotalAmount: 1000, Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 1000}}},
	}}
	svc := New(&memOrders{}, q, okDir{}, &nopCatalog{}, policy{}, cash)
	got, err := svc.ConvertQuote(context.Background(), "q1", "m", "t")
	if err != nil || got.PaymentMethodID != "m" || cash.n != 1 || cash.amount != 1000 {
		t.Fatalf("%v %+v %+v", err, got, cash)
	}
}

func TestUpdateQuoteOpen(t *testing.T) {
	q := &memQuotes{byID: map[string]domain.Quote{
		"q1": {ID: "q1", Status: domain.QuoteOpen, SupplierID: "s1", Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}}},
	}}
	svc := New(&memOrders{}, q, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	got, err := svc.UpdateQuote(context.Background(), "q1", domain.Quote{
		SupplierID: "s1", Notes: "n",
		Items: []domain.OrderItem{{ProductID: "p", Quantity: 2, UnitPrice: 15}},
	})
	if err != nil || got.TotalAmount != 30 || got.Notes != "n" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestUpdateQuoteConvertedRejected(t *testing.T) {
	q := &memQuotes{byID: map[string]domain.Quote{"q1": {ID: "q1", Status: domain.QuoteConverted, SupplierID: "s1"}}}
	svc := New(&memOrders{}, q, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	_, err := svc.UpdateQuote(context.Background(), "q1", domain.Quote{
		SupplierID: "s1", Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
	})
	if err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestDeleteQuoteOpen(t *testing.T) {
	q := &memQuotes{byID: map[string]domain.Quote{"q1": {ID: "q1", Status: domain.QuoteOpen}}}
	svc := New(&memOrders{}, q, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	if err := svc.DeleteQuote(context.Background(), "q1"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Get(context.Background(), "q1"); err != domain.ErrNotFound {
		t.Fatalf("%v", err)
	}
}

func TestDeleteQuoteConvertedRejected(t *testing.T) {
	q := &memQuotes{byID: map[string]domain.Quote{"q1": {ID: "q1", Status: domain.QuoteConverted}}}
	svc := New(&memOrders{}, q, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	if err := svc.DeleteQuote(context.Background(), "q1"); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestCreateOrderQuoteRequired(t *testing.T) {
	svc := New(&memOrders{}, &memQuotes{}, okDir{}, &nopCatalog{}, policy{required: true}, &cashSpy{})
	_, err := svc.CreateOrder(context.Background(), domain.PurchaseOrder{
		SupplierID: "s1", PaymentMethodID: "m", PaymentTermID: "t",
		Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
	})
	if err != domain.ErrQuoteRequired {
		t.Fatalf("%v", err)
	}
}

func TestReceiveRequiresWarehouse(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{
		"po1": {ID: "po1", Status: domain.OrderApproved, Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}}},
	}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	if err := svc.Receive(context.Background(), "po1", ""); err != domain.ErrWarehouseRequired {
		t.Fatalf("%v", err)
	}
}

func TestReceiveUsesDefaultWarehouse(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{
		"po1": {ID: "po1", Status: domain.OrderApproved, Items: []domain.OrderItem{{ProductID: "p", Quantity: 2, UnitPrice: 10}}},
	}}
	cat := &nopCatalog{}
	svc := New(orders, &memQuotes{}, okDir{}, cat, policy{warehouse: "w1"}, &cashSpy{})
	if err := svc.Receive(context.Background(), "po1", ""); err != nil {
		t.Fatal(err)
	}
	if cat.receivedWH != "w1" || orders.byID["po1"].Status != domain.OrderReceived {
		t.Fatalf("%s %s", cat.receivedWH, orders.byID["po1"].Status)
	}
}

func TestConferRequiresReceived(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{
		"po1": {ID: "po1", Status: domain.OrderApproved},
	}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	if err := svc.Confer(context.Background(), "po1"); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestConfer(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{
		"po1": {ID: "po1", Status: domain.OrderReceived},
	}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	if err := svc.Confer(context.Background(), "po1"); err != nil {
		t.Fatal(err)
	}
	if orders.byID["po1"].Status != domain.OrderConferred {
		t.Fatalf("%s", orders.byID["po1"].Status)
	}
}
