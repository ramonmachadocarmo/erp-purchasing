package domain

func CompareQuotes(quotes []Quote) QuoteComparison {
	cmp := QuoteComparison{Quotes: quotes, Products: []ProductCompare{}}
	if len(quotes) == 0 {
		return cmp
	}
	idx := map[string]int{}
	for _, q := range quotes {
		if cmp.LowestTotalID == "" || q.TotalAmount < cmp.LowestTotal {
			cmp.LowestTotalID = q.ID
			cmp.LowestTotal = q.TotalAmount
		}
		for _, it := range q.Items {
			i, ok := idx[it.ProductID]
			if !ok {
				idx[it.ProductID] = len(cmp.Products)
				cmp.Products = append(cmp.Products, ProductCompare{ProductID: it.ProductID, Lines: []QuoteLine{}})
				i = idx[it.ProductID]
			}
			line := QuoteLine{
				QuoteID: q.ID, SupplierID: q.SupplierID,
				Quantity: it.Quantity, UnitPrice: it.UnitPrice, TotalPrice: it.TotalPrice,
			}
			p := &cmp.Products[i]
			p.Lines = append(p.Lines, line)
			if p.BestQuoteID == "" || it.UnitPrice < p.BestUnitPrice {
				p.BestQuoteID = q.ID
				p.BestUnitPrice = it.UnitPrice
			}
		}
	}
	return cmp
}
