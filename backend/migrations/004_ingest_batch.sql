-- ============================================================
-- NexClaim Migration 004: OPD Ingest Batch + Visit
--
-- แยกจาก claim_batch (003) — claim_batch = output ที่ส่งให้ FDH/CHI แล้ว;
-- opd_ingest_batch = staging ที่ HIS push เข้ามา ก่อน NexClaim fetch
-- detail + run pipeline.
-- ============================================================

CREATE TABLE opd_ingest_batch (
    batch_id        VARCHAR(50)  PRIMARY KEY,       -- "BATCH-YYYYMMDD-xxxxxxxx"
    hcode           CHAR(5)      NOT NULL,
    period          CHAR(6)      NOT NULL,
    exported_by     VARCHAR(200),
    state           VARCHAR(20)  NOT NULL DEFAULT 'RECEIVED',
    -- RECEIVED → FETCHING → COMPLETED | FAILED
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    last_error      TEXT
);

CREATE INDEX ON opd_ingest_batch (hcode, period);
CREATE INDEX ON opd_ingest_batch (state);
CREATE INDEX ON opd_ingest_batch (created_at DESC);

CREATE TABLE opd_ingest_visit (
    batch_id        VARCHAR(50)  NOT NULL REFERENCES opd_ingest_batch(batch_id) ON DELETE CASCADE,
    vn              VARCHAR(20)  NOT NULL,
    hn              VARCHAR(15),
    pid             VARCHAR(13),
    patient_name    VARCHAR(200),
    visit_date      CHAR(8),                        -- YYYYMMDD ค.ศ.
    visit_time      VARCHAR(4),
    inscl           VARCHAR(6),
    inscl_name      VARCHAR(100),
    clinic_code     VARCHAR(10),
    clinic_name     VARCHAR(100),
    doctor_code     VARCHAR(20),
    total_charge    NUMERIC(12,2),
    ordinal         INT          NOT NULL,          -- ลำดับใน batch (preserve input order)
    PRIMARY KEY (batch_id, vn)
);

CREATE INDEX ON opd_ingest_visit (batch_id, ordinal);
CREATE INDEX ON opd_ingest_visit (inscl);
