import { useState } from 'react'
import { Icon } from '../components/Icon'
import { Task } from '../components/Task'
import { Alert, Badge, Skeletons, StatePanel, TradeGrid, TradeIcon } from '../components/ui'
import type { Problem as Analysis, Trade, Urgency } from '../contract'
import { api, MOCK } from '../lib/api'
import { MOCK_LOCATION } from '../lib/mocks'
import { useRecorder } from '../lib/audio'
import { locate, problemDraft } from '../lib/draft'
import { Link, navigate } from '../lib/router'
import { useLoad, useRole } from '../lib/session'
import { initials, km, rupees, TRADES, URGENCY } from '../lib/trades'

function applyProblem(p: Analysis) {
  problemDraft.set({ description: p.description, trade: p.trade, trade_confidence: p.trade_confidence, urgency: p.urgency, price_range: p.price_range })
  navigate('/app/find/confirm')
}

export function Problem() {
  const me = useRole('customer')
  const [text, setText] = useState(problemDraft.get().description)
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle')
  const [empty, setEmpty] = useState(false)

  async function analyze(input: { text?: string; audio?: Blob }) {
    setState('loading')
    try {
      applyProblem(await api.analyzeProblem(input))
    } catch {
      setState('error')
    }
  }
  const rec = useRecorder((audio) => analyze({ audio }))
  const recording = rec.state === 'recording'

  if (!me) return null
  if (state === 'loading') return (
    <Task screen="11-problem"><StatePanel kind="loading" title="Finding the right trade" text="Your description is saved while this finishes." /></Task>
  )

  return (
    <Task screen="11-problem">
      <div className="section-heading"><h1>What needs fixing?</h1><p className="muted">Tell us in English, Urdu or Roman Urdu.</p></div>
      {state === 'error' && <Alert tone="error" title="We couldn’t read that" text="Your description is saved. Try again, or choose a trade below." />}
      {rec.state === 'denied' && <Alert tone="error" title="Microphone blocked" text="Allow microphone access in your browser settings, or type instead." />}
      <div className="stack">
        <label className="field" htmlFor="problem">Describe the problem
          <textarea id="problem" placeholder="For example: My AC is running but not cooling." value={text}
            aria-invalid={empty} aria-describedby={empty ? 'problem-error' : undefined}
            onChange={(e) => { setText(e.target.value); setEmpty(false); problemDraft.set({ description: e.target.value }) }} />
          {empty && <span className="field-error" id="problem-error">Describe the problem, or hold the mic and speak.</span>}
        </label>
        <div className="row between">
          <span className="note">{recording ? 'Recording… tap again to finish' : 'Or tap the mic and speak'}</span>
          <button className={`mic ${recording ? 'recording' : ''}`} type="button" aria-pressed={recording}
            aria-label={recording ? 'Stop recording' : 'Record your problem'} onClick={rec.toggle}><Icon name="Mic" /></button>
        </div>
      </div>
      <button className="button full" type="button" onClick={() => (text.trim() ? analyze({ text: text.trim() }) : setEmpty(true))}>
        <Icon name="ArrowRight" />Find the right trade
      </button>
      <div className="divider" />
      <div className="stack-sm">
        <h2>Know who you need?</h2>
        <TradeGrid onPick={(trade) => { problemDraft.set({ trade, trade_confidence: 1, price_range: null }); navigate('/app/find/confirm') }} />
      </div>
    </Task>
  )
}

export function ProblemConfirm() {
  const me = useRole('customer')
  const [draft, setDraft] = useState(problemDraft.get())
  const [picking, setPicking] = useState(draft.trade === null || draft.trade_confidence < 0.6)
  const update = (patch: Partial<typeof draft>) => setDraft(problemDraft.set(patch))

  if (!me) return null
  if (picking || !draft.trade) return (
    <Task screen="12-problem-confirm">
      <div className="section-heading"><h1>Which trade do you need?</h1><p className="muted">We’re unsure. Choose the closest match.</p></div>
      <TradeGrid selected={draft.trade} onPick={(trade: Trade) => { update({ trade, trade_confidence: 1, price_range: null }); setPicking(false) }} />
    </Task>
  )
  const t = TRADES[draft.trade]
  const range = draft.price_range
  return (
    <Task screen="12-problem-confirm">
      <div className="section-heading"><h1>Check your request</h1><p className="muted">Check the details before we look nearby.</p></div>
      <section className="card">
        <div className="row">
          <TradeIcon trade={draft.trade} />
          <div className="grow"><h2>{t.en}</h2><p className="caption muted">{draft.description ? 'Based on your description' : 'Your choice'}</p></div>
          <button className="icon-button" type="button" aria-label="Change trade" onClick={() => setPicking(true)}><Icon name="Pencil" /></button>
        </div>
        {draft.description && <p>{draft.description}</p>}
      </section>
      <div className="stack-sm">
        <h3>How urgent is it?</h3>
        <div className="segment" role="radiogroup" aria-label="Urgency">
          {(Object.keys(URGENCY) as Urgency[]).map((u) => (
            <button key={u} type="button" role="radio" aria-checked={draft.urgency === u}
              className={`button outline ${draft.urgency === u ? 'selected' : ''}`} onClick={() => update({ urgency: u })}>{URGENCY[u].en}</button>
          ))}
        </div>
        <p className="note">Urgency helps explain your request. It does not schedule a later booking.</p>
      </div>
      <section className="card card-soft">
        {range ? (
          <>
            <p className="caption muted">Typical completed {t.en} jobs</p>
            <p className="price">{rupees(range.low)}–{range.high.toLocaleString('en-PK')}</p>
            <p className="note">Based on {range.sample} completed jobs. A guide, not a quote. Agree your final price in chat.</p>
          </>
        ) : <p className="note">No price guide yet for this trade. Agree your price in chat.</p>}
      </section>
      <Link className="button full" href="/app/find/location"><Icon name="Search" />Find available workers</Link>
    </Task>
  )
}

export function CustomerLocation() {
  const me = useRole('customer')
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle')
  async function go(demo = false) {
    setState('loading')
    try {
      problemDraft.set({ ...(demo ? MOCK_LOCATION : await locate()), radius_km: 5 })
      navigate('/app/workers')
    } catch {
      setState('error')
    }
  }
  if (!me) return null
  return (
    <Task screen="23-customer-location">
      <div className="section-heading"><h1>Share your location</h1><p className="muted">Share your location to find available workers around you.</p></div>
      {state === 'error'
        ? <StatePanel kind="error" title="We couldn’t get your location" text="Allow location access in your browser, then try again. We never guess where you are." />
        : <div className="state-panel"><div className="empty-art"><Icon name="LocateFixed" /></div></div>}
      <p className="note">Your location is used for nearby matching. This is not live worker tracking.</p>
      <button className="button full" type="button" onClick={() => go()} disabled={state === 'loading'}>
        <Icon name={state === 'loading' ? 'LoaderCircle' : state === 'error' ? 'RefreshCw' : 'LocateFixed'} />
        {state === 'error' ? 'Try again' : 'Use my location'}
      </button>
      {MOCK && state === 'error' && <button className="button outline full" type="button" onClick={() => go(true)}>Use demo location (G-10, Islamabad)</button>}
    </Task>
  )
}

export function WorkerList() {
  const me = useRole('customer')
  const [draft, setDraft] = useState(problemDraft.get())
  const { trade, lat, lng, radius_km } = draft
  const ready = trade !== null && lat !== null && lng !== null
  const workers = useLoad(() => (ready ? api.listWorkers({ trade, lat, lng, radius_km }) : Promise.resolve([])), [trade, lat, lng, radius_km])

  if (!me) return null
  if (!ready) return (
    <Task screen="13-worker-list" wide>
      <div className="section-heading"><h1>Start with your problem</h1><p className="muted">Tell us what needs fixing and where you are.</p></div>
      <Link className="button full" href={trade ? '/app/find/location' : '/app/find'}><Icon name="ArrowRight" />Continue</Link>
    </Task>
  )
  const t = TRADES[trade]
  const s = workers.state

  return (
    <Task screen="13-worker-list" wide>
      {s.status === 'loading' ? (
        <>
          <StatePanel kind="loading" title="Checking availability" text="We only show workers who are free right now." />
          <Skeletons />
        </>
      ) : s.status === 'error' ? (
        <>
          <StatePanel kind="error" title="Couldn’t load nearby workers" text="Your problem is saved on this screen. Retry to check current availability." />
          <button className="button full" type="button" onClick={workers.reload}><Icon name="RefreshCw" />Try again</button>
        </>
      ) : s.data.length === 0 ? (
        <>
          <div className="section-heading"><h1>No one free nearby</h1><p className="muted">Try a wider area. We only show workers who are available now.</p></div>
          <div className="empty-art"><Icon name="Search" /></div>
          {radius_km < 10 && (
            <button className="button full" type="button" onClick={() => setDraft(problemDraft.set({ radius_km: 10 }))}>
              <Icon name="Search" />Search within 10 km
            </button>
          )}
        </>
      ) : (
        <>
          <div className="section-heading"><h1>{t.en}s near you</h1><p className="muted">Only workers who are free right now</p></div>
          <div className="row between">
            <span className="row caption muted"><Icon name="MapPin" className="icon-sm" />Within {radius_km} km</span>
            <span className="caption muted">Nearest first</span>
          </div>
          {draft.price_range && (
            <section className="card card-soft">
              <div className="row between">
                <div><p className="caption muted">Typical {t.en} job</p><h3>{rupees(draft.price_range.low)}–{draft.price_range.high.toLocaleString('en-PK')}</h3></div>
                <Icon name="Banknote" />
              </div>
            </section>
          )}
          <div className="results-grid">
            {s.data.map((w) => (
              <Link key={w.id} className="worker-link" href={`/app/workers/${w.id}`}>
                <div className="row">
                  <span className="avatar">{initials(w.name)}</span>
                  <div className="grow"><h3>{w.name}</h3><p className="caption muted">{[t.en, w.area].filter(Boolean).join(' · ')}</p></div>
                  <Icon name="ChevronRight" />
                </div>
                <div className="worker-meta">
                  {w.distance_km !== null && <><Icon name="MapPin" /><span>{km(w.distance_km)} away</span><span>·</span></>}
                  <Icon name="Star" /><span>{w.rating_avg?.toFixed(1) ?? 'New'} · {w.jobs_done} jobs</span>
                </div>
                <div className="worker-footer"><Badge tone="success">Available now</Badge><span className="caption green">View profile</span></div>
              </Link>
            ))}
          </div>
        </>
      )}
    </Task>
  )
}
