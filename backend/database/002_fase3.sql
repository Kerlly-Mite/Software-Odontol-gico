-- Inventario básico de insumos clínicos (Fase 3).
-- Se ejecuta idempotentemente al iniciar la API, también en volúmenes existentes.
CREATE TABLE IF NOT EXISTS inventory_products (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id       BIGINT NOT NULL REFERENCES clinics(id),
    sku             TEXT NOT NULL,
    name            TEXT NOT NULL,
    category        TEXT NOT NULL DEFAULT 'Insumos',
    unit            TEXT NOT NULL DEFAULT 'unidad',
    minimum_stock   NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (minimum_stock >= 0),
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_by      BIGINT NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, sku)
);

CREATE TABLE IF NOT EXISTS inventory_movements (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id      BIGINT NOT NULL REFERENCES inventory_products(id),
    movement_type   TEXT NOT NULL CHECK (movement_type IN ('receipt','consumption','adjustment')),
    quantity        NUMERIC(12,2) NOT NULL CHECK (quantity > 0),
    reference       TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_by      BIGINT NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS inventory_movements_product_time_idx
    ON inventory_movements(product_id, created_at DESC);

CREATE TABLE IF NOT EXISTS inventory_consumption_rules (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id       BIGINT NOT NULL REFERENCES clinics(id),
    procedure_match TEXT NOT NULL,
    product_id      BIGINT NOT NULL REFERENCES inventory_products(id),
    quantity        NUMERIC(12,2) NOT NULL CHECK (quantity > 0),
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_by      BIGINT NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, procedure_match, product_id)
);

CREATE TABLE IF NOT EXISTS provider_commission_rules (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id           BIGINT NOT NULL REFERENCES clinics(id),
    provider_id         BIGINT NOT NULL REFERENCES users(id),
    procedure_match     TEXT NOT NULL DEFAULT '*',
    calculation_type    TEXT NOT NULL CHECK (calculation_type IN ('percentage','fixed','net_percentage')),
    calculation_value   NUMERIC(12,2) NOT NULL CHECK (calculation_value >= 0),
    procedure_fee       NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (procedure_fee >= 0),
    lab_cost            NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (lab_cost >= 0),
    materials_cost      NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (materials_cost >= 0),
    active              BOOLEAN NOT NULL DEFAULT TRUE,
    created_by          BIGINT NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, provider_id, procedure_match)
);

CREATE TABLE IF NOT EXISTS appointment_inventory_consumptions (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    appointment_id  BIGINT NOT NULL REFERENCES appointments(id),
    rule_id         BIGINT NOT NULL REFERENCES inventory_consumption_rules(id),
    movement_id     BIGINT NOT NULL UNIQUE REFERENCES inventory_movements(id),
    consumed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (appointment_id, rule_id)
);
