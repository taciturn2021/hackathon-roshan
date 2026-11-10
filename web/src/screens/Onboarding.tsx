import { useEffect, useRef, useState } from 'react'
import { Icon, type IconName } from '../components/Icon'
import { Task } from '../components/Task'
import { Alert, AudioMessage, TradeGrid } from '../components/ui'
import type { OnboardAnswer, OnboardField, Trade } from '../contract'
import { api } from '../lib/api'
import { MOCK_LOCATION } from '../lib/mocks'
import { playPrompt, toast, useRecorder } from '../lib/audio'
import { Hero, Steps } from '../components/Hero'
import { locate, onboardDraft } from '../lib/draft'
import { useT } from '../lib/i18n'
import { Link, navigate } from '../lib/router'
import { setMe, useRole } from '../lib/session'
import { initials, TRADES } from '../lib/trades'

const FIELDS: OnboardField[] = ['name', 'trade', 'area', 'years_exp']
const QUESTIONS: Record<OnboardField, { en: string; ur: string; icon: IconName; prompt: string }> = {
  name: { en: 'What is your name?', ur: 'آپ کا نام کیا ہے؟', icon: 'User', prompt: 'ask-name' },
  trade: { en: 'What work do you do?', ur: 'آپ کیا کام کرتے ہیں؟', icon: 'Wrench', prompt: 'ask-trade' },
  area: { en: 'Which area do you work in?', ur: 'آپ کس علاقے میں کام کرتے ہیں؟', icon: 'MapPin', prompt: 'ask-area' },
  years_exp: { en: 'How many years of experience?', ur: 'کتنے سال کا تجربہ ہے؟', icon: 'Calendar', prompt: 'ask-years' },
}
type Profile = OnboardAnswer['profile']
const filled = (p: Profile, f: OnboardField) => p[f] !== undefined && p[f] !== ''

const MAX_TRIES = 2

/** Where the worker is in signing up: answer the questions, give the location, check the profile. */
function SignupSteps({ at }: { at: number }) {
  const t = useT()
  return <Steps at={at} label={t('Steps to join', 'شامل ہونے کے قدم')} steps={[t('Answer 4 questions', '4 سوال'), t('Your location', 'لوکیشن'), t('Check and save', 'دیکھ کر محفوظ کریں')]} />
}

/** One question at a time: ask, listen, say what was understood, and only then move on. */
export function Onboard() {
  const t = useT()
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
        if (asked === 'trade' && defined.trade && a.audio_url) intro.current = a.audio_url // only an understood answer becomes the spoken introduction
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
        else {
          // name, area and years are not required: say it was skipped and carry on
          toast(t('Skipped this question. You can answer again at the end.', 'یہ سوال چھوڑ دیا۔ آخر میں دوبارہ بتا سکتے ہیں۔'))
          next(latest.current)
        }
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
      <Hero tone="stop" icon="MicOff" title={t('Allow the microphone', 'مائیک کی اجازت دیں')} hint={t('Your browser blocked the microphone. Allow it, then try again.', 'براؤزر نے مائیک روک دیا ہے۔ اجازت دیں، پھر دوبارہ کوشش کریں۔')} />
      <button className="button full" type="button" onClick={rec.reset}><Icon name="RefreshCw" />{t('Try again', 'دوبارہ کوشش')}</button>
    </Task>
  )
  if (phase === 'picker') return (
    <Task screen="07-worker-trade" worker prompt="ask-trade">
      <SignupSteps at={0} />
      <Hero tone="wait" icon="Wrench" title={t('Pick your work', 'اپنا کام چنیں')} hint={t('We could not hear it clearly. Tap the picture of your work.', 'آواز صاف نہیں آئی۔ اپنے کام کی تصویر دبائیں۔')} />
      <TradeGrid ur onPick={(trade: Trade) => { latest.current = { ...latest.current, trade }; setProfile(latest.current); next(latest.current) }} />
    </Task>
  )

  const recording = rec.state === 'recording'
  const busy = phase === 'thinking' || phase === 'heard'
  const understood: Record<OnboardField, string | undefined> = {
    name: profile.name,
    trade: profile.trade && t(`${TRADES[profile.trade].en} work`, `${TRADES[profile.trade].ur} کا کام`),
    area: profile.area,
    years_exp: profile.years_exp === undefined ? undefined : t(`${profile.years_exp} years`, `${profile.years_exp} سال`),
  }

  return (
    <Task screen="05-worker-onboard" worker>
      <SignupSteps at={0} />
      <p className="center caption muted digits">{t('Question', 'سوال')} {FIELDS.indexOf(field) + 1} / {FIELDS.length}</p>
      <div className="onboard-progress">
        {FIELDS.map((f, i) => {
          const done = filled(profile, f), current = f === field
          return <span key={f} className={done ? 'complete' : current ? 'current' : ''}>{done ? '✓' : i + 1}</span>
        })}
      </div>
      <div className="onboard-question">
        <div className="question-symbol"><Icon name={QUESTIONS[field].icon} /></div>
        <h1>{t(QUESTIONS[field].en, QUESTIONS[field].ur)}</h1>
        <button className="icon-button" type="button" aria-label={t('Hear the question', 'سوال سنیں')} onClick={() => playPrompt(QUESTIONS[field].prompt).catch(() => toast(t('Audio is not available yet.', 'آواز ابھی دستیاب نہیں۔')))}>
          <Icon name="Volume2" />
        </button>
        <div aria-live="polite" style={{ minHeight: 76, width: '100%' }}>
          {phase === 'thinking' && <Alert title={t('Understanding your answer…', 'سمجھ رہے ہیں…')} />}
          {phase === 'heard' && <Alert tone="success" title={t('Got it', 'ٹھیک ہے')} text={understood[field]} />}
          {phase === 'again' && <Alert tone="error" title={t('Did not understand. Please say it again.', 'سمجھ نہیں آیا، دوبارہ بولیں')} text={transcript ? `${t('Heard', 'سنا گیا')}: ${transcript}` : undefined} />}
        </div>
        <div className="mic-zone">
          <button className={`mic ${recording ? 'recording' : ''}`} type="button" onClick={rec.toggle} disabled={busy}
            aria-label={recording ? t('Send answer', 'جواب بھیجیں') : t('Answer', 'جواب دیں')} aria-pressed={recording}>
            <Icon name={phase === 'thinking' ? 'LoaderCircle' : 'Mic'} />
          </button>
          <p>{recording ? t('Speak, then tap again', 'بولیں، پھر دوبارہ دبائیں') : busy ? t('One moment', 'ذرا رکیں') : t('Tap the button and answer', 'بٹن دبائیں اور جواب دیں')}</p>
        </div>
      </div>
    </Task>
  )
}

export function WorkerLocation() {
  const t = useT()
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
      <SignupSteps at={1} />
      {state === 'error'
        ? <Hero tone="stop" icon="MapPin" title={t('Location not found', 'لوکیشن نہیں ملی')} hint={t('Allow location in your browser and try again, or use G-10.', 'براؤزر میں لوکیشن کی اجازت دے کر دوبارہ کوشش کریں، یا G-10 چنیں۔')} />
        : <Hero tone="wait" icon="MapPin" title={t('Where are you?', 'آپ کہاں ہیں؟')} hint={t('We use your location to send you jobs close by.', 'لوکیشن سے آپ کو قریب کے کام ملیں گے۔')} />}
      <button className="button full" type="button" onClick={() => go()} disabled={state === 'loading'}>
        <Icon name={state === 'loading' ? 'LoaderCircle' : state === 'error' ? 'RefreshCw' : 'LocateFixed'} />
        {state === 'error' ? t('Try again', 'دوبارہ کوشش') : t('Share my location', 'لوکیشن دیں')}
      </button>
      {state === 'error' && <button className="button outline full" type="button" onClick={() => go(true)}>{t('Use G-10, Islamabad', 'G-10 اسلام آباد چنیں')}</button>}
    </Task>
  )
}

export function WorkerConfirm() {
  const t = useT()
  const me = useRole('worker')
  const draft = onboardDraft.get()
  const p = draft.profile
  const [state, setState] = useState<'idle' | 'saving' | 'error'>('idle')
  useEffect(() => { if (!p.trade) navigate('/worker/onboard', true) }, [p.trade])
  if (!me || !p.trade) return null
  const trade = p.trade
  // Anything not answered this time keeps the value the profile already has.
  const had = me.worker
  const name = p.name || had?.name || me.user.name || 'Kaarigar'
  const area = p.area ?? had?.area ?? null
  const years = p.years_exp ?? had?.years_exp ?? null
  const intro = draft.intro_audio_url ?? had?.intro_audio_url ?? null
  const missing = [!p.name && !had?.name && t('name', 'نام'), area === null && t('area', 'علاقہ'), years === null && t('years of experience', 'تجربہ')].filter(Boolean)

  async function save() {
    setState('saving')
    try {
      const worker = await api.saveWorker({
        name, trade, skills: p.skills ?? [], years_exp: years, area,
        lat: draft.lat, lng: draft.lng, intro_audio_url: intro,
      })
      setMe({ ...me!, worker })
      onboardDraft.clear()
      toast(t('Profile saved. Welcome! Turn the button on to get work.', 'پروفائل بن گیا۔ خوش آمدید! کام لینے کے لیے بٹن آن کریں۔'))
      navigate('/app')
    } catch {
      setState('error')
    }
  }

  return (
    <Task screen="09-worker-confirm" worker prompt="confirm">
      <SignupSteps at={2} />
      <Hero tone="go" icon="CircleCheck" title={t('Is this right?', 'کیا معلومات ٹھیک ہیں؟')} hint={t('Green button to save. Microphone to answer again.', 'ٹھیک ہیں تو سبز بٹن دبائیں۔ دوبارہ بولنے کے لیے مائیک دبائیں۔')} />
      {state === 'error' && <Alert tone="error" title={t('Not saved. Try again.', 'محفوظ نہیں ہوا۔ دوبارہ کوشش کریں۔')} />}
      {missing.length > 0 && (
        <Alert title={t('Not heard: ', 'یہ نہیں سنا گیا: ') + missing.join(t(', ', '، '))} text={t('You can save now, or press the microphone to answer again.', 'ابھی محفوظ کر سکتے ہیں، یا دوبارہ بتانے کے لیے مائیک دبائیں۔')} />
      )}
      <section className="card">
        <div className="profile-header">
          <span className="avatar large digits">{initials(name)}</span>
          <span className="trade-icon"><Icon name={TRADES[trade].icon} className="icon-lg" /></span>
          <h1 dir="auto">{name}</h1>
        </div>
        <div className="icon-detail"><Icon name={TRADES[trade].icon} /><span>{t(`${TRADES[trade].en} work`, `${TRADES[trade].ur} کا کام`)}</span></div>
        {area && <div className="icon-detail"><Icon name="MapPin" /><span className="digits">{area}</span></div>}
        {years !== null && <div className="icon-detail"><Icon name="Calendar" /><span className="digits">{years}</span><span>{t('years', 'سال')}</span></div>}
        {intro && <AudioMessage url={intro} ur />}
      </section>
      <div className="row">
        <button className="button grow" type="button" onClick={save} disabled={state === 'saving'}>
          <Icon name={state === 'saving' ? 'LoaderCircle' : 'Check'} />{t('Yes, save', 'ٹھیک ہے')}
        </button>
        <Link className="icon-button" aria-label={t('Answer again', 'دوبارہ بولیں')} href="/worker/onboard"><Icon name="Mic" /></Link>
      </div>
    </Task>
  )
}
