// Turns a sign-in or API failure into a sentence a person can act on, in English or Urdu.
import { toast } from './audio'

type T = (en: string, ur: string) => string

const RETRY: [string, string] = ['Something went wrong. Please try again.', 'کچھ غلط ہو گیا۔ دوبارہ کوشش کریں۔']
const OFFLINE: [string, string] = ['No internet connection. Check it and try again.', 'انٹرنیٹ نہیں چل رہا۔ کنکشن دیکھ کر دوبارہ کوشش کریں۔']
const BAD_LOGIN: [string, string] = ['The email or password is wrong.', 'ای میل یا پاس ورڈ غلط ہے۔']
const BAD_PHONE: [string, string] = ['This phone number is not valid. Check it and type it again.', 'یہ فون نمبر درست نہیں۔ دیکھ کر دوبارہ لکھیں۔']
const POPUP: [string, string] = ['The Google window was closed before sign-in finished. Try again.', 'گوگل کی ونڈو سائن اِن سے پہلے بند ہو گئی۔ دوبارہ کوشش کریں۔']
const NOT_ALLOWED: [string, string] = ['You are not allowed to do this.', 'آپ یہ نہیں کر سکتے۔']
const MOVED_ON: [string, string] = ['This has already changed. Reload the page to see the latest.', 'یہ پہلے ہی بدل چکا ہے۔ تازہ حالت دیکھنے کے لیے صفحہ دوبارہ کھولیں۔']

const MESSAGES: Record<string, [string, string]> = {
  // Firebase sign-in
  'auth/invalid-phone-number': BAD_PHONE,
  'auth/missing-phone-number': BAD_PHONE,
  'auth/invalid-verification-code': ['The code is wrong. Check the SMS and type it again.', 'کوڈ غلط ہے۔ میسج دیکھ کر دوبارہ لکھیں۔'],
  'auth/missing-verification-code': ['Type the 6-digit code from the SMS.', 'میسج میں آیا 6 ہندسوں کا کوڈ لکھیں۔'],
  'auth/code-expired': ['The code has expired. Go back and ask for a new one.', 'کوڈ کا وقت ختم ہو گیا۔ واپس جا کر نیا کوڈ منگوائیں۔'],
  'auth/too-many-requests': ['Too many tries. Wait a few minutes, then try again.', 'بہت زیادہ کوششیں ہو گئیں۔ چند منٹ رک کر دوبارہ کوشش کریں۔'],
  'auth/quota-exceeded': ['No more codes can be sent today. Use another way to sign in.', 'آج مزید کوڈ نہیں بھیجے جا سکتے۔ سائن اِن کا دوسرا طریقہ آزمائیں۔'],
  'auth/operation-not-allowed': ['This way of signing in is not available right now. Try another way.', 'سائن اِن کا یہ طریقہ ابھی دستیاب نہیں۔ دوسرا طریقہ آزمائیں۔'],
  'auth/captcha-check-failed': ['This device could not be checked. Reload the page and try again.', 'اس ڈیوائس کی تصدیق نہیں ہو سکی۔ صفحہ دوبارہ کھول کر کوشش کریں۔'],
  'auth/invalid-app-credential': ['This device could not be checked. Reload the page and try again.', 'اس ڈیوائس کی تصدیق نہیں ہو سکی۔ صفحہ دوبارہ کھول کر کوشش کریں۔'],
  'auth/popup-closed-by-user': POPUP,
  'auth/cancelled-popup-request': POPUP,
  'auth/popup-blocked': ['Your browser blocked the Google window. Allow pop-ups for this site and try again.', 'براؤزر نے گوگل کی ونڈو روک دی۔ اس سائٹ کے لیے پاپ اَپ کی اجازت دے کر دوبارہ کوشش کریں۔'],
  'auth/invalid-credential': BAD_LOGIN,
  'auth/wrong-password': BAD_LOGIN,
  'auth/user-not-found': BAD_LOGIN,
  'auth/invalid-email': ['This email address is not valid.', 'یہ ای میل درست نہیں۔'],
  'auth/email-already-in-use': ['An account with this email already exists. Sign in instead.', 'اس ای میل سے اکاؤنٹ پہلے سے موجود ہے۔ سائن اِن کریں۔'],
  'auth/weak-password': ['Use a password of at least 6 characters.', 'کم از کم 6 حروف کا پاس ورڈ رکھیں۔'],
  'auth/network-request-failed': OFFLINE,
  // API
  rate_limited: ['Too many requests. Wait a little, then try again.', 'بہت زیادہ درخواستیں ہو گئیں۔ تھوڑا رک کر دوبارہ کوشش کریں۔'],
  worker_busy: ['This worker has just taken another job. Choose another worker.', 'اس کاریگر نے ابھی دوسرا کام لے لیا ہے۔ کوئی اور کاریگر چنیں۔'],
  unauthorized: ['Please sign in again.', 'دوبارہ سائن اِن کریں۔'],
  no_session: ['Please sign in again.', 'دوبارہ سائن اِن کریں۔'],
  forbidden: NOT_ALLOWED,
  worker_only: NOT_ALLOWED,
  own_offer: ['You cannot answer your own offer. Wait for the other person.', 'آپ اپنی ہی پیشکش کا جواب نہیں دے سکتے۔ دوسرے کے جواب کا انتظار کریں۔'],
  not_found: ['This could not be found.', 'یہ نہیں ملا۔'],
  offer_not_pending: MOVED_ON,
  invalid_status: MOVED_ON,
  job_closed: ['This job is closed.', 'یہ کام بند ہو چکا ہے۔'],
  job_not_completed: ['You can review the job once it is complete.', 'کام مکمل ہونے کے بعد ریویو دیا جا سکتا ہے۔'],
  already_reviewed: ['You have already reviewed this job.', 'آپ اس کام کا ریویو دے چکے ہیں۔'],
  bad_request: ['Some information is missing or not valid. Check it and try again.', 'کچھ معلومات کم ہیں یا درست نہیں۔ دیکھ کر دوبارہ کوشش کریں۔'],
}

/** A sentence for the person, never a code. `t` is the screen's translate function. */
export function errorText(e: unknown, t: T): string {
  const err = e as { code?: string; status?: number }
  const known = err?.code ? MESSAGES[err.code] : undefined
  if (known) return t(...known)
  if (!navigator.onLine || e instanceof TypeError) return t(...OFFLINE) // fetch throws TypeError when it cannot reach the server
  if ((err?.status ?? 0) >= 500) return t('Our server had a problem. Try again in a moment.', 'سرور میں مسئلہ آ گیا۔ تھوڑی دیر بعد دوبارہ کوشش کریں۔')
  return t(...RETRY)
}

// api.ts announces failed actions (not background refreshes) with this event; the reason shows as a short message.
// The page root carries the current language, so no React hook is needed here.
window.addEventListener('rr:error', (e) => {
  toast(errorText((e as CustomEvent).detail, (en, ur) => (document.documentElement.lang === 'ur' ? ur : en)))
})
