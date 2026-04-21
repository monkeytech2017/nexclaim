// Shared TypeScript types — mirror จาก Go model

export type INSCL =
  | '011' | 'WEL' | 'LGO' | 'OFC'
  | 'UCS' | 'NON' | 'WP1' | 'WP2'
  | 'SSS' | 'SS4'
  | 'TPBS' | 'WK' | 'MON' | 'PRS'

export type ClaimFormat = '16FILES' | 'CIPN' | 'CSOP' | 'AIPN' | 'SSOP'
export type Sender = 'FDH' | 'CHI' | 'WCF' | 'DOC'
export type BatchStatus = 'pending' | 'validating' | 'sending' | 'sent' | 'error' | 'c_code'

export const INSCL_LABELS: Record<INSCL, string> = {
  '011': 'ข้าราชการพลเรือน',
  'WEL': 'ลูกจ้างประจำ/บำนาญ',
  'LGO': 'อปท.',
  'OFC': 'หน่วยงานอิสระ',
  'UCS': 'บัตรทอง',
  'NON': 'ไร้สัญชาติ',
  'WP1': 'แรงงานต่างด้าว MOU',
  'WP2': 'แรงงานต่างด้าวขึ้นทะเบียน',
  'SSS': 'ประกันสังคม ม.33/39',
  'SS4': 'ประกันสังคม ม.40',
  'TPBS': 'พ.ร.บ.รถ',
  'WK':  'กองทุนทดแทน',
  'MON': 'พระภิกษุ',
  'PRS': 'ราชทัณฑ์',
}
