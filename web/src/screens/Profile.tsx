import { useState } from 'react'
import { Icon } from '../components/Icon'
import { Task } from '../components/Task'
import { Alert, AudioMessage, Badge, Skeletons, StatePanel, StatTiles } from '../components/ui'
import { api, isWorkerBusy } from '../lib/api'
import { play, toast } from '../lib/audio'
import { problemDraft } from '../lib/draft'
import { Link, navigate } from '../lib/router'
import { useLoad, useRole } from '../lib/session'
import { initials, km, rupees, TAGS, TRADES } from '../lib/trades'

export function WorkerProfile({ id }: { id: number }) {
  const me = useRole('customer')
  const worker = useLoad(() => api.getWorker(id), [id])
  const [state, setState] = useState<'idle' | 'sending' | 'busy' | 'error'>('idle')
  const draft = problemDraft.get()
  const s = worker.state

  if (!me) return null
  if (s.status === 'loading') return <Task screen="14-worker-profile"><Skeletons count={3} /></Task>
  if (s.status === 'error') return (
    <Task screen="14-worker-profile">
      <StatePanel kind="error" title="Couldn’t load this profile" text="Check your connection and try again." />
      <button className="button full" type="button" onClick={worker.reload}><Icon name="RefreshCw" />Try again</button>
    </Task>
  )
  const w = s.data
  const t = TRADES[w.trade]
  const first = w.name.split(' ').slice(-1)[0]
  const canRequest = draft.lat !== null && draft.lng !== null

  async function request() {
    if (draft.lat === null || draft.lng === null) return
    setState('sending')
    try {
      const job = await api.createJob({ worker_id: w.id, trade: w.trade, description: draft.description, urgency: draft.urgency, lat: draft.lat, lng: draft.lng })
      navigate(`/app/jobs/${job.id}`)
    } catch (e) {
      setState(isWorkerBusy(e) ? 'busy' : 'error')
    }
  }

  return (
    <Task screen="14-worker-profile">
      <div className="profile-header">
        <span className="avatar large">{initials(w.name)}</span>
        {w.available && w.is_online ? <Badge tone="success">Available now</Badge> : <Badge>Not available</Badge>}
        <h1>{w.name}</h1>
        <p className="muted">{[t.en, w.area].filter(Boolean).join(' · ')}</p>
        <p className="caption muted">{[w.distance_km !== null && `${km(w.distance_km)} away`, w.years_exp !== null && `${w.years_exp} years of experience`].filter(Boolean).join(' · ')}</p>
      </div>
      {w.intro_audio_url && (
        <button type="button" className="button outline full" onClick={() => play(w.intro_audio_url!).catch(() => toast('Audio is not available yet.'))}>
          <Icon name="Play" />Listen to {first}’s introduction
        </button>
      )}
      <div className="stat-grid">
        <div className="stat"><strong>{w.rating_avg?.toFixed(1) ?? 'New'}</strong><small className="muted">Rating</small></div>
        <div className="stat"><strong>{w.jobs_done}</strong><small className="muted">Jobs done</small></div>
        <div className="stat"><strong>{w.years_exp ?? '—'}</strong><small className="muted">Years</small></div>
      </div>
      {w.trust_summary && (
        <div className="trust-summary">
          <div className="row"><Icon name="MessageCircle" /><h3>What customers say</h3></div>
          <p className="caption" style={{ marginTop: 12 }}>{w.trust_summary}</p>
        </div>
      )}
      {w.tags.length > 0 && <div className="tags">{w.tags.map((tag) => <Badge key={tag} tone="success">{TAGS[tag] ?? tag}</Badge>)}</div>}
      <section>
        <h2>Reviews from completed jobs</h2>
        {w.reviews.length === 0 && <p className="muted" style={{ marginTop: 12 }}>No reviews yet. Reviews appear only after completed jobs.</p>}
        {w.reviews.map((r) => (
          <div className="review" key={r.job_id}>
            <div className="row between"><strong>{r.rating} / 5</strong><span className="caption muted">Completed job</span></div>
            {r.body && <p>{r.body}</p>}
            {r.audio_url && <AudioMessage url={r.audio_url} />}
          </div>
        ))}
      </section>
      {w.price_range && (
        <section className="card card-soft">
          <p className="caption muted">Typical completed {t.en} jobs</p>
          <p className="price">{rupees(w.price_range.low)}–{w.price_range.high.toLocaleString('en-PK')}</p>
        </section>
      )}
      {state === 'busy' && <Alert title={`${first} is no longer available`} text="No request was created. Choose another worker nearby." />}
      {state === 'error' && <Alert tone="error" title="Request not sent" text="Nothing was booked. Try again." />}
      {!canRequest ? (
        <Link className="button full profile-request" href="/app/find/location"><Icon name="LocateFixed" />Share your location to request</Link>
      ) : state === 'busy' ? (
        <Link className="button full profile-request" href="/app/workers"><Icon name="Search" />Find another worker</Link>
      ) : (
        <button className="button full profile-request" type="button" onClick={request} disabled={state === 'sending' || !w.available}>
          <Icon name={state === 'sending' ? 'LoaderCircle' : 'ArrowRight'} />Request this worker
        </button>
      )}
    </Task>
  )
}

/** The signed-in worker's own profile, as customers see it. */
export function OwnProfile() {
  const me = useRole('worker')
  const id = me?.worker?.id ?? 0
  const worker = useLoad(() => api.getWorker(id), [id])
  if (!me) return null
  const s = worker.state
  if (s.status === 'loading') return <Task screen="worker-profile" worker wide><Skeletons count={2} /></Task>
  if (s.status === 'error') return (
    <Task screen="worker-profile" worker wide>
      <StatePanel kind="error" title="پروفائل نہیں مل سکا" />
      <button className="button full" type="button" onClick={worker.reload}><Icon name="RefreshCw" />دوبارہ کوشش</button>
    </Task>
  )
  const w = s.data
  const t = TRADES[w.trade]
  return (
    <Task screen="worker-profile" worker prompt="confirm" wide>
      <div className="profile-strip">
        <span className="avatar large">{initials(w.name)}</span>
        <div className="grow">
          <h2 lang="en" dir="ltr">{w.name}</h2>
          <p className="icon-detail"><Icon name={t.icon} /><span>{t.ur} کا کام</span></p>
          {w.area && <p className="icon-detail"><Icon name="MapPin" /><span className="digits">{w.area}</span></p>}
        </div>
        {w.intro_audio_url && <AudioMessage url={w.intro_audio_url} ur />}
      </div>
      <StatTiles tiles={[
        { label: 'ریٹنگ', value: w.rating_avg?.toFixed(1) ?? '—', icon: 'Star', accent: true },
        { label: 'مکمل کام', value: String(w.jobs_done), icon: 'CircleCheck' },
        { label: 'سال', value: w.years_exp !== null ? String(w.years_exp) : '—', icon: 'Calendar' },
      ]} />
      <div className="dash-grid">
        <section className="panel span-2" lang="en" dir="ltr">
          <header className="panel-head"><h2>Reviews from completed jobs</h2><span className="muted digits">{w.reviews.length}</span></header>
          {w.reviews.length === 0 ? <p className="muted">No reviews yet.</p> : w.reviews.map((r) => (
            <div className="review" key={r.job_id}>
              <div className="row between"><strong className="digits">{r.rating} / 5</strong><span className="caption muted">Completed job</span></div>
              {r.body && <p>{r.body}</p>}
              {r.audio_url && <AudioMessage url={r.audio_url} />}
            </div>
          ))}
        </section>
        <section className="panel" lang="en" dir="ltr">
          <header className="panel-head"><h2>What customers see</h2></header>
          {w.trust_summary && <p>{w.trust_summary}</p>}
          {w.tags.length > 0 && <div className="tags">{w.tags.map((tag) => <Badge key={tag} tone="success">{TAGS[tag] ?? tag}</Badge>)}</div>}
          {w.price_range && <p className="muted">Typical {t.en} job: <span className="digits">{rupees(w.price_range.low)}–{w.price_range.high.toLocaleString('en-PK')}</span></p>}
        </section>
      </div>
    </Task>
  )
}
