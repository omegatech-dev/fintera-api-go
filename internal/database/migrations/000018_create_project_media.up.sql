-- Project media: drawings, project gallery and lot gallery.
CREATE TABLE IF NOT EXISTS project_drawings (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_type VARCHAR(10) NOT NULL,
    version VARCHAR(50),
    size_bytes BIGINT NOT NULL DEFAULT 0,
    note TEXT,
    uploaded_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_drawings_project
    ON project_drawings(project_id, created_at);

CREATE TABLE IF NOT EXISTS project_photos (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    file_path VARCHAR(500) NOT NULL,
    thumb_path VARCHAR(500),
    position INTEGER NOT NULL DEFAULT 0,
    alt VARCHAR(255),
    uploaded_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_photos_project
    ON project_photos(project_id, position);

CREATE TABLE IF NOT EXISTS lot_photos (
    id BIGSERIAL PRIMARY KEY,
    lot_id BIGINT NOT NULL REFERENCES lots(id) ON DELETE CASCADE,
    file_path VARCHAR(500) NOT NULL,
    thumb_path VARCHAR(500),
    position INTEGER NOT NULL DEFAULT 0,
    alt VARCHAR(255),
    uploaded_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lot_photos_lot
    ON lot_photos(lot_id, position);
