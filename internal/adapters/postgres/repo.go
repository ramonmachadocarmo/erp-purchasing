package postgres

import (
	"context"
	"errors"

	"erp/services/purchasing-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

type Orders struct{ *Repo }

func (o Orders) Create(ctx context.Context, po domain.PurchaseOrder) (domain.PurchaseOrder, error) {
	tx, err := o.pool.Begin(ctx)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `
		INSERT INTO purchase_orders (supplier_id, status, total_amount, expected_delivery_date, quote_id, payment_method_id, payment_term_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at
	`, po.SupplierID, po.Status, po.TotalAmount, po.ExpectedDeliveryDate, po.QuoteID, po.PaymentMethodID, po.PaymentTermID).Scan(&po.ID, &po.CreatedAt)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	for i, item := range po.Items {
		if err := tx.QueryRow(ctx, `
			INSERT INTO purchase_order_items (purchase_order_id, product_id, quantity, unit_price, total_price)
			VALUES ($1,$2,$3,$4,$5) RETURNING id
		`, po.ID, item.ProductID, item.Quantity, item.UnitPrice, item.TotalPrice).Scan(&po.Items[i].ID); err != nil {
			return domain.PurchaseOrder{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PurchaseOrder{}, err
	}
	return po, nil
}

func (o Orders) Get(ctx context.Context, id string) (domain.PurchaseOrder, error) {
	var po domain.PurchaseOrder
	err := o.pool.QueryRow(ctx, `
		SELECT id, supplier_id, status, total_amount, expected_delivery_date, created_at, quote_id, payment_method_id, payment_term_id
		FROM purchase_orders WHERE id=$1
	`, id).Scan(&po.ID, &po.SupplierID, &po.Status, &po.TotalAmount, &po.ExpectedDeliveryDate, &po.CreatedAt, &po.QuoteID, &po.PaymentMethodID, &po.PaymentTermID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PurchaseOrder{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	items, err := o.items(ctx, po.ID)
	po.Items = items
	return po, err
}

func (o Orders) List(ctx context.Context) ([]domain.PurchaseOrder, error) {
	rows, err := o.pool.Query(ctx, `
		SELECT id, supplier_id, status, total_amount, expected_delivery_date, created_at, quote_id, payment_method_id, payment_term_id
		FROM purchase_orders ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PurchaseOrder
	for rows.Next() {
		var po domain.PurchaseOrder
		if err := rows.Scan(&po.ID, &po.SupplierID, &po.Status, &po.TotalAmount, &po.ExpectedDeliveryDate, &po.CreatedAt, &po.QuoteID, &po.PaymentMethodID, &po.PaymentTermID); err != nil {
			return nil, err
		}
		items, err := o.items(ctx, po.ID)
		if err != nil {
			return nil, err
		}
		po.Items = items
		out = append(out, po)
	}
	if out == nil {
		out = []domain.PurchaseOrder{}
	}
	return out, rows.Err()
}

func (o Orders) UpdateStatus(ctx context.Context, id, status string) error {
	tag, err := o.pool.Exec(ctx, `UPDATE purchase_orders SET status=$2 WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (o Orders) items(ctx context.Context, id string) ([]domain.OrderItem, error) {
	rows, err := o.pool.Query(ctx, `
		SELECT id, product_id, quantity, unit_price, total_price FROM purchase_order_items WHERE purchase_order_id=$1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OrderItem
	for rows.Next() {
		var i domain.OrderItem
		if err := rows.Scan(&i.ID, &i.ProductID, &i.Quantity, &i.UnitPrice, &i.TotalPrice); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	if out == nil {
		out = []domain.OrderItem{}
	}
	return out, rows.Err()
}

type Quotes struct{ *Repo }

func (q Quotes) Create(ctx context.Context, quote domain.Quote) (domain.Quote, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return domain.Quote{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `
		INSERT INTO purchase_quotes (supplier_id, status, subtotal_amount, discount_amount, delivery_amount, total_amount, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at
	`, quote.SupplierID, quote.Status, quote.SubtotalAmount, quote.DiscountAmount, quote.DeliveryAmount, quote.TotalAmount, quote.Notes).Scan(&quote.ID, &quote.CreatedAt)
	if err != nil {
		return domain.Quote{}, err
	}
	for i, item := range quote.Items {
		if err := tx.QueryRow(ctx, `
			INSERT INTO purchase_quote_items (quote_id, product_id, quantity, unit_price, total_price)
			VALUES ($1,$2,$3,$4,$5) RETURNING id
		`, quote.ID, item.ProductID, item.Quantity, item.UnitPrice, item.TotalPrice).Scan(&quote.Items[i].ID); err != nil {
			return domain.Quote{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Quote{}, err
	}
	return quote, nil
}

func (q Quotes) Get(ctx context.Context, id string) (domain.Quote, error) {
	var quote domain.Quote
	err := q.pool.QueryRow(ctx, `
		SELECT id, supplier_id, status, subtotal_amount, discount_amount, delivery_amount, total_amount, notes, created_at
		FROM purchase_quotes WHERE id=$1
	`, id).Scan(&quote.ID, &quote.SupplierID, &quote.Status, &quote.SubtotalAmount, &quote.DiscountAmount, &quote.DeliveryAmount, &quote.TotalAmount, &quote.Notes, &quote.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Quote{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Quote{}, err
	}
	items, err := q.quoteItems(ctx, quote.ID)
	quote.Items = items
	return quote, err
}

func (q Quotes) List(ctx context.Context) ([]domain.Quote, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, supplier_id, status, subtotal_amount, discount_amount, delivery_amount, total_amount, notes, created_at
		FROM purchase_quotes ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Quote
	for rows.Next() {
		var quote domain.Quote
		if err := rows.Scan(&quote.ID, &quote.SupplierID, &quote.Status, &quote.SubtotalAmount, &quote.DiscountAmount, &quote.DeliveryAmount, &quote.TotalAmount, &quote.Notes, &quote.CreatedAt); err != nil {
			return nil, err
		}
		items, err := q.quoteItems(ctx, quote.ID)
		if err != nil {
			return nil, err
		}
		quote.Items = items
		out = append(out, quote)
	}
	if out == nil {
		out = []domain.Quote{}
	}
	return out, rows.Err()
}

func (q Quotes) Update(ctx context.Context, quote domain.Quote) (domain.Quote, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return domain.Quote{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE purchase_quotes SET supplier_id=$2, notes=$3, subtotal_amount=$4, discount_amount=$5, delivery_amount=$6, total_amount=$7
		WHERE id=$1 AND status=$8
	`, quote.ID, quote.SupplierID, quote.Notes, quote.SubtotalAmount, quote.DiscountAmount, quote.DeliveryAmount, quote.TotalAmount, domain.QuoteOpen)
	if err != nil {
		return domain.Quote{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Quote{}, domain.ErrInvalid
	}
	if _, err := tx.Exec(ctx, `DELETE FROM purchase_quote_items WHERE quote_id=$1`, quote.ID); err != nil {
		return domain.Quote{}, err
	}
	for i, item := range quote.Items {
		if err := tx.QueryRow(ctx, `
			INSERT INTO purchase_quote_items (quote_id, product_id, quantity, unit_price, total_price)
			VALUES ($1,$2,$3,$4,$5) RETURNING id
		`, quote.ID, item.ProductID, item.Quantity, item.UnitPrice, item.TotalPrice).Scan(&quote.Items[i].ID); err != nil {
			return domain.Quote{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Quote{}, err
	}
	return quote, nil
}

func (q Quotes) Delete(ctx context.Context, id string) error {
	tag, err := q.pool.Exec(ctx, `DELETE FROM purchase_quotes WHERE id=$1 AND status=$2`, id, domain.QuoteOpen)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (q Quotes) Convert(ctx context.Context, quote domain.Quote, methodID, termID string) (domain.PurchaseOrder, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE purchase_quotes SET status=$2 WHERE id=$1 AND status=$3`, quote.ID, domain.QuoteConverted, domain.QuoteOpen)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.PurchaseOrder{}, domain.ErrInvalid
	}
	po := domain.PurchaseOrder{
		SupplierID:      quote.SupplierID,
		QuoteID:         &quote.ID,
		PaymentMethodID: methodID,
		PaymentTermID:   termID,
		Status:          domain.OrderApproved,
		TotalAmount:     quote.TotalAmount,
		Items:           quote.Items,
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO purchase_orders (supplier_id, status, total_amount, quote_id, payment_method_id, payment_term_id)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at
	`, po.SupplierID, po.Status, po.TotalAmount, po.QuoteID, po.PaymentMethodID, po.PaymentTermID).Scan(&po.ID, &po.CreatedAt)
	if err != nil {
		return domain.PurchaseOrder{}, err
	}
	for i, item := range po.Items {
		if err := tx.QueryRow(ctx, `
			INSERT INTO purchase_order_items (purchase_order_id, product_id, quantity, unit_price, total_price)
			VALUES ($1,$2,$3,$4,$5) RETURNING id
		`, po.ID, item.ProductID, item.Quantity, item.UnitPrice, item.TotalPrice).Scan(&po.Items[i].ID); err != nil {
			return domain.PurchaseOrder{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PurchaseOrder{}, err
	}
	return po, nil
}

func (q Quotes) quoteItems(ctx context.Context, id string) ([]domain.OrderItem, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, product_id, quantity, unit_price, total_price FROM purchase_quote_items WHERE quote_id=$1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OrderItem
	for rows.Next() {
		var i domain.OrderItem
		if err := rows.Scan(&i.ID, &i.ProductID, &i.Quantity, &i.UnitPrice, &i.TotalPrice); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	if out == nil {
		out = []domain.OrderItem{}
	}
	return out, rows.Err()
}
