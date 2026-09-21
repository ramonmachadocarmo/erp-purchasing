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
	if status == domain.OrderReceived {
		o.StockReceived = true
	}
	m.byID[id] = o
	return nil
}

func (m *memOrders) SetPaymentStatus(_ context.Context, id, status string) error {
	o, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	o.PaymentStatus = status
	m.byID[id] = o
	return nil
}

func (m *memOrders) Update(_ context.Context, o domain.PurchaseOrder) (domain.PurchaseOrder, error) {
	cur, ok := m.byID[o.ID]
	if !ok {
		return domain.PurchaseOrder{}, domain.ErrNotFound
	}
	if cur.Status != domain.OrderApproved || cur.PaymentStatus == domain.PaymentPaid {
		return domain.PurchaseOrder{}, domain.ErrInvalid
	}
	m.byID[o.ID] = o
	return o, nil
}

func (m *memOrders) Delete(_ context.Context, id string) error {
	cur, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if cur.Status != domain.OrderApproved || cur.PaymentStatus == domain.PaymentPaid {
		return domain.ErrInvalid
	}
	delete(m.byID, id)
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
	n         int
	amount    float64
	cancelled int
}

func (c *cashSpy) CancelPurchase(context.Context, string) error {
	c.cancelled++
	return nil
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

func TestUpdateOrderReschedulesCashflow(t *testing.T) {
	cash := &cashSpy{}
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{
		"po1": {ID: "po1", Status: domain.OrderApproved, SupplierID: "s1"},
	}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, cash)
	got, err := svc.UpdateOrder(context.Background(), "po1", domain.PurchaseOrder{
		SupplierID: "s1", PaymentMethodID: "m", PaymentTermID: "t",
		Items: []domain.OrderItem{{ProductID: "p", Quantity: 3, UnitPrice: 10}},
	})
	if err != nil || got.TotalAmount != 30 || cash.n != 1 || cash.amount != 30 {
		t.Fatalf("%v %+v %+v", err, got, cash)
	}
}

func TestUpdateOrderReceivedRejected(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{"po1": {ID: "po1", Status: domain.OrderReceived}}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	_, err := svc.UpdateOrder(context.Background(), "po1", domain.PurchaseOrder{
		SupplierID: "s1", PaymentMethodID: "m", PaymentTermID: "t",
		Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
	})
	if err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestDeleteOrderApprovedCancelsCashflow(t *testing.T) {
	cash := &cashSpy{}
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{"po1": {ID: "po1", Status: domain.OrderApproved}}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, cash)
	if err := svc.DeleteOrder(context.Background(), "po1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := orders.byID["po1"]; ok || cash.cancelled != 1 {
		t.Fatalf("%+v %+v", orders.byID, cash)
	}
}

func TestDeleteOrderReceivedRejected(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{"po1": {ID: "po1", Status: domain.OrderReceived}}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	if err := svc.DeleteOrder(context.Background(), "po1"); err != domain.ErrInvalid {
		t.Fatalf("%v", err)
	}
}

func TestCreateOrderStartsPaymentPending(t *testing.T) {
	svc := New(&memOrders{}, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	got, err := svc.CreateOrder(context.Background(), domain.PurchaseOrder{
		SupplierID: "s1", PaymentMethodID: "m", PaymentTermID: "t",
		Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
	})
	if err != nil || got.PaymentStatus != domain.PaymentPending || got.Status != domain.OrderApproved {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestPaidOrderCannotBeEditedOrDeleted(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{
		"po1": {ID: "po1", Status: domain.OrderApproved, PaymentStatus: domain.PaymentPaid, SupplierID: "s1"},
	}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	_, err := svc.UpdateOrder(context.Background(), "po1", domain.PurchaseOrder{
		SupplierID: "s1", PaymentMethodID: "m", PaymentTermID: "t",
		Items: []domain.OrderItem{{ProductID: "p", Quantity: 1, UnitPrice: 10}},
	})
	if err != domain.ErrInvalid {
		t.Fatalf("update: %v", err)
	}
	if err := svc.DeleteOrder(context.Background(), "po1"); err != domain.ErrInvalid {
		t.Fatalf("delete: %v", err)
	}
	// reopening the payment makes it editable again
	if err := svc.SetPaymentStatus(context.Background(), "po1", domain.PaymentPending); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteOrder(context.Background(), "po1"); err != nil {
		t.Fatalf("delete after reopen: %v", err)
	}
}

func TestSetPaymentStatus(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{
		"po1": {ID: "po1", Status: domain.OrderApproved, PaymentStatus: domain.PaymentPending},
		"po2": {ID: "po2", Status: domain.OrderCancelled, PaymentStatus: domain.PaymentPending},
	}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	if err := svc.SetPaymentStatus(context.Background(), "po1", domain.PaymentPaid); err != nil || orders.byID["po1"].PaymentStatus != domain.PaymentPaid {
		t.Fatalf("%v %+v", err, orders.byID["po1"])
	}
	if err := svc.SetPaymentStatus(context.Background(), "po1", "BOGUS"); err != domain.ErrInvalid {
		t.Fatalf("bogus: %v", err)
	}
	if err := svc.SetPaymentStatus(context.Background(), "po2", domain.PaymentPaid); err != domain.ErrInvalid {
		t.Fatalf("cancelled: %v", err)
	}
}

func TestSetDeliveryStatusManual(t *testing.T) {
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{
		"open":     {ID: "open", Status: domain.OrderApproved},
		"received": {ID: "received", Status: domain.OrderReceived, StockReceived: true},
		"done":     {ID: "done", Status: domain.OrderConferred, StockReceived: true},
		"manual":   {ID: "manual", Status: domain.OrderConferred, StockReceived: false},
		"cancel":   {ID: "cancel", Status: domain.OrderCancelled},
	}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, &cashSpy{})
	ctx := context.Background()
	if err := svc.SetDeliveryStatus(ctx, "open", domain.OrderConferred); err != nil || orders.byID["open"].Status != domain.OrderConferred {
		t.Fatalf("finalize by hand: %v", err)
	}
	if err := svc.SetDeliveryStatus(ctx, "received", domain.OrderConferred); err != nil || orders.byID["received"].Status != domain.OrderConferred {
		t.Fatalf("finalize received: %v", err)
	}
	if err := svc.SetDeliveryStatus(ctx, "done", domain.OrderApproved); err != domain.ErrInvalid {
		t.Fatalf("reopen after stock entry must be refused: %v", err)
	}
	if err := svc.SetDeliveryStatus(ctx, "manual", domain.OrderApproved); err != nil || orders.byID["manual"].Status != domain.OrderApproved {
		t.Fatalf("reopen manual finalize: %v", err)
	}
	if err := svc.SetDeliveryStatus(ctx, "cancel", domain.OrderConferred); err != domain.ErrInvalid {
		t.Fatalf("cancelled: %v", err)
	}
	if err := svc.SetDeliveryStatus(ctx, "open", domain.OrderReceived); err != domain.ErrInvalid {
		t.Fatalf("RECEIVED only via Receber: %v", err)
	}
}

func TestCancelClearsCashflow(t *testing.T) {
	cash := &cashSpy{}
	orders := &memOrders{byID: map[string]domain.PurchaseOrder{"po1": {ID: "po1", Status: domain.OrderApproved}}}
	svc := New(orders, &memQuotes{}, okDir{}, &nopCatalog{}, policy{}, cash)
	if err := svc.Cancel(context.Background(), "po1"); err != nil {
		t.Fatal(err)
	}
	if orders.byID["po1"].Status != domain.OrderCancelled || cash.cancelled != 1 {
		t.Fatalf("%+v %+v", orders.byID["po1"], cash)
	}
	// retry after a cashflow failure: already cancelled, still cleans up
	if err := svc.Cancel(context.Background(), "po1"); err != nil || cash.cancelled != 2 {
		t.Fatalf("%v %+v", err, cash)
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
