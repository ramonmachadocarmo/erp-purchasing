CREATE TABLE suppliers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cnpj_cpf VARCHAR(18) NOT NULL UNIQUE,
    company_name VARCHAR(150) NOT NULL,
    trade_name VARCHAR(150),
    email VARCHAR(100) NOT NULL,
    phone VARCHAR(20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE purchase_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    supplier_id UUID NOT NULL REFERENCES suppliers(id) ON DELETE RESTRICT,
    status VARCHAR(30) NOT NULL,
    total_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    expected_delivery_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE purchase_order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
    product_id VARCHAR(50) NOT NULL,
    quantity NUMERIC(15,4) NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(15,4) NOT NULL CHECK (unit_price >= 0),
    total_price NUMERIC(15,2) NOT NULL
);

CREATE INDEX idx_suppliers_cnpj ON suppliers(cnpj_cpf);
CREATE INDEX idx_purchase_orders_supplier ON purchase_orders(supplier_id);
CREATE INDEX idx_purchase_orders_status ON purchase_orders(status);
CREATE INDEX idx_purchase_order_items_po ON purchase_order_items(purchase_order_id);
