-- Eixo financeiro do pedido de compra (PENDING -> PAID), independente do eixo de entrega
-- (status: APPROVED = pendente entrega, RECEIVED, CONFERRED = finalizado, CANCELLED).
-- stock_received marca que o estoque já recebeu a mercadoria (Receber), para impedir que uma
-- reabertura manual da entrega faça a entrada em estoque duas vezes.
ALTER TABLE purchase_orders
    ADD COLUMN payment_status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    ADD COLUMN stock_received BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE purchase_orders SET stock_received = TRUE WHERE status IN ('RECEIVED', 'CONFERRED');
