CREATE TABLE IF NOT EXISTS trucks (
    id VARCHAR(50) PRIMARY KEY,
    capacity INT NOT NULL CHECK (capacity > 0),
    current_load INT NOT NULL DEFAULT 0 CHECK (current_load >= 0 AND current_load <= capacity),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
