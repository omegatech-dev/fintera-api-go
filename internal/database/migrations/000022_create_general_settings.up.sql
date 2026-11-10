-- Customer-facing settings (support contact and reservation hold duration).
CREATE TABLE IF NOT EXISTS general_settings (
    id BIGSERIAL PRIMARY KEY,
    support_whatsapp VARCHAR(32) NOT NULL DEFAULT '',
    support_email VARCHAR(160) NOT NULL DEFAULT '',
    reservation_hold_hours INTEGER NOT NULL DEFAULT 48,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO general_settings (id, support_whatsapp, support_email, reservation_hold_hours)
VALUES (1, '', '', 48)
ON CONFLICT (id) DO NOTHING;
