CREATE TABLE IF NOT EXISTS status_lists (
    tenant_id   TEXT NOT NULL,
    list_id     BIGINT GENERATED ALWAYS AS IDENTITY,

    type        TEXT NOT NULL DEFAULT 'StatusList2021',
    version     INTEGER NOT NULL DEFAULT 1,

    capacity    INTEGER NOT NULL DEFAULT 131072,
    next_index  INTEGER NOT NULL DEFAULT 0,
    bitstring   BYTEA NOT NULL,

    did         TEXT NOT NULL,
    key_ref     TEXT NOT NULL,
    namespace   TEXT NOT NULL,
    "group"     TEXT,
    origin      TEXT NOT NULL,

    purpose     TEXT,
    status_url  TEXT,

    max_expiration_date TIMESTAMPTZ,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, list_id),

    CONSTRAINT status_lists_next_index_check
        CHECK (next_index >= 0 AND next_index <= capacity),

    CONSTRAINT status_lists_capacity_check
        CHECK (capacity > 0)
);

CREATE INDEX IF NOT EXISTS idx_status_lists_tenant_type
    ON status_lists (tenant_id, type);

CREATE INDEX IF NOT EXISTS idx_status_lists_tenant_updated
    ON status_lists (tenant_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_status_lists_signer
    ON status_lists (tenant_id, did, key_ref, namespace);

CREATE INDEX IF NOT EXISTS idx_status_lists_origin
    ON status_lists (tenant_id, origin);

CREATE INDEX IF NOT EXISTS idx_status_lists_expiration
    ON status_lists (tenant_id, max_expiration_date);