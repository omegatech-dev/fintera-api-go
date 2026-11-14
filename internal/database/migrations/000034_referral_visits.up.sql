-- Public referral link visits. The visitor session is stored as a hash of the
-- IP and user agent, never as raw personal data.
CREATE TABLE IF NOT EXISTS referral_visits (
    id BIGSERIAL PRIMARY KEY,
    seller_id BIGINT NOT NULL,
    slug VARCHAR(80) NOT NULL,
    visited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    path VARCHAR(500),
    session_hash VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_referral_visits_seller FOREIGN KEY (seller_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_referral_visits_seller_visited ON referral_visits (seller_id, visited_at);
CREATE INDEX IF NOT EXISTS idx_referral_visits_slug_visited ON referral_visits (slug, visited_at);
CREATE INDEX IF NOT EXISTS idx_referral_visits_visited_at ON referral_visits (visited_at);
