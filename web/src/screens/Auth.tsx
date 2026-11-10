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
import { auth } from '../lib/firebase'
import { FAKE_AUTH, safeNext, signIn } from '../lib/session'

/** Runs a Firebase sign-in step; mock and dev modes skip it and go straight to the API session. */
const firebase = async (step: () => Promise<unknown>) => { if (!FAKE_AUTH) await step() }
/** Logs a sign-in failure and returns its code (e.g. `auth/too-many-requests`) to show under the message. */
function failure(e: unknown): string {
  console.error('sign-in failed', e)
  return (e as { code?: string }).code ?? (e instanceof Error ? e.message : String(e))
}
const Why = ({ code }: { code: string }) => <p className="caption muted" lang="en" dir="ltr" style={{ textAlign: 'center' }}>{code}</p>
const google = () => firebase(() => signInWithPopup(auth(), new GoogleAuthProvider()))

export function RolePick() {
  return (
    <Task screen="01-role" wide>
      <div className="stack-lg" style={{ paddingTop: 32 }}>
        <Link className="role-tile worker-role" href="/worker/login" aria-label="Worker · کاریگر">
          <div><Icon name="Wrench" className="icon-lg" /></div>
          <h1 lang="ur" dir="rtl" style={{ fontFamily: 'var(--font-urdu)', lineHeight: 1.9 }}>کاریگر</h1>
          <Icon name="ChevronRight" />
        </Link>
        <Link className="role-tile" href="/login">
          <div><Icon name="House" className="icon-lg" /></div>
          <h1>Customer</h1>
          <Icon name="ChevronRight" />
        </Link>
      </div>
    </Task>
  )
}

/** Phone number, then the SMS code. Both roles use it; `ur` shows the worker's Urdu labels and plays the error prompt. */
function PhoneForm({ ur = false, onDone }: { ur?: boolean; onDone: () => Promise<void> }) {
  const [step, setStep] = useState<'phone' | 'code'>('phone')
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle')
  const [why, setWhy] = useState('')
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
  const title = step === 'phone' ? (ur ? 'اپنا فون نمبر لکھیں' : 'Phone number') : ur ? 'میسج میں آیا کوڈ لکھیں' : 'Code from the SMS'
  const big = { fontSize: 24, minWidth: 0 }

  return (
    <form className="stack" onSubmit={submit}>
      {ur ? <div className="section-heading"><h1>{title}</h1></div> : <span className="field">{title}</span>}
      {state === 'error' && (ur
        ? <StatePanel kind="error" title={step === 'phone' ? 'نمبر نہیں چلا، دوبارہ کوشش کریں' : 'کوڈ غلط ہے، دوبارہ لکھیں'} />
        : <span className="field-error">That didn’t work. Check the {step === 'phone' ? 'number' : 'code'} and try again.</span>)}
      {state === 'error' && <Why code={why} />}
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
        <Icon name={busy ? 'LoaderCircle' : step === 'phone' ? (ur ? 'ArrowLeft' : 'ArrowRight') : 'Check'} />
        {step === 'phone' ? (ur ? 'آگے' : 'Send code') : ur ? 'ٹھیک ہے' : 'Verify and sign in'}
      </button>
      <div id="recaptcha" />
    </form>
  )
}

export function WorkerSignIn() {
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle')
  const [why, setWhy] = useState('')
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
      {state === 'error' && <><StatePanel kind="error" title="دوبارہ کوشش کریں" /><Why code={why} /></>}
      <button className="button outline full" type="button" onClick={withGoogle} disabled={state === 'loading'}>
        <GoogleMark /><span lang="en" dir="ltr">Continue with Google</span>
      </button>
    </Task>
  )
}

export function CustomerSignIn({ mode }: { mode: 'signin' | 'signup' }) {
  const { query } = useLocation()
  const next = safeNext(query.get('next'))
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<{ email?: string; password?: string }>({})
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle')
  const [why, setWhy] = useState('')
  const signup = mode === 'signup'

  const validate = () => {
    const e: typeof errors = {}
    if (!/^\S+@\S+\.\S+$/.test(email)) e.email = 'Enter a valid email address.'
    if (password.length < 6) e.password = 'Use at least 6 characters.'
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
      <StatePanel kind="error" title="We couldn’t sign you in" text="Check your email and password, then try again." />
      <Why code={why} />
      <button className="button full" type="button" onClick={() => setState('idle')}><Icon name="RefreshCw" />Try again</button>
    </Task>
  )

  return (
    <Task screen={signup ? '04-customer-signup' : '03-customer-signin'}>
      <div className="section-heading">
        <h1>{signup ? 'Create your account' : 'Sign in to find a worker'}</h1>
        <p className="muted">{signup ? 'You’ll use it to book workers and review their work.' : 'Electricians, plumbers, AC technicians, carpenters and mechanics near you.'}</p>
      </div>
      <PhoneForm onDone={async () => { await signIn('customer'); navigate(next) }} />
      <div className="or">or</div>
      <button className="button outline full" type="button" onClick={() => go()} disabled={state === 'loading'}>
        <GoogleMark /> Continue with Google
      </button>
      <div className="or">or continue with email</div>
      <form className="stack" onSubmit={go} noValidate>
        <label className="field" htmlFor="email">Email
          <input id="email" name="email" type="email" autoComplete="email" value={email} placeholder="Email"
            aria-invalid={!!errors.email} aria-describedby={errors.email ? 'email-error' : undefined}
            onChange={(e) => setEmail(e.target.value)} onBlur={() => email && validate()} />
          {errors.email && <span className="field-error" id="email-error">{errors.email}</span>}
        </label>
        <label className="field" htmlFor="password">Password
          <input id="password" name="password" type="password" autoComplete={signup ? 'new-password' : 'current-password'} value={password}
            placeholder="Password" aria-invalid={!!errors.password} aria-describedby={errors.password ? 'password-error' : undefined}
            onChange={(e) => setPassword(e.target.value)} />
          {errors.password && <span className="field-error" id="password-error">{errors.password}</span>}
        </label>
        <button className="button full" type="submit" disabled={state === 'loading'}>
          {state === 'loading' && <Icon name="LoaderCircle" />}{signup ? 'Create account' : 'Sign in'}
        </button>
      </form>
      <Link className="button link" href={(signup ? '/login' : '/signup') + `?next=${encodeURIComponent(next)}`}>
        {signup ? 'Already have an account? Sign in' : 'New here? Create an account'}
      </Link>
    </Task>
  )
}

