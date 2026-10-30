-- Esquema inicial SIGO-Dental para las Fases 1 y 2.
-- PostgreSQL 15+. Aplicar en una base nueva; no contiene datos clínicos.

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE clinics (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            TEXT NOT NULL,
    timezone        TEXT NOT NULL DEFAULT 'America/Guayaquil',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    full_name       TEXT NOT NULL,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    mfa_enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE clinic_members (
    clinic_id       BIGINT NOT NULL REFERENCES clinics(id),
    user_id         BIGINT NOT NULL REFERENCES users(id),
    role            TEXT NOT NULL CHECK (role IN ('admin_ti','recepcionista','odontologo','asistente','auditor')),
    specialty       TEXT,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (clinic_id, user_id)
);

CREATE TABLE chairs (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id       BIGINT NOT NULL REFERENCES clinics(id),
    name            TEXT NOT NULL,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE (clinic_id, name)
);

CREATE TABLE patients (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id           BIGINT NOT NULL REFERENCES clinics(id),
    national_id         TEXT NOT NULL,
    given_names         TEXT NOT NULL,
    family_names        TEXT NOT NULL,
    birth_date          DATE,
    age_years           SMALLINT CHECK (age_years IS NULL OR age_years BETWEEN 0 AND 125),
    phone               TEXT,
    email               TEXT,
    emergency_contact   TEXT,
    blood_type          TEXT CHECK (blood_type IS NULL OR blood_type IN ('O+','O-','A+','A-','B+','B-','AB+','AB-')),
    insurance_name      TEXT,
    active              BOOLEAN NOT NULL DEFAULT TRUE,
    created_by          BIGINT NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, national_id)
);

CREATE TABLE patient_guardians (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    patient_id      BIGINT NOT NULL REFERENCES patients(id),
    full_name       TEXT NOT NULL,
    national_id     TEXT,
    relationship    TEXT NOT NULL,
    phone           TEXT,
    legal_guardian  BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE medical_alerts (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    patient_id      BIGINT NOT NULL REFERENCES patients(id),
    category        TEXT NOT NULL CHECK (category IN ('allergy','condition','medication','surgery','coagulation','other')),
    description     TEXT NOT NULL,
    critical        BOOLEAN NOT NULL DEFAULT FALSE,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    recorded_by     BIGINT NOT NULL REFERENCES users(id),
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE appointments (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id       BIGINT NOT NULL REFERENCES clinics(id),
    patient_id      BIGINT NOT NULL REFERENCES patients(id),
    chair_id        BIGINT NOT NULL REFERENCES chairs(id),
    provider_id     BIGINT NOT NULL REFERENCES users(id),
    starts_at       TIMESTAMPTZ NOT NULL,
    ends_at         TIMESTAMPTZ NOT NULL,
    reason          TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled','confirmed','waiting','in_progress','attended','cancelled','no_show')),
    created_by      BIGINT NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at),
    EXCLUDE USING gist (chair_id WITH =, tstzrange(starts_at, ends_at, '[)') WITH &&)
        WHERE (status NOT IN ('cancelled','no_show'))
);

CREATE INDEX appointments_provider_time_idx ON appointments(provider_id, starts_at);
CREATE INDEX appointments_patient_time_idx ON appointments(patient_id, starts_at DESC);

CREATE TABLE treatment_plans (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    patient_id      BIGINT NOT NULL REFERENCES patients(id),
    provider_id     BIGINT NOT NULL REFERENCES users(id),
    title           TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','presented','partially_approved','approved','in_progress','completed','cancelled')),
    subtotal        NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    discount        NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (discount >= 0),
    total           NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (total >= 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE treatment_plan_items (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plan_id         BIGINT NOT NULL REFERENCES treatment_plans(id),
    procedure_name  TEXT NOT NULL,
    phase_number    INTEGER NOT NULL DEFAULT 1 CHECK (phase_number > 0),
    sessions        INTEGER NOT NULL DEFAULT 1 CHECK (sessions > 0),
    fee             NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (fee >= 0),
    lab_cost        NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (lab_cost >= 0),
    materials_cost  NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (materials_cost >= 0),
    approved_at     TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
);

CREATE TABLE encounters (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    patient_id      BIGINT NOT NULL REFERENCES patients(id),
    appointment_id  BIGINT REFERENCES appointments(id),
    provider_id     BIGINT NOT NULL REFERENCES users(id),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','completed','amended'))
);

CREATE TABLE clinical_notes (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    encounter_id    BIGINT NOT NULL REFERENCES encounters(id),
    author_id       BIGINT NOT NULL REFERENCES users(id),
    subjective      TEXT NOT NULL DEFAULT '',
    objective       TEXT NOT NULL DEFAULT '',
    assessment      TEXT NOT NULL DEFAULT '',
    plan            TEXT NOT NULL DEFAULT '',
    signed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    signature_ref   TEXT NOT NULL,
    amendment_of    BIGINT REFERENCES clinical_notes(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE FUNCTION reject_clinical_note_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'Las notas clínicas firmadas son inmutables; registre una adenda.';
END;
$$;
CREATE TRIGGER clinical_notes_immutable
    BEFORE UPDATE OR DELETE ON clinical_notes
    FOR EACH ROW EXECUTE FUNCTION reject_clinical_note_mutation();

CREATE TABLE odontograms (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    encounter_id    BIGINT NOT NULL REFERENCES encounters(id),
    provider_id     BIGINT NOT NULL REFERENCES users(id),
    dentition       TEXT NOT NULL CHECK (dentition IN ('permanent','primary')),
    notation        TEXT NOT NULL CHECK (notation IN ('FDI','universal')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tooth_findings (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    odontogram_id   BIGINT NOT NULL REFERENCES odontograms(id),
    tooth_number    SMALLINT NOT NULL,
    surface         TEXT NOT NULL CHECK (surface IN ('mesial','distal','occlusal','incisal','vestibular','lingual','palatal','whole_tooth')),
    finding         TEXT NOT NULL CHECK (finding IN ('caries','restoration','endodontics','implant','fixed_prosthesis','removable_prosthesis','extraction','sealant','missing','other')),
    note            TEXT,
    UNIQUE (odontogram_id, tooth_number, surface, finding)
);

CREATE TABLE periodontal_readings (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    encounter_id        BIGINT NOT NULL REFERENCES encounters(id),
    tooth_number        SMALLINT NOT NULL,
    site                TEXT NOT NULL CHECK (site IN ('mesiobuccal','midbuccal','distobuccal','mesiolingual','midlingual','distolingual')),
    probing_depth_mm    NUMERIC(4,1) NOT NULL CHECK (probing_depth_mm BETWEEN 0 AND 15),
    gingival_margin_mm  NUMERIC(4,1),
    bleeding            BOOLEAN NOT NULL DEFAULT FALSE,
    mobility            SMALLINT CHECK (mobility IS NULL OR mobility BETWEEN 0 AND 3),
    furcation           SMALLINT CHECK (furcation IS NULL OR furcation BETWEEN 0 AND 3),
    recorded_by         BIGINT NOT NULL REFERENCES users(id),
    recorded_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (encounter_id, tooth_number, site)
);

CREATE TABLE prescriptions (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    encounter_id    BIGINT NOT NULL REFERENCES encounters(id),
    provider_id     BIGINT NOT NULL REFERENCES users(id),
    medication      TEXT NOT NULL,
    instructions    TEXT NOT NULL,
    duration        TEXT NOT NULL,
    signed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    signature_ref   TEXT NOT NULL
);

CREATE TABLE informed_consents (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    patient_id      BIGINT NOT NULL REFERENCES patients(id),
    encounter_id    BIGINT REFERENCES encounters(id),
    procedure_name  TEXT NOT NULL,
    document_ref    TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','signed','declined','expired')),
    signer_name     TEXT,
    signer_role     TEXT CHECK (signer_role IS NULL OR signer_role IN ('patient','guardian')),
    signature_ref   TEXT,
    otp_verified_at TIMESTAMPTZ,
    created_by      BIGINT NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    signed_at       TIMESTAMPTZ
);

CREATE TABLE invoices (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id       BIGINT NOT NULL REFERENCES clinics(id),
    patient_id      BIGINT NOT NULL REFERENCES patients(id),
    plan_id         BIGINT REFERENCES treatment_plans(id),
    number          TEXT NOT NULL,
    concept         TEXT NOT NULL DEFAULT 'Atención odontológica',
    total           NUMERIC(12,2) NOT NULL CHECK (total >= 0),
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','partially_paid','paid','void')),
    issued_by       BIGINT NOT NULL REFERENCES users(id),
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (clinic_id, number)
);

CREATE TABLE payments (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_id      BIGINT NOT NULL REFERENCES invoices(id),
    amount          NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    method          TEXT NOT NULL CHECK (method IN ('cash','card','transfer','insurance','other')),
    reference       TEXT,
    received_by     BIGINT NOT NULL REFERENCES users(id),
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_log (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clinic_id       BIGINT NOT NULL REFERENCES clinics(id),
    actor_id        BIGINT REFERENCES users(id),
    action          TEXT NOT NULL,
    entity_type     TEXT NOT NULL,
    entity_id       TEXT NOT NULL,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    source_ip       INET,
    details         JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX audit_log_clinic_time_idx ON audit_log(clinic_id, occurred_at DESC);
CREATE INDEX medical_alerts_patient_active_idx ON medical_alerts(patient_id) WHERE active;
