-- ============================================================
-- NexClaim Migration 002: HIS Mapping Tables
-- mapping HIS column → field ตามสเปคของแต่ละหน่วยงาน
-- ============================================================

-- HIS Field Mapping — config ต่อ รพ. ต่อ target format
CREATE TABLE his_field_map (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    hcode           CHAR(5)      NOT NULL REFERENCES m_hospital(hcode),
    his_table       VARCHAR(100) NOT NULL,   -- ชื่อตาราง HIS เช่น opd_visit
    his_column      VARCHAR(100) NOT NULL,   -- ชื่อ column ใน HIS
    target_file     VARCHAR(20)  NOT NULL,   -- OPD, IPD, ODX, IDX, DRU, CHA ...
    target_field    VARCHAR(50)  NOT NULL,   -- field spec: DATEOPD, DIAG, ...
    transform       VARCHAR(50)  DEFAULT 'none', -- to_ad, format_hhmm, map_typeout, validate_icd10, ...
    is_required     BOOLEAN      NOT NULL DEFAULT false,
    default_value   VARCHAR(200),            -- ถ้าไม่มีใน HIS ใช้ค่านี้ เช่น UUC="1"
    note            TEXT,
    UNIQUE (hcode, his_table, his_column, target_file, target_field)
);

-- HIS INSCL Mapping — รหัสสิทธิใน HIS → INSCL มาตรฐาน
CREATE TABLE his_inscl_map (
    hcode           CHAR(5)      NOT NULL REFERENCES m_hospital(hcode),
    his_pttype      VARCHAR(20)  NOT NULL,  -- รหัสที่ HIS ใช้ เช่น "A01", "GOV"
    inscl           VARCHAR(6)   NOT NULL REFERENCES m_inscl(inscl),
    agency_code     VARCHAR(10)  REFERENCES m_agency(agency_code), -- สำหรับ OFC
    note            TEXT,
    PRIMARY KEY (hcode, his_pttype)
);

-- HIS Drug Mapping — รหัสยาใน HIS → TMT code
CREATE TABLE his_drug_map (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    hcode           CHAR(5)      NOT NULL REFERENCES m_hospital(hcode),
    his_drug_code   VARCHAR(50)  NOT NULL,  -- รหัสยาใน HIS
    tmt_code        CHAR(24)     REFERENCES m_tmt_drug(tmt_code),
    his_drug_name   VARCHAR(200),
    note            TEXT,
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    UNIQUE (hcode, his_drug_code)
);

-- HIS Doctor Mapping — รหัสแพทย์ใน HIS → license_no
CREATE TABLE his_doctor_map (
    hcode           CHAR(5)      NOT NULL REFERENCES m_hospital(hcode),
    his_doctor_code VARCHAR(50)  NOT NULL,  -- รหัสแพทย์ใน HIS
    doctor_id       VARCHAR(20)  REFERENCES m_doctor(doctor_id),
    PRIMARY KEY (hcode, his_doctor_code)
);

-- HIS ICD Mapping — รหัส ICD ใน HIS (ถ้าต่างจาก WHO standard)
CREATE TABLE his_icd_map (
    hcode           CHAR(5)      NOT NULL REFERENCES m_hospital(hcode),
    his_icd_code    VARCHAR(20)  NOT NULL,
    icd_type        CHAR(2)      NOT NULL CHECK (icd_type IN ('10', '9C')), -- ICD-10 หรือ ICD-9CM
    std_code        VARCHAR(7)   NOT NULL,  -- รหัสมาตรฐาน
    PRIMARY KEY (hcode, his_icd_code, icd_type)
);
