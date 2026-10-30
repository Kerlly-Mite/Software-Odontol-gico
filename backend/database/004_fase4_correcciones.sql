-- Correcciones priorizadas de Fase 4.
ALTER TABLE appointments
    ADD COLUMN IF NOT EXISTS plan_item_id BIGINT REFERENCES treatment_plan_items(id);

CREATE TABLE IF NOT EXISTS cash_closures (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id       BIGINT NOT NULL REFERENCES clinics(id),
    closed_by       BIGINT NOT NULL REFERENCES users(id),
    period_start    TIMESTAMPTZ NOT NULL,
    closed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    declared_cash   NUMERIC(12,2) NOT NULL CHECK (declared_cash >= 0),
    expected_cash   NUMERIC(12,2) NOT NULL CHECK (expected_cash >= 0),
    total_collected NUMERIC(12,2) NOT NULL CHECK (total_collected >= 0),
    payment_count   INTEGER NOT NULL CHECK (payment_count >= 0),
    difference      NUMERIC(12,2) NOT NULL
);
CREATE INDEX IF NOT EXISTS cash_closures_clinic_time_idx
    ON cash_closures(clinic_id, closed_at DESC);
