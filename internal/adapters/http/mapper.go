package httpadapter

import (
	"erp-schema/model"
	"erp/services/purchasing-service/internal/domain"
)

// toDomainQuote and fromDomainQuote map the shared erp-schema wire type at the HTTP
// boundary only — domain.Quote (internal/domain/purchasing.go) stays this service's own
// private model and is never replaced by the generated type. See ../../../../../erp-schema
// (sibling repo) for the schema source and why only Quote/Address are shared at all.

func toDomainQuote(m model.Quote) domain.Quote {
	items := make([]domain.OrderItem, len(m.Items))
	for i, it := range m.Items {
		item := domain.OrderItem{ProductID: it.ProductID, Quantity: it.Quantity, UnitPrice: it.UnitPrice}
		if it.TotalPrice != nil {
			item.TotalPrice = *it.TotalPrice
		}
		if it.ID != nil {
			item.ID = *it.ID
		}
		items[i] = item
	}
	q := domain.Quote{SupplierID: m.SupplierID, Notes: m.Notes, Items: items}
	if m.ID != nil {
		q.ID = *m.ID
	}
	if m.DiscountAmount != nil {
		q.DiscountAmount = *m.DiscountAmount
	}
	if m.DeliveryAmount != nil {
		q.DeliveryAmount = *m.DeliveryAmount
	}
	return q
}

func fromDomainQuote(q domain.Quote) model.Quote {
	items := make([]model.QuoteLine, len(q.Items))
	for i, it := range q.Items {
		totalPrice := it.TotalPrice
		line := model.QuoteLine{ProductID: it.ProductID, Quantity: it.Quantity, UnitPrice: it.UnitPrice, TotalPrice: &totalPrice}
		if it.ID != "" {
			id := it.ID
			line.ID = &id
		}
		items[i] = line
	}
	subtotal, discount, delivery, total := q.SubtotalAmount, q.DiscountAmount, q.DeliveryAmount, q.TotalAmount
	m := model.Quote{
		SupplierID:     q.SupplierID,
		Notes:          q.Notes,
		Items:          items,
		SubtotalAmount: &subtotal,
		DiscountAmount: &discount,
		DeliveryAmount: &delivery,
		TotalAmount:    &total,
	}
	if q.ID != "" {
		id := q.ID
		m.ID = &id
	}
	if q.Status != "" {
		status := model.QuoteStatus(q.Status)
		m.Status = &status
	}
	if !q.CreatedAt.IsZero() {
		t := q.CreatedAt
		m.CreatedAt = &t
	}
	return m
}

func fromDomainQuotes(qs []domain.Quote) []model.Quote {
	out := make([]model.Quote, len(qs))
	for i, q := range qs {
		out[i] = fromDomainQuote(q)
	}
	return out
}
