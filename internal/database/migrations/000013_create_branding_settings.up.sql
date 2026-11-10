CREATE TABLE IF NOT EXISTS branding_settings (
    id BIGSERIAL PRIMARY KEY,
    app_name VARCHAR(60) NOT NULL DEFAULT 'Fintera',
    logo_path VARCHAR(255),
    logo_dark_path VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT branding_settings_singleton CHECK (id = 1)
);

INSERT INTO branding_settings (id, app_name)
VALUES (1, 'Fintera')
ON CONFLICT (id) DO NOTHING;
