-- ============================================================
-- NexClaim Migration 001: Master Data Tables
-- ลำดับ: สร้าง master ก่อนเสมอ (ไม่มี FK ขึ้นกัน)
-- ============================================================

-- 1. รหัสสิทธิการรักษา (จาก สปสช./CGD/สปส.)
CREATE TABLE m_inscl (
    inscl           VARCHAR(6)   PRIMARY KEY,
    name_th         VARCHAR(100) NOT NULL,
    fund_type       VARCHAR(20)  NOT NULL, -- CSMBS, UC, SSO, TPBS, WK, OTHER
    format_ipd      VARCHAR(10)  NOT NULL, -- CIPN, AIPN, 16FILES
    format_opd      VARCHAR(10)  NOT NULL, -- CSOP, SSOP, 16FILES
    sender          VARCHAR(10)  NOT NULL, -- FDH, CHI, WCF, DOC
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

INSERT INTO m_inscl VALUES
    ('011',  'ข้าราชการพลเรือน/ทหาร/ตำรวจ', 'CSMBS', 'CIPN',    'CSOP',    'FDH', true, now()),
    ('WEL',  'ลูกจ้างประจำ/บำนาญ',           'CSMBS', 'CIPN',    'CSOP',    'FDH', true, now()),
    ('LGO',  'อปท./กทม./พัทยา',              'CSMBS', 'CIPN',    'CSOP',    'FDH', true, now()),
    ('OFC',  'หน่วยงานอิสระ (Agency แยก)',   'CSMBS', 'CIPN',    'CSOP',    'FDH', true, now()),
    ('UCS',  'บัตรทอง',                      'UC',    '16FILES', '16FILES', 'FDH', true, now()),
    ('NON',  'ไร้สัญชาติ/ปัญหาสถานะ',        'UC',    '16FILES', '16FILES', 'FDH', true, now()),
    ('WP1',  'แรงงานต่างด้าว MOU',            'UC',    '16FILES', '16FILES', 'FDH', true, now()),
    ('WP2',  'แรงงานต่างด้าวขึ้นทะเบียน',    'UC',    '16FILES', '16FILES', 'FDH', true, now()),
    ('SSS',  'ประกันสังคม ม.33/39',           'SSO',   'AIPN',    'SSOP',    'CHI', true, now()),
    ('SS4',  'ประกันสังคม ม.40',              'SSO',   'AIPN',    'SSOP',    'CHI', true, now()),
    ('TPBS', 'พ.ร.บ.ผู้ประสบภัยรถ',           'TPBS',  '16FILES', '16FILES', 'FDH', true, now()),
    ('WK',   'กองทุนทดแทน',                  'WK',    '16FILES', '16FILES', 'WCF', true, now()),
    ('MON',  'พระภิกษุสงฆ์',                 'UC',    '16FILES', '16FILES', 'FDH', true, now()),
    ('PRS',  'ผู้ต้องขัง (ราชทัณฑ์)',         'OTHER', '16FILES', '16FILES', 'DOC', true, now());

-- 2. หน่วยงานอิสระ (สำหรับ OFC — Agency ใน XML Header)
CREATE TABLE m_agency (
    agency_code     VARCHAR(10)  PRIMARY KEY,
    name_th         VARCHAR(100) NOT NULL,
    fdh_endpoint    TEXT,
    contact_email   VARCHAR(200),
    is_active       BOOLEAN      NOT NULL DEFAULT true
);

INSERT INTO m_agency VALUES
    ('CGD',  'กรมบัญชีกลาง',         '/api/claim/cipn',             NULL, true),
    ('LGO',  'อปท. (รวม)',            '/api/claim/cipn',             NULL, true),
    ('NBTC', 'กสทช.',                 '/api/claim/cipn?agency=NBTC', NULL, true),
    ('BAAC', 'กฟก. (ธ.ก.ส.)',         '/api/claim/cipn?agency=BAAC', NULL, true),
    ('ECT',  'กกต.',                  '/api/claim/cipn?agency=ECT',  NULL, true),
    ('PEA',  'สผผ. (กฟภ.)',           '/api/claim/cipn?agency=PEA',  NULL, true),
    ('MEA',  'กฟน.',                  '/api/claim/cipn?agency=MEA',  NULL, true),
    ('MWA',  'ประปานครหลวง',          '/api/claim/cipn?agency=MWA',  NULL, true),
    ('SRT',  'การรถไฟแห่งประเทศไทย', '/api/claim/cipn?agency=SRT',  NULL, true);

-- 3. หมวดค่าบริการ 01–16 (ประกาศกระทรวงการคลัง — บังคับ CSMBS/LGO CHA)
CREATE TABLE m_chrgitem (
    code        CHAR(2)      PRIMARY KEY,
    name_th     VARCHAR(200) NOT NULL,
    applies_to  VARCHAR(100) DEFAULT 'CSMBS,LGO,OFC'
);

INSERT INTO m_chrgitem VALUES
    ('01', 'ค่าห้องและค่าอาหาร',                              'CSMBS,LGO,OFC'),
    ('02', 'ค่าอวัยวะเทียมและอุปกรณ์ในการบำบัดรักษาโรค',    'CSMBS,LGO,OFC'),
    ('03', 'ค่ายาและสารอาหารทางเส้นเลือด',                    'CSMBS,LGO,OFC'),
    ('04', 'ค่าเลือดและส่วนประกอบของเลือด',                   'CSMBS,LGO,OFC'),
    ('05', 'ค่าตรวจวินิจฉัยทางเทคนิคการแพทย์และพยาธิวิทยา', 'CSMBS,LGO,OFC'),
    ('06', 'ค่าตรวจวินิจฉัยและรักษาทางรังสีวิทยา',           'CSMBS,LGO,OFC'),
    ('07', 'ค่าตรวจวินิจฉัยโดยวิธีพิเศษอื่นๆ',               'CSMBS,LGO,OFC'),
    ('08', 'ค่าบริการโลหิตและส่วนประกอบของโลหิต',            'CSMBS,LGO,OFC'),
    ('09', 'ค่าบริการทางการพยาบาล',                           'CSMBS,LGO,OFC'),
    ('10', 'ค่าบริการทางกายภาพบำบัดและเวชกรรมฟื้นฟู',       'CSMBS,LGO,OFC'),
    ('11', 'ค่าบริการทางทันตกรรม',                            'CSMBS,LGO,OFC'),
    ('12', 'ค่าบริการทางแพทย์แผนไทย',                        'CSMBS,LGO,OFC'),
    ('13', 'ค่าบริการสาธารณสุขอื่น',                          'CSMBS,LGO,OFC'),
    ('14', 'ค่าบริการวิชาชีพเวชกรรม',                        'CSMBS,LGO,OFC'),
    ('15', 'ค่าบริการฝังเข็มและบำบัดด้วยผู้ประกอบโรคศิลปะ', 'CSMBS,LGO,OFC'),
    ('16', 'ค่าบริการอื่นๆ ที่ไม่เกี่ยวกับการรักษาพยาบาล',  'CSMBS,LGO,OFC');

-- 4. โรงพยาบาล
CREATE TABLE m_hospital (
    hcode           CHAR(5)      PRIMARY KEY,
    name_th         VARCHAR(200) NOT NULL,
    changwat        CHAR(2),
    amphur          CHAR(4),
    his_db_key      VARCHAR(50),  -- key สำหรับ connect HIS ของ รพ. นี้
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- 5. แพทย์ผู้รักษา
CREATE TABLE m_doctor (
    doctor_id       VARCHAR(20)  PRIMARY KEY,  -- internal ID ของ รพ.
    hcode           CHAR(5)      REFERENCES m_hospital(hcode),
    license_no      VARCHAR(10)  NOT NULL,     -- เลขใบประกอบวิชาชีพ 6 หลัก (DRDX/DROPID)
    name_th         VARCHAR(200),
    specialty       VARCHAR(100),
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ON m_doctor (hcode, license_no);

-- 6. ICD-10 (โหลดจาก data/icd10.json)
CREATE TABLE m_icd10 (
    code        VARCHAR(7)   PRIMARY KEY,
    name_th     TEXT,
    name_en     TEXT,
    chapter     VARCHAR(5),
    is_active   BOOLEAN      NOT NULL DEFAULT true
);

-- 7. ICD-9CM (โหลดจาก data/icd9cm.json)
CREATE TABLE m_icd9cm (
    code        VARCHAR(7)   PRIMARY KEY,
    name_th     TEXT,
    name_en     TEXT,
    is_active   BOOLEAN      NOT NULL DEFAULT true
);

-- 8. TMT Drug (โหลดจาก data/tmt.json)
CREATE TABLE m_tmt_drug (
    tmt_code        CHAR(24)     PRIMARY KEY,
    name_th         TEXT,
    generic_name    TEXT,
    strength        VARCHAR(100),
    dosage_form     VARCHAR(100),
    unit            VARCHAR(20),
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
