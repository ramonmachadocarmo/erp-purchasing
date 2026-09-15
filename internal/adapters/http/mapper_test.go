package httpadapter

import (
	"testing"
	"time"

	"erp-schema/model"
	"erp/services/purchasing-service/internal/domain"
)

func TestToDomainQuote(t *testing.T) {
	id := "q1"
	discount, delivery := 5.0, 3.0
	lineID := "line1"
	totalPrice := 20.0
	in := model.Quote{
		ID:             &id,
		SupplierID:     "sup-1",
		Notes:          "urgent",
		DiscountAmount: &discount,
		DeliveryAmount: &delivery,
		Items: []model.QuoteLine{
			{ID: &lineID, ProductID: "p1", Quantity: 2, UnitPrice: 10, TotalPrice: &totalPrice},
		},
	}
	got := toDomainQuote(in)
	if got.ID != "q1" || got.SupplierID != "sup-1" || got.Notes != "urgent" {
		t.Fatalf("basic fields mismatch: %+v", got)
	}
	// These two are exactly the fields whose absence caused the original mobile drift bug.
	if got.DiscountAmount != 5 || got.DeliveryAmount != 3 {
		t.Fatalf("discount/delivery not mapped: %+v", got)
	}
	if len(got.Items) != 1 || got.Items[0].ID != "line1" || got.Items[0].ProductID != "p1" ||
		got.Items[0].Quantity != 2 || got.Items[0].UnitPrice != 10 || got.Items[0].TotalPrice != 20 {
		t.Fatalf("item not mapped: %+v", got.Items)
	}
}

func TestToDomainQuoteOmitsOptionalServerFields(t *testing.T) {
	// A create request has no id/discount/delivery — must not panic on nil pointers, and
	// must leave the domain fields at their zero value.
	in := model.Quote{
		SupplierID: "sup-1",
		Notes:      "",
		Items:      []model.QuoteLine{{ProductID: "p1", Quantity: 1, UnitPrice: 5}},
	}
	got := toDomainQuote(in)
	if got.ID != "" || got.DiscountAmount != 0 || got.DeliveryAmount != 0 {
		t.Fatalf("expected zero values for absent optional fields: %+v", got)
	}
	if got.Items[0].ID != "" || got.Items[0].TotalPrice != 0 {
		t.Fatalf("expected zero values for absent optional item fields: %+v", got.Items[0])
	}
}

func TestFromDomainQuote(t *testing.T) {
	created := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	in := domain.Quote{
		ID: "q1", SupplierID: "sup-1", Status: domain.QuoteOpen, Notes: "n",
		SubtotalAmount: 100, DiscountAmount: 10, DeliveryAmount: 5, TotalAmount: 95,
		Items:     []domain.OrderItem{{ID: "line1", ProductID: "p1", Quantity: 2, UnitPrice: 10, TotalPrice: 20}},
		CreatedAt: created,
	}
	got := fromDomainQuote(in)
	if got.ID == nil || *got.ID != "q1" {
		t.Fatalf("id not mapped: %+v", got.ID)
	}
	if got.Status == nil || string(*got.Status) != domain.QuoteOpen {
		t.Fatalf("status not mapped: %+v", got.Status)
	}
	if got.DiscountAmount == nil || *got.DiscountAmount != 10 || got.DeliveryAmount == nil || *got.DeliveryAmount != 5 {
		t.Fatalf("discount/delivery not mapped: %+v", got)
	}
	if got.SubtotalAmount == nil || *got.SubtotalAmount != 100 || got.TotalAmount == nil || *got.TotalAmount != 95 {
		t.Fatalf("subtotal/total not mapped: %+v", got)
	}
	if got.CreatedAt == nil || !got.CreatedAt.Equal(created) {
		t.Fatalf("created_at not mapped: %+v", got.CreatedAt)
	}
	if len(got.Items) != 1 || *got.Items[0].ID != "line1" || got.Items[0].ProductID != "p1" {
		t.Fatalf("item not mapped: %+v", got.Items)
	}
}

func TestFromDomainQuoteOmitsUnsetIdentifiersAndTimestamp(t *testing.T) {
	// A quote about to be created has no ID/Status/CreatedAt yet — confirms these become
	// nil (omitted from JSON) rather than zero-valued placeholders like "" or "0001-01-01".
	in := domain.Quote{SupplierID: "sup-1", Items: []domain.OrderItem{{ProductID: "p1", Quantity: 1, UnitPrice: 1}}}
	got := fromDomainQuote(in)
	if got.ID != nil || got.Status != nil || got.CreatedAt != nil {
		t.Fatalf("expected nil id/status/created_at, got %+v / %+v / %+v", got.ID, got.Status, got.CreatedAt)
	}
	if got.Items[0].ID != nil {
		t.Fatalf("expected nil item id, got %+v", got.Items[0].ID)
	}
}

func TestFromDomainQuotes(t *testing.T) {
	in := []domain.Quote{
		{ID: "q1", SupplierID: "s1"},
		{ID: "q2", SupplierID: "s2"},
	}
	got := fromDomainQuotes(in)
	if len(got) != 2 || *got[0].ID != "q1" || *got[1].ID != "q2" {
		t.Fatalf("slice mapping mismatch: %+v", got)
	}
}

func TestFromDomainQuotesEmpty(t *testing.T) {
	got := fromDomainQuotes(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %+v", got)
	}
}

// Round-trip: toDomainQuote(fromDomainQuote(x)) should preserve every field that a client
// is allowed to send, confirming the two mappers agree with each other's shape.
func TestQuoteMapperRoundTrip(t *testing.T) {
	in := domain.Quote{
		SupplierID: "sup-1", Notes: "n", DiscountAmount: 10, DeliveryAmount: 5,
		Items: []domain.OrderItem{{ProductID: "p1", Quantity: 2, UnitPrice: 10, TotalPrice: 20}},
	}
	got := toDomainQuote(fromDomainQuote(in))
	if got.SupplierID != in.SupplierID || got.Notes != in.Notes ||
		got.DiscountAmount != in.DiscountAmount || got.DeliveryAmount != in.DeliveryAmount {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, in)
	}
}
