CREATE TABLE purchase_quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    supplier_id UUID NOT NULL,
    status VARCHAR(30) NOT NULL,
    total_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE purchase_quote_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id UUID NOT NULL REFERENCES purchase_quotes(id) ON DELETE CASCADE,
    product_id VARCHAR(50) NOT NULL,
    quantity NUMERIC(15,4) NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(15,4) NOT NULL CHECK (unit_price >= 0),
    total_price NUMERIC(15,2) NOT NULL
);

ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS quote_id UUID REFERENCES purchase_quotes(id);

CREATE INDEX idx_purchase_quotes_supplier ON purchase_quotes(supplier_id);
CREATE INDEX idx_purchase_quotes_status ON purchase_quotes(status);
CREATE INDEX idx_purchase_quote_items_quote ON purchase_quote_items(quote_id);
CREATE INDEX idx_purchase_orders_quote ON purchase_orders(quote_id);
