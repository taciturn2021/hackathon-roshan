import { useEffect, useRef, useState } from 'react'
import { Icon, type IconName } from '../components/Icon'
import { Task } from '../components/Task'
import { Alert, AudioMessage, StatePanel, TradeGrid } from '../components/ui'
import type { OnboardAnswer, OnboardField, Trade } from '../contract'
import { api } from '../lib/api'
import { MOCK_LOCATION } from '../lib/mocks'
import { playPrompt, toast, useRecorder } from '../lib/audio'
import { locate, onboardDraft } from '../lib/draft'
import { Link, navigate } from '../lib/router'
import { setMe, useRole } from '../lib/session'
import { initials, TRADES } from '../lib/trades'

const FIELDS: OnboardField[] = ['name', 'trade', 'area', 'years_exp']
const QUESTIONS: Record<OnboardField, { ur: string; icon: IconName; prompt: string }> = {
  name: { ur: 'آپ کا نام کیا ہے؟', icon: 'User', prompt: 'ask-name' },
  trade: { ur: 'آپ کیا کام کرتے ہیں؟', icon: 'Wrench', prompt: 'ask-trade' },
  area: { ur: 'آپ کس علاقے میں کام کرتے ہیں؟', icon: 'MapPin', prompt: 'ask-area' },
  years_exp: { ur: 'کتنے سال کا تجربہ ہے؟', icon: 'Calendar', prompt: 'ask-years' },
}
type Profile = OnboardAnswer['profile']
const filled = (p: Profile, f: OnboardField) => p[f] !== undefined && p[f] !== ''

const MAX_TRIES = 2

/** One question at a time: ask, listen, say what was understood, and only then move on. */
export function Onboard() {
  const me = useRole('worker')
  const [profile, setProfile] = useState<Profile>({})
  const [field, setField] = useState<OnboardField>('name')
  const [phase, setPhase] = useState<'asking' | 'thinking' | 'heard' | 'again' | 'picker'>('asking')
  const [transcript, setTranscript] = useState('')
  const tries = useRef(0)
  const latest = useRef<Profile>({})
  const intro = useRef<string | null>(null)
  const live = useRef(true)

  function finish(p: Profile) {
    onboardDraft.set({ profile: p, intro_audio_url: intro.current })
    navigate('/worker/location')
  }
  /** The next question the worker has not already answered (one answer can cover several). */
  function next(p: Profile) {
    if (!live.current) return
    const rest = FIELDS.slice(FIELDS.indexOf(field) + 1).find((f) => !filled(p, f))
    tries.current = 0
    setTranscript('')
    if (!rest) return finish(p)
    setField(rest)
    setPhase('asking')
  }

  const rec = useRecorder((clip) => {
    const asked = field
    tries.current++
    setPhase('thinking')
    api.onboard(clip, asked)
      .then((a) => {
        const defined = Object.fromEntries(Object.entries(a.profile).filter(([, v]) => v !== undefined && v !== ''))
        if (asked === 'trade' && a.confidence < 0.6) delete defined.trade // unsure: ask again, then the picker
        latest.current = { ...latest.current, ...defined }
        if (asked === 'trade' && a.audio_url) intro.current = a.audio_url
        setProfile(latest.current)
        setTranscript(a.transcript)
        return filled(latest.current, asked)
      }, () => false)
      .then((understood) => {
        if (!live.current) return
        if (understood) {
          setPhase('heard')
          playPrompt('ok').catch(() => undefined)
          setTimeout(() => next(latest.current), 1500)
        } else if (tries.current < MAX_TRIES) setPhase('again')
        else if (asked === 'trade') setPhase('picker')
        else next(latest.current) // name, area and years are not required: carry on
      })
  })

  useEffect(() => {
    live.current = true
    onboardDraft.clear()
    rec.prepare()
    return () => { live.current = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  // Only onboarding speaks by itself: the question when it appears, and "say it again" after an unclear answer.
  useEffect(() => {
    if (phase === 'asking') playPrompt(QUESTIONS[field].prompt).catch(() => undefined)
    if (phase === 'again') playPrompt('again').catch(() => undefined)
  }, [phase, field])

  if (!me) return null
  if (rec.state === 'denied') return (
    <Task screen="05-worker-onboard" worker prompt="error">
      <div className="section-heading"><h1>مائیک کی اجازت دیں</h1></div>
      <div className="state-panel"><div className="state-icon error"><Icon name="MicOff" /></div></div>
      <button className="button full" type="button" onClick={rec.reset}><Icon name="RefreshCw" />دوبارہ کوشش</button>
    </Task>
  )
  if (phase === 'picker') return (
    <Task screen="07-worker-trade" worker prompt="ask-trade">
      <div className="section-heading"><h1>اپنا کام چنیں</h1></div>
      <TradeGrid ur onPick={(trade: Trade) => { latest.current = { ...latest.current, trade }; setProfile(latest.current); next(latest.current) }} />
    </Task>
  )

  const recording = rec.state === 'recording'
  const busy = phase === 'thinking' || phase === 'heard'
  const understood: Record<OnboardField, string | undefined> = {
    name: profile.name,
    trade: profile.trade && `${TRADES[profile.trade].ur} کا کام`,
    area: profile.area,
    years_exp: profile.years_exp === undefined ? undefined : `${profile.years_exp} سال`,
  }

  return (
    <Task screen="05-worker-onboard" worker>
      <div className="onboard-progress">
        {FIELDS.map((f, i) => {
          const done = filled(profile, f), current = f === field
          return <span key={f} className={done ? 'complete' : current ? 'current' : ''}>{done ? '✓' : i + 1}</span>
        })}
      </div>
      <div className="onboard-question">
        <div className="question-symbol"><Icon name={QUESTIONS[field].icon} /></div>
        <h1>{QUESTIONS[field].ur}</h1>
        <button className="icon-button" type="button" aria-label="سوال سنیں" onClick={() => playPrompt(QUESTIONS[field].prompt).catch(() => toast('آواز ابھی دستیاب نہیں۔'))}>
          <Icon name="Volume2" />
        </button>
        <div aria-live="polite" style={{ minHeight: 76, width: '100%' }}>
          {phase === 'thinking' && <Alert title="سمجھ رہے ہیں…" />}
          {phase === 'heard' && <Alert tone="success" title="ٹھیک ہے" text={understood[field]} />}
          {phase === 'again' && <Alert tone="error" title="سمجھ نہیں آیا، دوبارہ بولیں" text={transcript ? `سنا گیا: ${transcript}` : undefined} />}
        </div>
        <div className="mic-zone">
          <button className={`mic ${recording ? 'recording' : ''}`} type="button" onClick={rec.toggle} disabled={busy}
            aria-label={recording ? 'جواب بھیجیں' : 'جواب دیں'} aria-pressed={recording}>
            <Icon name={phase === 'thinking' ? 'LoaderCircle' : 'Mic'} />
          </button>
          <p>{recording ? 'بولیں، پھر دوبارہ دبائیں' : busy ? 'ذرا رکیں' : 'بٹن دبائیں اور جواب دیں'}</p>
        </div>
      </div>
    </Task>
  )
}

export function WorkerLocation() {
  const me = useRole('worker')
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle')
  async function go(demo = false) {
    setState('loading')
    try {
      onboardDraft.set(demo ? MOCK_LOCATION : await locate())
      navigate('/worker/confirm')
    } catch {
      setState('error')
    }
  }
  if (!me) return null
  return (
    <Task screen="08-worker-location" worker prompt="ask-area">
      <div className="section-heading"><h1>آپ کہاں ہیں؟</h1></div>
      {state === 'error'
        ? <StatePanel kind="error" title="لوکیشن نہیں ملی" icon="MapPin" />
        : <div className="state-panel"><div className="empty-art"><Icon name="LocateFixed" /></div></div>}
      <button className="button full" type="button" onClick={() => go()} disabled={state === 'loading'}>
        <Icon name={state === 'loading' ? 'LoaderCircle' : state === 'error' ? 'RefreshCw' : 'LocateFixed'} />
        {state === 'error' ? 'دوبارہ کوشش' : 'لوکیشن دیں'}
      </button>
      {state === 'error' && <button className="button outline full" type="button" lang="en" dir="ltr" onClick={() => go(true)}>Use G-10, Islamabad</button>}
    </Task>
  )
}

export function WorkerConfirm() {
  const me = useRole('worker')
  const draft = onboardDraft.get()
  const p = draft.profile
  const [state, setState] = useState<'idle' | 'saving' | 'error'>('idle')
  useEffect(() => { if (!p.trade) navigate('/worker/onboard', true) }, [p.trade])
  if (!me || !p.trade) return null
  const trade = p.trade

  async function save() {
    setState('saving')
    try {
      const worker = await api.saveWorker({
        name: p.name || me!.user.name || 'Kaarigar', trade, skills: p.skills ?? [], years_exp: p.years_exp ?? null,
        area: p.area ?? null, lat: draft.lat, lng: draft.lng, intro_audio_url: draft.intro_audio_url,
      })
      setMe({ ...me!, worker })
      onboardDraft.clear()
      navigate('/app')
    } catch {
      setState('error')
    }
  }

  return (
    <Task screen="09-worker-confirm" worker prompt="confirm">
      <div className="section-heading"><h1>کیا معلومات ٹھیک ہیں؟</h1></div>
      {state === 'error' && <Alert tone="error" title="محفوظ نہیں ہوا۔ دوبارہ کوشش کریں۔" />}
      <section className="card">
        <div className="profile-header">
          <span className="avatar large digits">{initials(p.name ?? '?')}</span>
          <span className="trade-icon"><Icon name={TRADES[trade].icon} className="icon-lg" /></span>
          {p.name && <h1>{p.name}</h1>}
        </div>
        <div className="icon-detail"><Icon name={TRADES[trade].icon} /><span>{TRADES[trade].ur} کا کام</span></div>
        {p.area && <div className="icon-detail"><Icon name="MapPin" /><span className="digits">{p.area}</span></div>}
        {p.years_exp !== undefined && <div className="icon-detail"><Icon name="Calendar" /><span className="digits">{p.years_exp}</span><span>سال</span></div>}
        {draft.intro_audio_url && <AudioMessage url={draft.intro_audio_url} ur />}
      </section>
      <div className="row">
        <button className="button grow" type="button" onClick={save} disabled={state === 'saving'}>
          <Icon name={state === 'saving' ? 'LoaderCircle' : 'Check'} />ٹھیک ہے
        </button>
        <Link className="icon-button" aria-label="دوبارہ بولیں" href="/worker/onboard"><Icon name="Mic" /></Link>
      </div>
    </Task>
  )
}
