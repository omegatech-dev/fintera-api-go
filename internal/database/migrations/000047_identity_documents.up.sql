-- Identity documents of a person: the DNI and the RTN. The file lives on the
-- person, so every contract of that person shows the same two documents, and
-- a new upload replaces the previous one.
--
-- The number is 000047 because 000046 is already claimed by the pending
-- branch fix/historical-clients-rtn-and-evidence
-- (000046_users_unique_empty_normalization). A gap is harmless for
-- golang-migrate; two files with the same number are not.
CREATE TABLE IF NOT EXISTS identity_documents (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind VARCHAR(16) NOT NULL,
    file_path TEXT NOT NULL,
    preview_path TEXT,
    original_filename TEXT NOT NULL,
    mime_type VARCHAR(120) NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    uploaded_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT identity_documents_user_kind_key UNIQUE (user_id, kind),
    CONSTRAINT identity_documents_kind_check CHECK (kind IN ('dni', 'rtn'))
);

CREATE INDEX IF NOT EXISTS idx_identity_documents_user_id
    ON identity_documents (user_id);

-- Link between a contract and the identity documents of its applicant. It also
-- stores the metadata of the files that were attached to a contract before
-- this feature existed: those rows have no person-level document, no kind and
-- are read-only.
--
-- kind is nullable on purpose. A legacy contract may hold several files and
-- nothing says which one was the DNI, so they are not attributed to a slot.
-- PostgreSQL treats NULL as distinct in a unique constraint, so several
-- legacy rows coexist in the same contract while two rows claiming the same
-- kind do not.
CREATE TABLE IF NOT EXISTS contract_identity_documents (
    id BIGSERIAL PRIMARY KEY,
    contract_id BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    identity_document_id BIGINT REFERENCES identity_documents(id) ON DELETE CASCADE,
    kind VARCHAR(16),
    source VARCHAR(16) NOT NULL DEFAULT 'upload',
    file_path TEXT,
    original_filename TEXT,
    mime_type VARCHAR(120),
    size_bytes BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT contract_identity_documents_contract_kind_key UNIQUE (contract_id, kind),
    CONSTRAINT contract_identity_documents_kind_check CHECK (kind IN ('dni', 'rtn')),
    CONSTRAINT contract_identity_documents_source_check CHECK (source IN ('upload', 'legacy'))
);

CREATE INDEX IF NOT EXISTS idx_contract_identity_documents_contract_id
    ON contract_identity_documents (contract_id);

CREATE INDEX IF NOT EXISTS idx_contract_identity_documents_identity_document_id
    ON contract_identity_documents (identity_document_id);
