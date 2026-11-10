import { useState, type FormEvent } from 'react'
import { Icon } from '../components/Icon'
import { Task } from '../components/Task'
import { GoogleMark, StatePanel } from '../components/ui'
import { playPrompt } from '../lib/audio'
import { Link, navigate, useLocation } from '../lib/router'
import { safeNext, signIn } from '../lib/session'

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

export function WorkerSignIn() {
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle')
  async function go() {
    setState('loading')
    try {
      const me = await signIn('worker')
      navigate(me.worker ? '/app' : '/worker/onboard')
    } catch {
      setState('error')
      playPrompt('error').catch(() => undefined)
    }
  }
  return (
    <Task screen="02-worker-signin" worker prompt="signin">
      <div style={{ flex: 1 }} />
      {state === 'error' && <StatePanel kind="error" title="دوبارہ کوشش کریں" />}
      <button className="button full" type="button" onClick={go} disabled={state === 'loading'}>
        {state === 'loading' ? <Icon name="LoaderCircle" /> : <GoogleMark />}
        <span lang="en" dir="ltr">Continue with Google</span>
      </button>
      <div style={{ flex: 1 }} />
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
      await signIn('customer')
      navigate(next)
    } catch {
      setState('error')
    }
  }

  if (state === 'error') return (
    <Task screen="03-customer-signin">
      <StatePanel kind="error" title="We couldn’t sign you in" text="Check your email and password, then try again." />
      <button className="button full" type="button" onClick={() => setState('idle')}><Icon name="RefreshCw" />Try again</button>
    </Task>
  )

  return (
    <Task screen={signup ? '04-customer-signup' : '03-customer-signin'}>
      <div className="section-heading">
        <h1>{signup ? 'Create your account' : 'Sign in to find a worker'}</h1>
        <p className="muted">{signup ? 'You’ll use it to book workers and review their work.' : 'Electricians, plumbers, AC technicians, carpenters and mechanics near you.'}</p>
      </div>
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

