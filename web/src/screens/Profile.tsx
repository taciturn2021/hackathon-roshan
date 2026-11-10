import { useState } from 'react'
import { Icon } from '../components/Icon'
import { Task } from '../components/Task'
import { Alert, AudioMessage, Badge, Skeletons, StatePanel, StatTiles } from '../components/ui'
import { api, isWorkerBusy } from '../lib/api'
import { play, toast } from '../lib/audio'
import { problemDraft } from '../lib/draft'
import { useT } from '../lib/i18n'
import { Link, navigate } from '../lib/router'
import { useLoad, useRole } from '../lib/session'
import { initials, km, rupees, TAGS, TAGS_UR, TRADES } from '../lib/trades'

export function WorkerProfile({ id }: { id: number }) {
  const me = useRole('customer')
  const t = useT()
  const worker = useLoad(() => api.getWorker(id), [id])
  const [state, setState] = useState<'idle' | 'sending' | 'busy' | 'error'>('idle')
  const draft = problemDraft.get()
  const s = worker.state

  if (!me) return null
  if (s.status === 'loading') return <Task screen="14-worker-profile"><Skeletons count={3} /></Task>
  if (s.status === 'error') return (
    <Task screen="14-worker-profile">
      <StatePanel kind="error" title={t('Couldn’t load this profile', 'پروفائل نہیں مل سکا')} text={t('Check your connection and try again.', 'انٹرنیٹ چیک کریں اور دوبارہ کوشش کریں۔')} />
      <button className="button full" type="button" onClick={worker.reload}><Icon name="RefreshCw" />{t('Try again', 'دوبارہ کوشش')}</button>
    </Task>
  )
  const w = s.data
  const tr = TRADES[w.trade]
  const first = w.name
  const canRequest = draft.lat !== null && draft.lng !== null

  async function request() {
    if (draft.lat === null || draft.lng === null) return
    setState('sending')
    try {
      const job = await api.createJob({ worker_id: w.id, trade: w.trade, description: draft.description || t(`${tr.en} needed`, `${tr.ur} کا کام چاہیے`), urgency: draft.urgency, lat: draft.lat, lng: draft.lng })
      navigate(`/app/jobs/${job.id}`)
    } catch (e) {
      setState(isWorkerBusy(e) ? 'busy' : 'error')
    }
  }

  return (
    <Task screen="14-worker-profile">
      <div className="profile-header">
        <span className="avatar large">{initials(w.name)}</span>
        {w.available && w.is_online ? <Badge tone="success">{t('Available now', 'ابھی دستیاب')}</Badge> : <Badge>{t('Not available', 'دستیاب نہیں')}</Badge>}
        <h1>{w.name}</h1>
        <p className="muted">{[t(tr.en, `${tr.ur} کا کام`), w.area].filter(Boolean).join(' · ')}</p>
        <p className="caption muted">{[
          w.distance_km !== null && t(`${km(w.distance_km)} away`, `${w.distance_km.toFixed(1)} کلومیٹر دور`),
          w.years_exp !== null && t(`${w.years_exp} years of experience`, `${w.years_exp} سال کا تجربہ`),
        ].filter(Boolean).join(' · ')}</p>
      </div>
      {w.intro_audio_url && (
        <button type="button" className="button outline full" onClick={() => play(w.intro_audio_url!).catch(() => toast(t('Audio is not available yet.', 'آڈیو ابھی دستیاب نہیں۔')))}>
          <Icon name="Play" />{t(`Listen to ${first}’s introduction`, `${first} کا تعارف سنیں`)}
        </button>
      )}
      <div className="stat-grid">
        <div className="stat"><strong>{w.rating_avg?.toFixed(1) ?? t('New', 'نیا')}</strong><small className="muted">{t('Rating', 'ریٹنگ')}</small></div>
        <div className="stat"><strong>{w.jobs_done}</strong><small className="muted">{t('Jobs done', 'مکمل کام')}</small></div>
        <div className="stat"><strong>{w.years_exp ?? '—'}</strong><small className="muted">{t('Years', 'سال')}</small></div>
      </div>
      {w.trust_summary && (
        <div className="trust-summary">
          <div className="row"><Icon name="MessageCircle" /><h3>{t('What customers say', 'گاہک کیا کہتے ہیں')}</h3></div>
          <p className="caption" style={{ marginTop: 12 }}>{w.trust_summary}</p>
        </div>
      )}
      {w.tags.length > 0 && <div className="tags">{w.tags.map((tag) => <Badge key={tag} tone="success">{t(TAGS[tag] ?? tag, TAGS_UR[tag] ?? tag)}</Badge>)}</div>}
      <section>
        <h2>{t('Reviews from completed jobs', 'مکمل کاموں کے ریویو')}</h2>
        {w.reviews.length === 0 && <p className="muted" style={{ marginTop: 12 }}>{t('No reviews yet. Reviews appear only after completed jobs.', 'ابھی کوئی ریویو نہیں۔ ریویو صرف کام مکمل ہونے کے بعد آتے ہیں۔')}</p>}
        {w.reviews.map((r) => (
          <div className="review" key={r.job_id}>
            <div className="row between"><strong className="digits">{r.rating} / 5</strong><span className="caption muted">{t('Completed job', 'مکمل کام')}</span></div>
            {r.body && <p>{r.body}</p>}
            {r.audio_url && <AudioMessage url={r.audio_url} />}
          </div>
        ))}
      </section>
      {w.price_range && (
        <section className="card card-soft">
          <p className="caption muted">{t(`Typical completed ${tr.en} jobs`, `${tr.ur} کے مکمل کاموں کی عام قیمت`)}</p>
          <p className="price">{rupees(w.price_range.low)}–{w.price_range.high.toLocaleString('en-PK')}</p>
        </section>
      )}
      {state === 'busy' && <Alert title={t(`${first} is no longer available`, `${first} اب دستیاب نہیں`)} text={t('No request was created. Choose another worker nearby.', 'کوئی درخواست نہیں بھیجی گئی۔ قریب سے کوئی اور کاریگر چنیں۔')} />}
      {state === 'error' && <Alert tone="error" title={t('Request not sent', 'درخواست نہیں بھیجی جا سکی')} text={t('Nothing was booked. Try again.', 'کوئی بکنگ نہیں ہوئی۔ دوبارہ کوشش کریں۔')} />}
      {!canRequest ? (
        <Link className="button full profile-request" href="/app/find/location"><Icon name="LocateFixed" />{t('Share your location to request', 'درخواست کے لیے لوکیشن بتائیں')}</Link>
      ) : state === 'busy' ? (
        <Link className="button full profile-request" href="/app/workers"><Icon name="Search" />{t('Find another worker', 'کوئی اور کاریگر ڈھونڈیں')}</Link>
      ) : (
        <button className="button full profile-request" type="button" onClick={request} disabled={state === 'sending' || !w.available}>
          <Icon name={state === 'sending' ? 'LoaderCircle' : 'ArrowRight'} />{t('Request this worker', 'اس کاریگر کو درخواست بھیجیں')}
        </button>
      )}
    </Task>
  )
}

/** The signed-in worker's own profile, as customers see it. */
export function OwnProfile() {
  const me = useRole('worker')
  const t = useT()
  const id = me?.worker?.id ?? 0
  const worker = useLoad(() => (id ? api.getWorker(id) : new Promise<never>(() => undefined)), [id]) // no worker: the role check redirects
  if (!me) return null
  const s = worker.state
  if (s.status === 'loading') return <Task screen="worker-profile" worker wide><Skeletons count={2} /></Task>
  if (s.status === 'error') return (
    <Task screen="worker-profile" worker wide>
      <StatePanel kind="error" title={t('Couldn’t load your profile', 'پروفائل نہیں مل سکا')} />
      <button className="button full" type="button" onClick={worker.reload}><Icon name="RefreshCw" />{t('Try again', 'دوبارہ کوشش')}</button>
    </Task>
  )
  const w = s.data
  const tr = TRADES[w.trade]
  return (
    <Task screen="worker-profile" worker prompt="confirm" wide>
      <div className="profile-strip">
        <span className="avatar large">{initials(w.name)}</span>
        <div className="grow">
          <h2 lang="en" dir="ltr">{w.name}</h2>
          <p className="icon-detail"><Icon name={tr.icon} /><span>{t(tr.en, `${tr.ur} کا کام`)}</span></p>
          {w.area && <p className="icon-detail"><Icon name="MapPin" /><span className="digits">{w.area}</span></p>}
        </div>
        {w.intro_audio_url && <AudioMessage url={w.intro_audio_url} ur />}
      </div>
      <StatTiles tiles={[
        { label: t('Rating', 'ریٹنگ'), value: w.rating_avg?.toFixed(1) ?? '—', icon: 'Star', accent: true },
        { label: t('Jobs done', 'مکمل کام'), value: String(w.jobs_done), icon: 'CircleCheck' },
        { label: t('Years', 'سال'), value: w.years_exp !== null ? String(w.years_exp) : '—', icon: 'Calendar' },
      ]} />
      <div className="dash-grid">
        <section className="panel span-2">
          <header className="panel-head"><h2>{t('Reviews from completed jobs', 'مکمل کاموں کے ریویو')}</h2><span className="muted digits">{w.reviews.length}</span></header>
          {w.reviews.length === 0 ? <p className="muted">{t('No reviews yet.', 'ابھی کوئی ریویو نہیں۔')}</p> : w.reviews.map((r) => (
            <div className="review" key={r.job_id}>
              <div className="row between"><strong className="digits">{r.rating} / 5</strong><span className="caption muted">{t('Completed job', 'مکمل کام')}</span></div>
              {r.body && <p>{r.body}</p>}
              {r.audio_url && <AudioMessage url={r.audio_url} />}
            </div>
          ))}
        </section>
        <section className="panel">
          <header className="panel-head"><h2>{t('What customers see', 'گاہک کو کیا نظر آتا ہے')}</h2></header>
          {w.trust_summary && <p>{w.trust_summary}</p>}
          {w.tags.length > 0 && <div className="tags">{w.tags.map((tag) => <Badge key={tag} tone="success">{t(TAGS[tag] ?? tag, TAGS_UR[tag] ?? tag)}</Badge>)}</div>}
          {w.price_range && <p className="muted">{t(`Typical ${tr.en} job:`, `${tr.ur} کے کام کی عام قیمت:`)} <span className="digits">{rupees(w.price_range.low)}–{w.price_range.high.toLocaleString('en-PK')}</span></p>}
        </section>
      </div>
    </Task>
  )
}
