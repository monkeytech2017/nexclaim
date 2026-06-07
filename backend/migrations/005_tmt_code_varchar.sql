-- ============================================================
-- NexClaim Migration 005: tmt_code CHAR(24) → VARCHAR(24)
--
-- เหตุผล: เดิม m_tmt_drug.tmt_code และ his_drug_map.tmt_code
-- ประกาศเป็น CHAR(24) จากสมมติฐานผิดว่า TMT code ยาว 24 หลัก
-- แต่ค่าจริงคือ TMTID ของ TMT (https://tmt.this.or.th) ซึ่งยาว
-- เพียง 6–7 หลัก (เช่น 100005, 1314446)
--
-- ปัญหาของ CHAR(24): Postgres pad ช่องว่างต่อท้ายให้ครบ 24 ตัวอักษร
-- ทำให้ exact-match ใน validator พัง (validator โหลด
-- SELECT tmt_code ... แล้วเทียบกับรหัส 6–7 หลักที่ HIS ส่งมา ไม่ตรง)
-- และ space ต่อท้ายจะรั่วเข้าไปใน claim XML ที่ generate ออกมา
-- 34,221 แถวที่เพิ่ง bulk-load เข้า m_tmt_drug ตอนนี้ถูก pad อยู่
--
-- USING rtrim(tmt_code) จึงจำเป็น เพื่อตัด space ต่อท้ายของแถวเดิม
-- ทั้งหมดให้เหลือ TMTID จริง ระหว่างเปลี่ยน type เป็น VARCHAR
-- ============================================================

BEGIN;

-- 1. ตัด FK เดิม (default name = his_drug_map_tmt_code_fkey) ก่อนเปลี่ยน type
ALTER TABLE his_drug_map
    DROP CONSTRAINT IF EXISTS his_drug_map_tmt_code_fkey;

-- 2. m_tmt_drug.tmt_code: CHAR(24) → VARCHAR(24) + rtrim แถวที่ถูก pad
--    (PRIMARY KEY ยังคงอยู่ — ALTER TYPE ไม่ทำลาย PK)
ALTER TABLE m_tmt_drug
    ALTER COLUMN tmt_code TYPE VARCHAR(24) USING rtrim(tmt_code);

-- 3. his_drug_map.tmt_code: CHAR(24) → VARCHAR(24) + rtrim ให้ตรงกับ PK ใหม่
ALTER TABLE his_drug_map
    ALTER COLUMN tmt_code TYPE VARCHAR(24) USING rtrim(tmt_code);

-- 4. ผูก FK กลับด้วยชื่อ default เดิม (idempotent-friendly: drop ใน step 1 แล้ว)
ALTER TABLE his_drug_map
    ADD CONSTRAINT his_drug_map_tmt_code_fkey
        FOREIGN KEY (tmt_code) REFERENCES m_tmt_drug(tmt_code);

COMMIT;
