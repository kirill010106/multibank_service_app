CREATE TYPE bank_provider AS ENUM ('vbank', 'sbank', 'abank');

CREATE TYPE connection_status AS ENUM (
    'pending',
    'active',
    'expired',
    'revoked'
);

CREATE TABLE bank_connections (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bank_provider bank_provider NOT NULL,

    bank_client_id VARCHAR(255) NOT NULL,
    consent_id VARCHAR(255) NOT NULL,
    access_token TEXT NOT NULL,
    token_expires_at TIMESTAMP,

    status connection_status NOT NULL DEFAULT 'pending',

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE (user_id, bank_provider, bank_client_id)
);

CREATE INDEX idx_bank_connections_user ON bank_connections(user_id);
CREATE INDEX idx_bank_connections_status ON bank_connections(status);

COMMENT ON TABLE bank_connections IS 'Подключения юзеров к банкам через Open Banking';
COMMENT ON COLUMN bank_connections.bank_client_id IS 'ID клиента в банке (team200-1)';
COMMENT ON COLUMN bank_connections.consent_id IS 'ID согласия для межбанковых запросов';
COMMENT ON COLUMN bank_connections.access_token IS 'Bank token для API запросов';