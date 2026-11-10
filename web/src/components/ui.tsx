import { useState, type Dispatch, type ReactNode, type SetStateAction } from 'react'
import type { Job, JobStatus, Trade } from '../contract'
import { play, playPrompt, toast } from '../lib/audio'
import { useT } from '../lib/i18n'
import { km, TRADE_KEYS, TRADES, URGENCY } from '../lib/trades'
import { Icon, type IconName } from './Icon'

const WAVE = [9, 16, 12, 24, 18, 30, 14, 22, 28, 18, 10, 24, 32, 20, 13, 26, 18, 10, 15, 22, 12, 8]
const NOT_READY = { en: 'Audio is not available yet.', ur: 'آواز ابھی دستیاب نہیں۔' }

/** Play button with waveform. A null url means the audio is still being prepared. (`ur` is no longer used: text follows the chosen language.) */
export function AudioMessage({ url }: { url: string | null; ur?: boolean }) {
  const t = useT()
  const [duration, setDuration] = useState<string | null>(null)
  const start = () => {
    if (!url) return
    play(url).then((a) => {
      // Recorded webm clips report Infinity until fully read; show no duration rather than "Infinity:NaN".
      if (Number.isFinite(a.duration)) setDuration(`${Math.floor(a.duration / 60)}:${String(Math.round(a.duration % 60)).padStart(2, '0')}`)
    }, () => toast(t(NOT_READY.en, NOT_READY.ur)))
  }
  return (
    <div className="audio-message">
      <button className="icon-button" type="button" onClick={start} disabled={!url}
        aria-label={url ? t('Play voice message', 'آواز سنیں') : t('Audio is being prepared', 'آواز تیار ہو رہی ہے')}>
        <Icon name={url ? 'Play' : 'LoaderCircle'} />
      </button>
      <span className="wave" aria-hidden="true">{WAVE.map((h, i) => <span key={i} style={{ height: h }} />)}</span>
      {duration && <span className="caption digits">{duration}</span>}
    </div>
  )
}

/** Worker screens: replayable spoken instruction at the top. */
export function PromptBar({ prompt }: { prompt: string }) {
  const t = useT()
  return (
    <div className="promptbar">
      <button className="icon-button" type="button" aria-label={t('Listen to the instructions', 'ہدایات سنیں')} onClick={() => playPrompt(prompt).catch(() => toast(t(NOT_READY.en, NOT_READY.ur)))}>
        <Icon name="Volume2" />
      </button>
      <span>{t('Tap to listen', 'سننے کے لیے دبائیں')}</span>
    </div>
  )
}

export function StatePanel({ kind, title, text, icon }: { kind: 'loading' | 'error' | 'success'; title: string; text?: string; icon?: IconName }) {
  const fallback: IconName = kind === 'loading' ? 'LoaderCircle' : kind === 'error' ? 'CircleAlert' : 'Check'
  return (
    <section className="state-panel" role={kind === 'error' ? 'alert' : 'status'}>
      <div className={`state-icon ${kind === 'success' ? '' : kind}`}><Icon name={icon ?? fallback} /></div>
      <h1>{title}</h1>
      {text && <p className="muted">{text}</p>}
    </section>
  )
}

export function Skeletons({ count = 2 }: { count?: number }) {
  return Array.from({ length: count }, (_, i) => (
    <div className="skeleton-card" aria-hidden="true" key={i}>
      <div className="row"><div className="skeleton round" /><div className="grow stack-sm"><div className="skeleton" /><div className="skeleton" style={{ width: '65%' }} /></div></div>
      <div className="skeleton" />
    </div>
  ))
}

export function Alert({ title, text, tone = 'info' }: { title: string; text?: string; tone?: 'info' | 'error' | 'success' }) {
  return (
    <div className={`alert ${tone === 'error' ? '' : tone}`} role={tone === 'error' ? 'alert' : 'status'}>
      <Icon name={tone === 'success' ? 'CircleCheck' : 'CircleAlert'} />
      <div><strong>{title}</strong>{text && <p>{text}</p>}</div>
    </div>
  )
}

export function Badge({ tone, children }: { tone?: 'success' | 'warning' | 'error'; children: ReactNode }) {
  return <span className={`badge ${tone ?? ''}`}><span className="dot" />{children}</span>
}

export function TradeIcon({ trade, small }: { trade: Trade; small?: boolean }) {
  return <span className={`trade-icon ${small ? 'small' : ''}`}><Icon name={TRADES[trade].icon} className="icon-lg" /></span>
}

/** The five trade pictures as a picker. */
export function TradeGrid({ onPick, selected }: { onPick: (t: Trade) => void; ur?: boolean; selected?: Trade | null }) {
  const t = useT()
  return (
    <div className="grid-2">
      {TRADE_KEYS.map((k) => (
        <button key={k} type="button" className={`trade-tile ${selected === k ? 'selected' : ''}`} aria-pressed={selected === k} onClick={() => onPick(k)}>
          <TradeIcon trade={k} />
          <span>{t(TRADES[k].en, TRADES[k].ur)}</span>
        </button>
      ))}
    </div>
  )
}

const STEPS: JobStatus[] = ['requested', 'confirmed', 'in_progress', 'completed']
export function Timeline({ status }: { status: JobStatus; ur?: boolean }) {
  const t = useT()
  const at = STEPS.indexOf(status)
  const labels = [t('Requested', 'درخواست'), t('Confirmed', 'پکا'), t('Working', 'جاری'), t('Done', 'مکمل')]
  return (
    <div className="timeline" aria-label={t('Job progress', 'کام کی پیش رفت')}>
      {labels.map((l, i) => (
        <div key={l} className={`step ${i <= at ? 'active' : ''}`} aria-current={i === at ? 'step' : undefined}>
          <div className="step-line" />{l}
        </div>
      ))}
    </div>
  )
}

/** Job summary card. Customers read the description; workers get the Urdu brief with its audio. */
export function JobCard({ job, worker = false }: { job: Job; worker?: boolean }) {
  const t = useT()
  const trade = TRADES[job.trade]
  const place = worker ? km(job.worker.distance_km) : [job.worker.area, km(job.worker.distance_km)].filter(Boolean).join(' · ')
  return (
    <section className="card">
      <div className="row">
        <TradeIcon trade={job.trade} small />
        <div className="grow">
          <h2>{t(trade.en, `${trade.ur} کا کام`)}</h2>
          {place && <p className="caption muted digits">{place}</p>}
        </div>
        <Badge tone={job.urgency === 'flexible' ? undefined : 'warning'}>{t(URGENCY[job.urgency].en, URGENCY[job.urgency].ur)}</Badge>
      </div>
      {worker ? (
        <>
          <AudioMessage url={job.brief_audio_url} ur />
          {job.brief_ur && <p className="urdu-message">{job.brief_ur}</p>}
        </>
      ) : job.description && <p className="muted">{job.description}</p>}
    </section>
  )
}

export function Keypad({ onChange }: { onChange: Dispatch<SetStateAction<string>> }) {
  const t = useT()
  const press = (k: string) => onChange((v) => (k === 'Delete' ? v.slice(0, -1) : (v + k).replace(/^0+/, '').slice(0, 7)))
  return (
    <div className="keypad">
      {['1', '2', '3', '4', '5', '6', '7', '8', '9', '00', '0'].map((k) => <button key={k} type="button" aria-label={k} onClick={() => press(k)}>{k}</button>)}
      <button type="button" aria-label={t('Delete', 'مٹائیں')} onClick={() => press('Delete')}><Icon name="Delete" /></button>
    </div>
  )
}

/** Google's four-colour "G", as its sign-in guidelines require. */
export function GoogleMark() {
  return (
    <span className="google-mark" aria-hidden="true">
      <svg viewBox="0 0 48 48" width="20" height="20">
        <path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z" />
        <path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z" />
        <path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z" />
        <path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z" />
      </svg>
    </span>
  )
}

export type Tile = { label: string; value: string; icon: IconName; note?: string; accent?: boolean }
/** Row of headline numbers. */
export function StatTiles({ tiles }: { tiles: Tile[] }) {
  return (
    <div className="stat-tiles">
      {tiles.map((t) => (
        <div className={`tile ${t.accent ? 'accent' : ''}`} key={t.label}>
          <span className="tile-label"><Icon name={t.icon} />{t.label}</span>
          <strong className="tile-value digits">{t.value}</strong>
          {t.note && <small className="muted">{t.note}</small>}
        </div>
      ))}
    </div>
  )
}
