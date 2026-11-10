import { useRef, useState, type FormEvent } from 'react'
import {
  createUserWithEmailAndPassword, GoogleAuthProvider, RecaptchaVerifier, signInWithEmailAndPassword,
  signInWithPhoneNumber, signInWithPopup, type ConfirmationResult,
} from 'firebase/auth'
import { Icon } from '../components/Icon'
import { Task } from '../components/Task'
import { GoogleMark, StatePanel } from '../components/ui'
import { playPrompt } from '../lib/audio'
import { Link, navigate, useLocation } from '../lib/router'
import { errorText } from '../lib/errors'
import { auth } from '../lib/firebase'
import { useLang, useT } from '../lib/i18n'
import { FAKE_AUTH, safeNext, signIn } from '../lib/session'

/** Runs a Firebase sign-in step; mock and dev modes skip it and go straight to the API session. */
const firebase = async (step: () => Promise<unknown>) => { if (!FAKE_AUTH) await step() }
/** Keeps the failure for the screen (shown as a sentence by errorText) and logs the detail for developers. */
function failure(e: unknown): unknown {
  console.error('sign-in failed', e)
  return e
}
const google = () => firebase(() => signInWithPopup(auth(), new GoogleAuthProvider()))

export function RolePick() {
  const t = useT()
  const ur = useLang() === 'ur'
  const big = ur ? { fontFamily: 'var(--font-urdu)', lineHeight: 1.9 } : undefined
  const flip = { display: 'flex', transform: ur ? 'scaleX(-1)' : undefined } // the chevron points the reading way
  return (
    <Task screen="01-role" wide>
      <div className="stack-lg" style={{ paddingTop: 32 }}>
        <Link className="role-tile worker-role" href="/worker/login">
          <div><Icon name="Wrench" className="icon-lg" /></div>
          <h1 style={big}>{t('Worker', 'کاریگر')}</h1>
          <span style={flip}><Icon name="ChevronRight" /></span>
        </Link>
        <Link className="role-tile" href="/login">
          <div><Icon name="House" className="icon-lg" /></div>
          <h1 style={big}>{t('Customer', 'گاہک')}</h1>
          <span style={flip}><Icon name="ChevronRight" /></span>
        </Link>
      </div>
    </Task>
  )
}

/** Phone number, then the SMS code. Both roles use it; `ur` gives the worker's larger layout and plays the error prompt. */
function PhoneForm({ ur = false, onDone }: { ur?: boolean; onDone: () => Promise<void> }) {
  const t = useT()
  const rtl = useLang() === 'ur'
  const [step, setStep] = useState<'phone' | 'code'>('phone')
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle')
  const [why, setWhy] = useState<unknown>(null)
  const sent = useRef<ConfirmationResult | null>(null)
  const captcha = useRef<RecaptchaVerifier | null>(null)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setState('loading')
    try {
      if (step === 'phone') {
        await firebase(async () => {
          captcha.current ??= new RecaptchaVerifier(auth(), 'recaptcha', { size: 'invisible' })
          sent.current = await signInWithPhoneNumber(auth(), '+92' + phone.replace(/^0/, ''), captcha.current)
        })
        setStep('code')
      } else {
        await firebase(() => sent.current!.confirm(code))
        await onDone()
      }
      setState('idle')
    } catch (e) {
      setWhy(failure(e))
      setState('error')
      // A used reCAPTCHA cannot verify again: start a fresh one for the next try.
      captcha.current?.clear()
      captcha.current = null
      if (ur) playPrompt('error').catch(() => undefined)
    }
  }
  const digits = (set: (v: string) => void, max: number) => (e: { target: { value: string } }) => set(e.target.value.replace(/\D/g, '').slice(0, max))
  const busy = state === 'loading'
  const title = step === 'phone' ? t('Phone number', 'اپنا فون نمبر لکھیں') : t('Code from the SMS', 'میسج میں آیا کوڈ لکھیں')
  const failed = errorText(why, t)
  const big = { fontSize: 24, minWidth: 0 }

  return (
    <form className="stack" onSubmit={submit}>
      {ur ? <div className="section-heading"><h1>{title}</h1></div> : <span className="field">{title}</span>}
      {state === 'error' && (ur ? <StatePanel kind="error" title={failed} /> : <span className="field-error" role="alert">{failed}</span>)}
      {step === 'phone' ? (
        <div className="row" dir="ltr" lang="en">
          <span className="digits" style={big}>+92</span>
          <input className="grow digits" style={big} type="tel" inputMode="numeric" autoComplete="tel-national" aria-label={title}
            placeholder="3001234567" value={phone} onChange={digits(setPhone, 11)} />
        </div>
      ) : (
        <input className="digits" style={big} dir="ltr" type="tel" inputMode="numeric" autoComplete="one-time-code" aria-label={title}
          placeholder="123456" value={code} onChange={digits(setCode, 6)} autoFocus />
      )}
      <button className="button full" type="submit" disabled={busy || (step === 'phone' ? phone.length < 10 : code.length < 6)}>
        <Icon name={busy ? 'LoaderCircle' : step === 'phone' ? (rtl ? 'ArrowLeft' : 'ArrowRight') : 'Check'} />
        {step === 'phone' ? t('Send code', 'آگے') : t('Verify and sign in', 'ٹھیک ہے')}
      </button>
      <div id="recaptcha" />
    </form>
  )
}

export function WorkerSignIn() {
  const t = useT()
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle')
  const [why, setWhy] = useState<unknown>(null)
  async function finish() {
    const me = await signIn('worker')
    navigate(me.worker ? '/app' : '/worker/onboard')
  }
  async function withGoogle() {
    setState('loading')
    try {
      await google()
      await finish()
    } catch (e) {
      setWhy(failure(e))
      setState('error')
      playPrompt('error').catch(() => undefined)
    }
  }
  return (
    <Task screen="02-worker-signin" worker prompt="signin">
      <PhoneForm ur onDone={finish} />
      {state === 'error' && <><StatePanel kind="error" title={errorText(why, t)} /></>}
      <button className="button outline full" type="button" onClick={withGoogle} disabled={state === 'loading'}>
        <GoogleMark /><span lang="en" dir="ltr">Continue with Google</span>
      </button>
    </Task>
  )
}

export function CustomerSignIn({ mode }: { mode: 'signin' | 'signup' }) {
  const t = useT()
  const { query } = useLocation()
  const next = safeNext(query.get('next'))
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<{ email?: string; password?: string }>({})
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle')
  const [why, setWhy] = useState<unknown>(null)
  const signup = mode === 'signup'

  const validate = () => {
    const e: typeof errors = {}
    if (!/^\S+@\S+\.\S+$/.test(email)) e.email = t('Enter a valid email address.', 'صحیح ای میل لکھیں۔')
    if (password.length < 6) e.password = t('Use at least 6 characters.', 'کم از کم 6 حروف لکھیں۔')
    setErrors(e)
    return Object.keys(e).length === 0
  }
  async function go(e?: FormEvent) {
    e?.preventDefault()
    if (e && !validate()) return
    setState('loading')
    try {
      if (e) await firebase(() => (signup ? createUserWithEmailAndPassword : signInWithEmailAndPassword)(auth(), email, password))
      else await google()
      await signIn('customer')
      navigate(next)
    } catch (e) {
      setWhy(failure(e))
      setState('error')
    }
  }

  if (state === 'error') return (
    <Task screen="03-customer-signin">
      <StatePanel kind="error" title={t('We couldn’t sign you in', 'سائن اِن نہیں ہو سکا')}
        text={errorText(why, t)} />
      <button className="button full" type="button" onClick={() => setState('idle')}><Icon name="RefreshCw" />{t('Try again', 'دوبارہ کوشش کریں')}</button>
    </Task>
  )

  return (
    <Task screen={signup ? '04-customer-signup' : '03-customer-signin'}>
      <div className="section-heading">
        <h1>{signup ? t('Create your account', 'اپنا اکاؤنٹ بنائیں') : t('Sign in to find a worker', 'کاریگر ڈھونڈنے کے لیے سائن اِن کریں')}</h1>
        <p className="muted">{signup
          ? t('You’ll use it to book workers and review their work.', 'اسی سے آپ کاریگر بک کریں گے اور ان کے کام کا ریویو دیں گے۔')
          : t('Electricians, plumbers, AC technicians, carpenters and mechanics near you.', 'آپ کے قریب الیکٹریشن، پلمبر، اے سی ٹیکنیشن، کارپینٹر اور مکینک۔')}</p>
      </div>
      <PhoneForm onDone={async () => { await signIn('customer'); navigate(next) }} />
      <div className="or">{t('or', 'یا')}</div>
      <button className="button outline full" type="button" onClick={() => go()} disabled={state === 'loading'}>
        <GoogleMark /><span lang="en" dir="ltr">Continue with Google</span>
      </button>
      <div className="or">{t('or continue with email', 'یا ای میل سے جاری رکھیں')}</div>
      <form className="stack" onSubmit={go} noValidate>
        <label className="field" htmlFor="email">{t('Email', 'ای میل')}
          <input id="email" name="email" type="email" autoComplete="email" value={email} placeholder={t('Email', 'ای میل')}
            aria-invalid={!!errors.email} aria-describedby={errors.email ? 'email-error' : undefined}
            onChange={(e) => setEmail(e.target.value)} onBlur={() => email && validate()} />
          {errors.email && <span className="field-error" id="email-error">{errors.email}</span>}
        </label>
        <label className="field" htmlFor="password">{t('Password', 'پاس ورڈ')}
          <input id="password" name="password" type="password" autoComplete={signup ? 'new-password' : 'current-password'} value={password}
            placeholder={t('Password', 'پاس ورڈ')} aria-invalid={!!errors.password} aria-describedby={errors.password ? 'password-error' : undefined}
            onChange={(e) => setPassword(e.target.value)} />
          {errors.password && <span className="field-error" id="password-error">{errors.password}</span>}
        </label>
        <button className="button full" type="submit" disabled={state === 'loading'}>
          {state === 'loading' && <Icon name="LoaderCircle" />}{signup ? t('Create account', 'اکاؤنٹ بنائیں') : t('Sign in', 'سائن اِن')}
        </button>
      </form>
      <Link className="button link" href={(signup ? '/login' : '/signup') + `?next=${encodeURIComponent(next)}`}>
        {signup ? t('Already have an account? Sign in', 'پہلے سے اکاؤنٹ ہے؟ سائن اِن کریں') : t('New here? Create an account', 'نئے ہیں؟ اکاؤنٹ بنائیں')}
      </Link>
    </Task>
  )
}

