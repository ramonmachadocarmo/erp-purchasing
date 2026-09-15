package domain

import "testing"

func TestCompareQuotes(t *testing.T) {
	a := Quote{
		ID: "a", SupplierID: "s1", TotalAmount: 30,
		Items: []OrderItem{
			{ProductID: "p1", Quantity: 2, UnitPrice: 10, TotalPrice: 20},
			{ProductID: "p2", Quantity: 1, UnitPrice: 10, TotalPrice: 10},
		},
	}
	b := Quote{
		ID: "b", SupplierID: "s2", TotalAmount: 22,
		Items: []OrderItem{
			{ProductID: "p1", Quantity: 2, UnitPrice: 8, TotalPrice: 16},
			{ProductID: "p2", Quantity: 1, UnitPrice: 6, TotalPrice: 6},
		},
	}
	got := CompareQuotes([]Quote{a, b})
	if got.LowestTotalID != "b" || got.LowestTotal != 22 {
		t.Fatalf("lowest total: %+v", got)
	}
	if len(got.Products) != 2 {
		t.Fatalf("products: %d", len(got.Products))
	}
	for _, p := range got.Products {
		if p.BestQuoteID != "b" {
			t.Fatalf("best for %s: %s", p.ProductID, p.BestQuoteID)
		}
	}
}

func TestTotals(t *testing.T) {
	items, total := Totals([]OrderItem{
		{ProductID: "p", Quantity: 2, UnitPrice: 500},
		{ProductID: "", Quantity: 1, UnitPrice: 10},
	})
	if len(items) != 1 || total != 1000 || items[0].TotalPrice != 1000 {
		t.Fatalf("%v %v", items, total)
	}
}
