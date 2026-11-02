-- +goose Up
CREATE TABLE users (
    tg_id         BIGINT PRIMARY KEY,
    username      TEXT,
    first_name    TEXT NOT NULL DEFAULT '',
    last_name     TEXT,
    phone         TEXT,
    role          TEXT NOT NULL DEFAULT 'user',
    is_registered BOOLEAN NOT NULL DEFAULT false,
    is_blocked    BOOLEAN NOT NULL DEFAULT false,
    seller_rating NUMERIC NOT NULL DEFAULT 0,
    buyer_rating  NUMERIC NOT NULL DEFAULT 0,
    seller_deals  INT NOT NULL DEFAULT 0,
    buyer_deals   INT NOT NULL DEFAULT 0,
    registered_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX users_phone_idx ON users (phone) WHERE phone IS NOT NULL;
CREATE INDEX users_username_idx ON users (lower(username));

CREATE TABLE username_history (
    id            BIGSERIAL PRIMARY KEY,
    tg_id         BIGINT NOT NULL,
    username      TEXT NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tg_id, username)
);
CREATE INDEX username_history_username_idx ON username_history (lower(username));

CREATE TABLE deals (
    id                 BIGSERIAL PRIMARY KEY,
    chat_id            BIGINT NOT NULL,
    guarantor_id       BIGINT NOT NULL REFERENCES users (tg_id),
    seller_id          BIGINT REFERENCES users (tg_id),
    buyer_id           BIGINT REFERENCES users (tg_id),
    seller_confirmed   BOOLEAN NOT NULL DEFAULT false,
    buyer_confirmed    BOOLEAN NOT NULL DEFAULT false,
    status             TEXT NOT NULL,
    event_name         TEXT NOT NULL,
    event_at           TIMESTAMPTZ NOT NULL,
    tickets_count      INT NOT NULL,
    currency           TEXT NOT NULL,
    amount             BIGINT NOT NULL,
    commission_pct     NUMERIC(5, 2) NOT NULL,
    commission_payer   TEXT NOT NULL,
    commission_amount  BIGINT NOT NULL,
    buyer_pays         BIGINT NOT NULL,
    seller_receives    BIGINT NOT NULL,
    payout_details     TEXT,
    seller_invite_link TEXT,
    buyer_invite_link  TEXT,
    deadline_at        TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at          TIMESTAMPTZ
);
-- одна активная сделка на группу
CREATE UNIQUE INDEX deals_active_chat_idx ON deals (chat_id)
    WHERE status NOT IN ('CLOSED', 'RESOLVED', 'CANCELLED');
CREATE INDEX deals_status_idx ON deals (status);
CREATE INDEX deals_deadline_idx ON deals (deadline_at) WHERE deadline_at IS NOT NULL;

CREATE TABLE scam_entries (
    id            BIGSERIAL PRIMARY KEY,
    type          TEXT NOT NULL,
    value         TEXT NOT NULL,
    display       TEXT,
    reason        TEXT NOT NULL,
    deal_id       BIGINT REFERENCES deals (id),
    source        TEXT NOT NULL,
    added_by      BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    removed_at    TIMESTAMPTZ,
    removed_by    BIGINT,
    remove_reason TEXT
);
CREATE UNIQUE INDEX scam_entries_active_idx ON scam_entries (type, value) WHERE removed_at IS NULL;

CREATE TABLE deal_status_log (
    id          BIGSERIAL PRIMARY KEY,
    deal_id     BIGINT NOT NULL REFERENCES deals (id),
    from_status TEXT,
    to_status   TEXT NOT NULL,
    actor_id    BIGINT,
    comment     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX deal_status_log_deal_idx ON deal_status_log (deal_id, id);

CREATE TABLE deal_files (
    id             BIGSERIAL PRIMARY KEY,
    deal_id        BIGINT NOT NULL REFERENCES deals (id),
    kind           TEXT NOT NULL,
    file_id        TEXT NOT NULL,
    file_unique_id TEXT NOT NULL,
    file_name      TEXT,
    mime_type      TEXT,
    uploaded_by    BIGINT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX deal_files_deal_idx ON deal_files (deal_id);

CREATE TABLE message_log (
    id             BIGSERIAL PRIMARY KEY,
    chat_id        BIGINT NOT NULL,
    message_id     BIGINT NOT NULL,
    version        INT NOT NULL DEFAULT 0,
    deal_id        BIGINT REFERENCES deals (id),
    user_id        BIGINT NOT NULL,
    text           TEXT,
    file_id        TEXT,
    log_message_id BIGINT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (chat_id, message_id, version)
);
CREATE INDEX message_log_deal_idx ON message_log (deal_id);

CREATE TABLE disputes (
    id                 BIGSERIAL PRIMARY KEY,
    deal_id            BIGINT NOT NULL REFERENCES deals (id),
    opened_by          BIGINT,
    reason             TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'open',
    resolution         TEXT,
    seller_amount      BIGINT,
    buyer_refund       BIGINT,
    fault              TEXT,
    scam_flagged       BOOLEAN NOT NULL DEFAULT false,
    resolved_by        BIGINT,
    resolution_comment TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at        TIMESTAMPTZ
);
CREATE INDEX disputes_open_idx ON disputes (status) WHERE status = 'open';

CREATE TABLE reviews (
    id          BIGSERIAL PRIMARY KEY,
    deal_id     BIGINT NOT NULL REFERENCES deals (id),
    author_id   BIGINT NOT NULL REFERENCES users (tg_id),
    target_id   BIGINT NOT NULL REFERENCES users (tg_id),
    target_role TEXT NOT NULL,
    score       SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
    text        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (deal_id, author_id)
);
CREATE INDEX reviews_target_idx ON reviews (target_id);

CREATE TABLE rating_changes (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (tg_id),
    deal_id    BIGINT NOT NULL REFERENCES deals (id),
    role       TEXT NOT NULL,
    delta      NUMERIC NOT NULL,
    reason     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX rating_changes_user_idx ON rating_changes (user_id);

CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_by BIGINT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE conversation_state (
    user_id    BIGINT NOT NULL,
    chat_id    BIGINT NOT NULL,
    flow       TEXT NOT NULL,
    step       TEXT NOT NULL,
    data       JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, chat_id)
);

-- +goose Down
DROP TABLE conversation_state, settings, rating_changes, reviews, disputes,
    message_log, deal_files, deal_status_log, scam_entries, deals,
    username_history, users;
