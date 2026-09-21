package application

import (
	"context"

	"erp/services/purchasing-service/internal/domain"
)

type Service struct {
	orders   domain.OrderRepository
	quotes   domain.QuoteRepository
	dir      domain.Directory
	catalog  domain.Catalog
	policy   domain.Policy
	cashflow domain.Cashflow
}

func New(orders domain.OrderRepository, quotes domain.QuoteRepository, dir domain.Directory, catalog domain.Catalog, policy domain.Policy, cashflow domain.Cashflow) *Service {
	return &Service{orders: orders, quotes: quotes, dir: dir, catalog: catalog, policy: policy, cashflow: cashflow}
}

func (s *Service) CreateQuote(ctx context.Context, q domain.Quote) (domain.Quote, error) {
	if err := s.dir.EnsureSupplier(ctx, q.SupplierID); err != nil {
		return domain.Quote{}, err
	}
	items, total := domain.Totals(q.Items)
	if len(items) == 0 {
		return domain.Quote{}, domain.ErrInvalid
	}
	q.Items = items
	q.SubtotalAmount = total
	q.TotalAmount = total - q.DiscountAmount + q.DeliveryAmount
	q.Status = domain.QuoteOpen
	return s.quotes.Create(ctx, q)
}

func (s *Service) GetQuote(ctx context.Context, id string) (domain.Quote, error) {
	return s.quotes.Get(ctx, id)
}

func (s *Service) ListQuotes(ctx context.Context) ([]domain.Quote, error) {
	return s.quotes.List(ctx)
}

func (s *Service) UpdateQuote(ctx context.Context, id string, q domain.Quote) (domain.Quote, error) {
	cur, err := s.quotes.Get(ctx, id)
	if err != nil {
		return domain.Quote{}, err
	}
	if cur.Status != domain.QuoteOpen {
		return domain.Quote{}, domain.ErrInvalid
	}
	if err := s.dir.EnsureSupplier(ctx, q.SupplierID); err != nil {
		return domain.Quote{}, err
	}
	items, total := domain.Totals(q.Items)
	if len(items) == 0 {
		return domain.Quote{}, domain.ErrInvalid
	}
	q.ID = id
	q.Items = items
	q.SubtotalAmount = total
	q.TotalAmount = total - q.DiscountAmount + q.DeliveryAmount
	q.Status = domain.QuoteOpen
	q.CreatedAt = cur.CreatedAt
	return s.quotes.Update(ctx, q)
}

func (s *Service) DeleteQuote(ctx context.Context, id string) error {
	cur, err := s.quotes.Get(ctx, id)
	if err != nil {
		return err
	}
	if cur.Status != domain.QuoteOpen {
		return domain.ErrInvalid
	}
	return s.quotes.Delete(ctx, id)
}

func (s *Service) ConvertQuote(ctx context.Context, id, methodID, termID string) (domain.PurchaseOrder, error) {
	if methodID == "" || termID == "" {
		return domain.PurchaseOrder{}, domain.ErrInvalid
	}
	q, err := s.quotes.Get(ctx, id)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	if q.Status != domain.QuoteOpen {
		return domain.PurchaseOrder{}, domain.ErrInvalid
	}
	po, err := s.quotes.Convert(ctx, q, methodID, termID)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	if err := s.cashflow.SchedulePurchase(ctx, po.ID, po.SupplierID, po.PaymentMethodID, po.PaymentTermID, po.TotalAmount, po.CreatedAt); err != nil {
		return domain.PurchaseOrder{}, err
	}
	return po, nil
}

func (s *Service) CompareQuotes(ctx context.Context, ids []string) (domain.QuoteComparison, error) {
	if len(ids) < 1 {
		return domain.QuoteComparison{}, domain.ErrInvalid
	}
	quotes := make([]domain.Quote, 0, len(ids))
	for _, id := range ids {
		q, err := s.quotes.Get(ctx, id)
		if err != nil {
			return domain.QuoteComparison{}, err
		}
		quotes = append(quotes, q)
	}
	return domain.CompareQuotes(quotes), nil
}

func (s *Service) CreateOrder(ctx context.Context, o domain.PurchaseOrder) (domain.PurchaseOrder, error) {
	required, err := s.policy.QuoteRequired(ctx)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	if required && (o.QuoteID == nil || *o.QuoteID == "") {
		return domain.PurchaseOrder{}, domain.ErrQuoteRequired
	}
	if o.QuoteID != nil && *o.QuoteID != "" {
		return s.ConvertQuote(ctx, *o.QuoteID, o.PaymentMethodID, o.PaymentTermID)
	}
	if o.PaymentMethodID == "" || o.PaymentTermID == "" {
		return domain.PurchaseOrder{}, domain.ErrInvalid
	}
	if err := s.dir.EnsureSupplier(ctx, o.SupplierID); err != nil {
		return domain.PurchaseOrder{}, err
	}
	items, total := domain.Totals(o.Items)
	if len(items) == 0 {
		return domain.PurchaseOrder{}, domain.ErrInvalid
	}
	o.Items = items
	o.TotalAmount = total
	o.Status = domain.OrderApproved
	o.PaymentStatus = domain.PaymentPending
	created, err := s.orders.Create(ctx, o)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	if err := s.cashflow.SchedulePurchase(ctx, created.ID, created.SupplierID, created.PaymentMethodID, created.PaymentTermID, created.TotalAmount, created.CreatedAt); err != nil {
		return domain.PurchaseOrder{}, err
	}
	return created, nil
}

func (s *Service) GetOrder(ctx context.Context, id string) (domain.PurchaseOrder, error) {
	return s.orders.Get(ctx, id)
}

func (s *Service) ListOrders(ctx context.Context) ([]domain.PurchaseOrder, error) {
	return s.orders.List(ctx)
}

// editable: an order can only be edited/deleted while it is pending delivery and unpaid —
// a paid order already moved money, so it has to be reopened (payment back to PENDING) first.
func editable(o domain.PurchaseOrder) bool {
	return o.Status == domain.OrderApproved && o.PaymentStatus != domain.PaymentPaid
}

// UpdateOrder edits a pending-delivery, unpaid order and re-plans its cash schedule (cashflow replaces the
// whole schedule of the reference, so it stays derived from the order's current state).
func (s *Service) UpdateOrder(ctx context.Context, id string, o domain.PurchaseOrder) (domain.PurchaseOrder, error) {
	cur, err := s.orders.Get(ctx, id)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	if !editable(cur) {
		return domain.PurchaseOrder{}, domain.ErrInvalid
	}
	if o.PaymentMethodID == "" || o.PaymentTermID == "" {
		return domain.PurchaseOrder{}, domain.ErrInvalid
	}
	if err := s.dir.EnsureSupplier(ctx, o.SupplierID); err != nil {
		return domain.PurchaseOrder{}, err
	}
	items, total := domain.Totals(o.Items)
	if len(items) == 0 {
		return domain.PurchaseOrder{}, domain.ErrInvalid
	}
	o.ID = id
	o.Items = items
	o.TotalAmount = total
	o.Status = domain.OrderApproved
	o.PaymentStatus = domain.PaymentPending
	o.QuoteID = cur.QuoteID
	o.CreatedAt = cur.CreatedAt
	updated, err := s.orders.Update(ctx, o)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	if updated.TotalAmount <= 0 {
		err = s.cashflow.CancelPurchase(ctx, updated.ID)
	} else {
		err = s.cashflow.SchedulePurchase(ctx, updated.ID, updated.SupplierID, updated.PaymentMethodID, updated.PaymentTermID, updated.TotalAmount, updated.CreatedAt)
	}
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	return updated, nil
}

// DeleteOrder removes a pending-delivery, unpaid order and clears its cash schedule.
func (s *Service) DeleteOrder(ctx context.Context, id string) error {
	cur, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if !editable(cur) {
		return domain.ErrInvalid
	}
	if err := s.orders.Delete(ctx, id); err != nil {
		return err
	}
	return s.cashflow.CancelPurchase(ctx, id)
}

func (s *Service) Receive(ctx context.Context, id, warehouseID string) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status != domain.OrderApproved {
		return domain.ErrInvalid
	}
	if warehouseID == "" {
		warehouseID, err = s.policy.DefaultWarehouse(ctx)
		if err != nil {
			return err
		}
	}
	if warehouseID == "" {
		return domain.ErrWarehouseRequired
	}
	for _, item := range o.Items {
		if err := s.catalog.RegisterPurchasePrice(ctx, item.ProductID, item.UnitPrice, o.ID); err != nil {
			return err
		}
	}
	if err := s.catalog.Receive(ctx, o.ID, warehouseID, o.Items); err != nil {
		return err
	}
	return s.orders.UpdateStatus(ctx, id, domain.OrderReceived)
}

func (s *Service) Confer(ctx context.Context, id string) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status != domain.OrderReceived {
		return domain.ErrInvalid
	}
	return s.orders.UpdateStatus(ctx, id, domain.OrderConferred)
}

// Cancel cancels an order that has not been received yet and clears its cash schedule.
// Idempotent: cancelling an already-cancelled order just retries the cashflow clean-up.
func (s *Service) Cancel(ctx context.Context, id string) error {
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status == domain.OrderReceived || o.Status == domain.OrderConferred {
		return domain.ErrInvalid
	}
	if o.Status != domain.OrderCancelled {
		if err := s.orders.UpdateStatus(ctx, id, domain.OrderCancelled); err != nil {
			return err
		}
	}
	return s.cashflow.CancelPurchase(ctx, id)
}

// SetPaymentStatus is the manual PENDING <-> PAID toggle of the financial axis.
func (s *Service) SetPaymentStatus(ctx context.Context, id, status string) error {
	if status != domain.PaymentPending && status != domain.PaymentPaid {
		return domain.ErrInvalid
	}
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status == domain.OrderCancelled {
		return domain.ErrInvalid
	}
	return s.orders.SetPaymentStatus(ctx, id, status)
}

// SetDeliveryStatus is the manual override of the delivery axis (the NF flow — Receber and
// Conferir — moves it too): APPROVED (pending delivery) or CONFERRED (finalized). Finalizing
// by hand does not touch stock. Reopening is refused once stock was received, otherwise a
// later Receber would enter the goods twice.
func (s *Service) SetDeliveryStatus(ctx context.Context, id, status string) error {
	if status != domain.OrderApproved && status != domain.OrderConferred {
		return domain.ErrInvalid
	}
	o, err := s.orders.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Status == domain.OrderCancelled {
		return domain.ErrInvalid
	}
	if o.Status == status {
		return nil
	}
	if status == domain.OrderApproved && o.StockReceived {
		return domain.ErrInvalid
	}
	return s.orders.UpdateStatus(ctx, id, status)
}
