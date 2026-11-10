import { useRef, useState, type FormEvent } from 'react'
import {
  createUserWithEmailAndPassword, GoogleAuthProvider, RecaptchaVerifier, signInWithEmailAndPassword,
  signInWithPhoneNumber, signInWithPopup, type ConfirmationResult,
} from 'firebase/auth'
import { Icon } from '../components/Icon'
import { Task } from '../components/Task'
import { GoogleMark, PromptBar, StatePanel } from '../components/ui'
import type { User } from '../contract'
import type { Me } from '../lib/api'
import { playPrompt } from '../lib/audio'
import { Link, navigate, useLocation } from '../lib/router'
import { errorText } from '../lib/errors'
import { auth } from '../lib/firebase'
import { useLang, useT } from '../lib/i18n'
import { FAKE_AUTH, safeNext, setMe, signIn, signOut } from '../lib/session'

/** Runs a Firebase sign-in step; mock and dev modes skip it and go straight to the API session. */
const firebase = async (step: () => Promise<unknown>) => { if (!FAKE_AUTH) await step() }
/** Keeps the failure for the screen (shown as a sentence by errorText) and logs the detail for developers. */
function failure(e: unknown): unknown {
  console.error('sign-in failed', e)
  return e
}
const google = () => firebase(() => signInWithPopup(auth(), new GoogleAuthProvider()))

/**
 * Who is signing in. Each tile shows its picture, Urdu and English together whatever the language setting,
 * so a worker who cannot read English still finds the wrench; the spoken prompt asks the same question.
 */
export function RolePick() {
  const t = useT()
  const ur = useLang() === 'ur'
  const flip = { display: 'flex', transform: ur ? 'scaleX(-1)' : undefined } // the chevron points the reading way
  return (
    <Task screen="01-role" wide>
      <PromptBar prompt="role" />
      <div className="section-heading"><h1>{t('Who are you?', 'آپ کون ہیں؟')}</h1></div>
      <div className="stack-lg">
        <Link className="role-tile worker-role" href="/worker/login">
          <div className="role-icon"><Icon name="Wrench" className="icon-lg" /></div>
          <div className="grow role-words">
            <p className="role-ur" lang="ur" dir="rtl">میں کاریگر ہوں</p>
            <p className="role-en" lang="en" dir="ltr">I’m a worker. I want work.</p>
          </div>
          <span style={flip}><Icon name="ChevronRight" /></span>
        </Link>
        <Link className="role-tile customer-role" href="/login">
          <div className="role-icon"><Icon name="House" className="icon-lg" /></div>
          <div className="grow role-words">
            <p className="role-ur" lang="ur" dir="rtl">مجھے کاریگر چاہیے</p>
            <p className="role-en" lang="en" dir="ltr">I need a worker.</p>
          </div>
          <span style={flip}><Icon name="ChevronRight" /></span>
        </Link>
      </div>
    </Task>
  )
}

/** Says whose sign-in page this is, in both languages, so nobody joins with the wrong role by accident. */
function RoleBanner({ role }: { role: User['role'] }) {
  const worker = role === 'worker'
  return (
    <div className={`role-banner ${worker ? 'worker-role' : 'customer-role'}`}>
      <Icon name={worker ? 'Wrench' : 'House'} />
      <div className="role-words">
        <p className="role-ur" lang="ur" dir="rtl">{worker ? 'کاریگر سائن اِن' : 'گاہک سائن اِن'}</p>
        <p className="role-en" lang="en" dir="ltr">{worker ? 'Worker sign-in' : 'Customer sign-in'}</p>
      </div>
    </div>
  )
}

/** The way out for someone on the other role's page. */
function OtherDoor({ to }: { to: User['role'] }) {
  const worker = to === 'worker'
  return (
    <Link className="other-door" href={worker ? '/worker/login' : '/login'}>
      <Icon name={worker ? 'Wrench' : 'House'} />
      <span className="role-words">
        <span className="role-ur" lang="ur" dir="rtl">{worker ? 'کاریگر ہیں؟ یہاں دبائیں' : 'کاریگر چاہیے؟ یہاں دبائیں'}</span>
        <span className="role-en" lang="en" dir="ltr">{worker ? 'Are you a worker? Sign in here' : 'Need a worker? Sign in here'}</span>
      </span>
    </Link>
  )
}

/**
 * Shown when an existing account signed in through the other role's page. The account's role never changes
 * (the API ignores the requested role for existing users), so say so instead of switching screens silently.
 */
function WrongDoor({ me, onContinue }: { me: Me; onContinue: () => void }) {
  const t = useT()
  const worker = me.user.role === 'worker'
  return (
    <Task screen={worker ? '02-worker-signin' : '03-customer-signin'} worker={worker}>
      <StatePanel kind="success" icon={worker ? 'Wrench' : 'House'}
        title={worker ? t('This number is registered as a worker', 'یہ نمبر کاریگر کے طور پر رجسٹر ہے')
          : t('This account is a customer account', 'یہ گاہک کا اکاؤنٹ ہے')}
        text={worker ? t('You are signed in to your worker account.', 'آپ اپنے کاریگر اکاؤنٹ میں سائن اِن ہیں۔')
          : t('A worker account needs its own phone number. You are signed in as a customer.', 'کاریگر اکاؤنٹ کے لیے الگ فون نمبر چاہیے۔ آپ گاہک کے طور پر سائن اِن ہیں۔')} />
      <button className="button full" type="button" onClick={onContinue}>
        <Icon name="ArrowRight" />{worker ? t('Continue as a worker', 'کاریگر کے طور پر جاری رکھیں') : t('Continue as a customer', 'گاہک کے طور پر جاری رکھیں')}
      </button>
      <button className="button outline full" type="button" onClick={() => signOut()}>
        <Icon name="LogOut" />{t('Sign out', 'سائن آؤٹ')}
      </button>
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
  const [wrong, setWrong] = useState<Me | null>(null)
  async function finish() {
    const me = await signIn('worker', false)
    if (me.user.role !== 'worker') return setWrong(me)
    setMe(me)
    navigate(me.worker ? '/app' : '/worker/onboard')
  }
  if (wrong) return <WrongDoor me={wrong} onContinue={() => { setMe(wrong); navigate('/app') }} />
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
      <RoleBanner role="worker" />
      <PhoneForm ur onDone={finish} />
      {state === 'error' && <><StatePanel kind="error" title={errorText(why, t)} /></>}
      <button className="button outline full" type="button" onClick={withGoogle} disabled={state === 'loading'}>
        <GoogleMark /><span lang="en" dir="ltr">Continue with Google</span>
      </button>
      <OtherDoor to="customer" />
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
  const [wrong, setWrong] = useState<Me | null>(null)
  const signup = mode === 'signup'
  async function enter() {
    const me = await signIn('customer', false)
    if (me.user.role !== 'customer') return setWrong(me)
    setMe(me)
    navigate(next)
  }

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
      await enter()
    } catch (e) {
      setWhy(failure(e))
      setState('error')
    }
  }

  if (wrong) return <WrongDoor me={wrong} onContinue={() => { setMe(wrong); navigate(wrong.worker ? '/app' : '/worker/onboard') }} />
  if (state === 'error') return (
    <Task screen="03-customer-signin">
      <StatePanel kind="error" title={t('We couldn’t sign you in', 'سائن اِن نہیں ہو سکا')}
        text={errorText(why, t)} />
      <button className="button full" type="button" onClick={() => setState('idle')}><Icon name="RefreshCw" />{t('Try again', 'دوبارہ کوشش کریں')}</button>
    </Task>
  )

  return (
    <Task screen={signup ? '04-customer-signup' : '03-customer-signin'}>
      <RoleBanner role="customer" />
      <div className="section-heading">
        <h1>{signup ? t('Create your account', 'اپنا اکاؤنٹ بنائیں') : t('Sign in to find a worker', 'کاریگر ڈھونڈنے کے لیے سائن اِن کریں')}</h1>
        <p className="muted">{signup
          ? t('You’ll use it to book workers and review their work.', 'اسی سے آپ کاریگر بک کریں گے اور ان کے کام کا ریویو دیں گے۔')
          : t('Electricians, plumbers, AC technicians, carpenters and mechanics near you.', 'آپ کے قریب الیکٹریشن، پلمبر، اے سی ٹیکنیشن، کارپینٹر اور مکینک۔')}</p>
      </div>
      <PhoneForm onDone={enter} />
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
      <OtherDoor to="worker" />
    </Task>
  )
}

