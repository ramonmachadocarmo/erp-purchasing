ALTER TABLE purchase_quotes
    ADD COLUMN subtotal_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN discount_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN delivery_amount NUMERIC(15,2) NOT NULL DEFAULT 0;

UPDATE purchase_quotes SET subtotal_amount = total_amount;
