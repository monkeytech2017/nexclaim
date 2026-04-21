-- ============================================================
-- NexClaim Migration 003: Transaction & Log Tables
-- ============================================================

-- Claim Batch — กลุ่มการส่งแต่ละรอบ
CREATE TABLE claim_batch (
    batch_id        UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    hcode           CHAR(5)      NOT NULL REFERENCES m_hospital(hcode),
    period          CHAR(6)      NOT NULL,   -- YYYYMM
    inscl           VARCHAR(6)   NOT NULL REFERENCES m_inscl(inscl),
    agency_code     VARCHAR(10)  REFERENCES m_agency(agency_code),
    format          VARCHAR(10)  NOT NULL,   -- 16FILES/CIPN/CSOP/AIPN/SSOP
    sender          VARCHAR(10)  NOT NULL,   -- FDH/CHI/WCF/DOC
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    -- pending → validating → sending → sent → error | c_code
    total_records   INT          NOT NULL DEFAULT 0,
    valid_records   INT          NOT NULL DEFAULT 0,
    error_records   INT          NOT NULL DEFAULT 0,
    fdh_txn_id      VARCHAR(100),            -- txnId จาก FDH
    zip_filename    VARCHAR(200),
    zip_md5         CHAR(32),
    deadline_at     TIMESTAMPTZ,             -- 24 ชม. deadline (fast payment)
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    sent_at         TIMESTAMPTZ,
    ack_at          TIMESTAMPTZ,
    error_msg       TEXT
);

CREATE INDEX ON claim_batch (hcode, period, inscl);
CREATE INDEX ON claim_batch (status);
CREATE INDEX ON claim_batch (deadline_at) WHERE status = 'pending';
CREATE INDEX ON claim_batch (fdh_txn_id) WHERE fdh_txn_id IS NOT NULL;

-- Claim Record — แต่ละ visit/admit
CREATE TABLE claim_record (
    record_id       UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id        UUID         NOT NULL REFERENCES claim_batch(batch_id),
    hcode           CHAR(5)      NOT NULL,
    hn              VARCHAR(15)  NOT NULL,
    an              VARCHAR(9),              -- IPD เท่านั้น
    seq             VARCHAR(15),             -- OPD เท่านั้น
    is_ipd          BOOLEAN      NOT NULL DEFAULT false,
    service_date    DATE         NOT NULL,
    inscl           VARCHAR(6)   NOT NULL,
    pttype          VARCHAR(10),
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    total_charge    NUMERIC(10,2),
    paid_amount     NUMERIC(10,2),
    has_error       BOOLEAN      NOT NULL DEFAULT false,
    raw_json        JSONB,                   -- ข้อมูล raw ก่อน transform (เพื่อ re-process)
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX ON claim_record (batch_id);
CREATE INDEX ON claim_record (hn);
CREATE INDEX ON claim_record (an) WHERE an IS NOT NULL;
CREATE INDEX ON claim_record (seq) WHERE seq IS NOT NULL;
CREATE INDEX ON claim_record (has_error) WHERE has_error = true;

-- C-Code Log — ผลการตรวจสอบจาก REP
CREATE TABLE c_code_log (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id        UUID         NOT NULL REFERENCES claim_batch(batch_id),
    record_id       UUID         REFERENCES claim_record(record_id),
    hn              VARCHAR(15),
    an_or_seq       VARCHAR(15),
    c_code          VARCHAR(10)  NOT NULL,   -- C101, C115, ...
    c_desc          TEXT,
    field_name      VARCHAR(50),             -- field ที่มีปัญหา
    field_value     VARCHAR(200),            -- ค่าที่ผิด
    resolved        BOOLEAN      NOT NULL DEFAULT false,
    resolved_by     VARCHAR(100),
    resolved_at     TIMESTAMPTZ,
    received_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX ON c_code_log (batch_id, resolved);
CREATE INDEX ON c_code_log (c_code);
CREATE INDEX ON c_code_log (resolved) WHERE resolved = false;

-- Send Log — log การส่งทุกครั้ง (รวม retry)
CREATE TABLE send_log (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id        UUID         NOT NULL REFERENCES claim_batch(batch_id),
    attempt_no      SMALLINT     NOT NULL DEFAULT 1,
    endpoint        TEXT         NOT NULL,
    http_status     INT,
    fdh_txn_id      VARCHAR(100),
    response_body   TEXT,                    -- ตัด PII ออกก่อน log
    duration_ms     INT,
    success         BOOLEAN,
    error_msg       TEXT,
    sent_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX ON send_log (batch_id);
