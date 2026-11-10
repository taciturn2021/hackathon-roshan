import { useState } from 'react'
import { Icon } from '../components/Icon'
import { Task } from '../components/Task'
import { Alert, Badge, Skeletons, StatePanel, TradeGrid, TradeIcon } from '../components/ui'
import type { Problem as Analysis, Trade, Urgency } from '../contract'
import { api } from '../lib/api'
import { MOCK_LOCATION } from '../lib/mocks'
import { useRecorder } from '../lib/audio'
import { locate, problemDraft } from '../lib/draft'
import { useT } from '../lib/i18n'
import { Link, navigate } from '../lib/router'
import { useLoad, useRole } from '../lib/session'
import { initials, km, rupees, TRADES, URGENCY } from '../lib/trades'

function applyProblem(p: Analysis) {
  problemDraft.set({ description: p.description, trade: p.trade, trade_confidence: p.trade_confidence, urgency: p.urgency, price_range: p.price_range })
  navigate('/app/find/confirm')
}

export function Problem() {
  const me = useRole('customer')
  const t = useT()
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
    <Task screen="11-problem"><StatePanel kind="loading" title={t('Finding the right trade', 'صحیح کاریگر ڈھونڈ رہے ہیں')} text={t('Your description is saved while this finishes.', 'آپ کی بات محفوظ ہے۔ ذرا انتظار کریں۔')} /></Task>
  )

  return (
    <Task screen="11-problem">
      <div className="section-heading"><h1>{t('What needs fixing?', 'کیا ٹھیک کروانا ہے؟')}</h1><p className="muted">{t('Tell us in English, Urdu or Roman Urdu.', 'اردو، انگریزی یا رومن اردو میں بتائیں۔')}</p></div>
      {state === 'error' && <Alert tone="error" title={t('We couldn’t read that', 'ہم سمجھ نہیں سکے')} text={t('Your description is saved. Try again, or choose a trade below.', 'آپ کی بات محفوظ ہے۔ دوبارہ کوشش کریں، یا نیچے سے کام چنیں۔')} />}
      {rec.state === 'denied' && <Alert tone="error" title={t('Microphone blocked', 'مائیک بند ہے')} text={t('Allow microphone access in your browser settings, or type instead.', 'براؤزر کی سیٹنگ میں مائیک کی اجازت دیں، یا لکھ کر بتائیں۔')} />}
      <div className="stack">
        <label className="field" htmlFor="problem">{t('Describe the problem', 'مسئلہ بتائیں')}
          <textarea id="problem" placeholder={t('For example: My AC is running but not cooling.', 'مثلاً: میرا اے سی چل رہا ہے مگر ٹھنڈا نہیں کر رہا۔')} value={text}
            aria-invalid={empty} aria-describedby={empty ? 'problem-error' : undefined}
            onChange={(e) => { setText(e.target.value); setEmpty(false); problemDraft.set({ description: e.target.value }) }} />
          {empty && <span className="field-error" id="problem-error">{t('Describe the problem, or hold the mic and speak.', 'مسئلہ لکھیں، یا مائیک دبا کر بولیں۔')}</span>}
        </label>
        <div className="row between">
          <span className="note">{recording ? t('Recording… tap again to finish', 'ریکارڈنگ ہو رہی ہے… ختم کرنے کے لیے دوبارہ دبائیں') : t('Or tap the mic and speak', 'یا مائیک دبا کر بولیں')}</span>
          <button className={`mic ${recording ? 'recording' : ''}`} type="button" aria-pressed={recording}
            aria-label={recording ? t('Stop recording', 'ریکارڈنگ روکیں') : t('Record your problem', 'اپنا مسئلہ ریکارڈ کریں')} onClick={rec.toggle}><Icon name="Mic" /></button>
        </div>
      </div>
      <button className="button full" type="button" onClick={() => (text.trim() ? analyze({ text: text.trim() }) : setEmpty(true))}>
        <Icon name="ArrowRight" />{t('Find the right trade', 'صحیح کاریگر ڈھونڈیں')}
      </button>
      <div className="divider" />
      <div className="stack-sm">
        <h2>{t('Know who you need?', 'معلوم ہے کون چاہیے؟')}</h2>
        <TradeGrid onPick={(trade) => { problemDraft.set({ trade, trade_confidence: 1, price_range: null }); navigate('/app/find/confirm') }} />
      </div>
    </Task>
  )
}

export function ProblemConfirm() {
  const me = useRole('customer')
  const t = useT()
  const [draft, setDraft] = useState(problemDraft.get())
  const [picking, setPicking] = useState(draft.trade === null || draft.trade_confidence < 0.6)
  const update = (patch: Partial<typeof draft>) => setDraft(problemDraft.set(patch))

  if (!me) return null
  if (picking || !draft.trade) return (
    <Task screen="12-problem-confirm">
      <div className="section-heading"><h1>{t('Which trade do you need?', 'آپ کو کون سا کاریگر چاہیے؟')}</h1><p className="muted">{t('We’re unsure. Choose the closest match.', 'ہمیں پکا نہیں پتا۔ سب سے ملتا جلتا کام چنیں۔')}</p></div>
      <TradeGrid selected={draft.trade} onPick={(trade: Trade) => { update({ trade, trade_confidence: 1, price_range: null }); setPicking(false) }} />
    </Task>
  )
  const tr = TRADES[draft.trade]
  const range = draft.price_range
  return (
    <Task screen="12-problem-confirm">
      <div className="section-heading"><h1>{t('Check your request', 'اپنی درخواست دیکھ لیں')}</h1><p className="muted">{t('Check the details before we look nearby.', 'قریب کاریگر ڈھونڈنے سے پہلے تفصیل دیکھ لیں۔')}</p></div>
      <section className="card">
        <div className="row">
          <TradeIcon trade={draft.trade} />
          <div className="grow"><h2>{t(tr.en, `${tr.ur} کا کام`)}</h2><p className="caption muted">{draft.description ? t('Based on your description', 'آپ کی بتائی ہوئی بات کے مطابق') : t('Your choice', 'آپ کا انتخاب')}</p></div>
          <button className="icon-button" type="button" aria-label={t('Change trade', 'کام بدلیں')} onClick={() => setPicking(true)}><Icon name="Pencil" /></button>
        </div>
        {draft.description && <p dir="auto">{draft.description}</p>}
      </section>
      <div className="stack-sm">
        <h3>{t('How urgent is it?', 'کتنی جلدی چاہیے؟')}</h3>
        <div className="segment" role="radiogroup" aria-label={t('Urgency', 'کتنی جلدی')}>
          {(Object.keys(URGENCY) as Urgency[]).map((u) => (
            <button key={u} type="button" role="radio" aria-checked={draft.urgency === u}
              className={`button outline ${draft.urgency === u ? 'selected' : ''}`} onClick={() => update({ urgency: u })}>{t(URGENCY[u].en, URGENCY[u].ur)}</button>
          ))}
        </div>
        <p className="note">{t('Urgency helps explain your request. It does not schedule a later booking.', 'اس سے کاریگر کو پتا چلتا ہے کہ کام کتنا جلدی کا ہے۔ اس سے بعد کی بکنگ نہیں ہوتی۔')}</p>
      </div>
      <section className="card card-soft">
        {range ? (
          <>
            <p className="caption muted">{t(`Typical completed ${tr.en} jobs`, `${tr.ur} کے مکمل کاموں کی عام قیمت`)}</p>
            <p className="price">{rupees(range.low)}–{range.high.toLocaleString('en-PK')}</p>
            <p className="note">{t(`Based on ${range.sample} completed jobs. A guide, not a quote. Agree your final price in chat.`, `${range.sample} مکمل کاموں کے مطابق۔ یہ صرف اندازہ ہے، پکی قیمت نہیں۔ اصل قیمت چیٹ میں طے کریں۔`)}</p>
          </>
        ) : <p className="note">{t('No price guide yet for this trade. Agree your price in chat.', 'اس کام کی قیمت کا اندازہ ابھی موجود نہیں۔ قیمت چیٹ میں طے کریں۔')}</p>}
      </section>
      <Link className="button full" href="/app/find/location"><Icon name="Search" />{t('Find available workers', 'دستیاب کاریگر ڈھونڈیں')}</Link>
    </Task>
  )
}

export function CustomerLocation() {
  const me = useRole('customer')
  const t = useT()
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
      <div className="section-heading"><h1>{t('Share your location', 'اپنی لوکیشن بتائیں')}</h1><p className="muted">{t('Share your location to find available workers around you.', 'لوکیشن بتائیں تاکہ آپ کے قریب دستیاب کاریگر مل سکیں۔')}</p></div>
      {state === 'error'
        ? <StatePanel kind="error" title={t('We couldn’t get your location', 'آپ کی لوکیشن نہیں مل سکی')} text={t('Allow location access in your browser, then try again. We never guess where you are.', 'براؤزر میں لوکیشن کی اجازت دیں، پھر دوبارہ کوشش کریں۔ ہم آپ کی جگہ کا اندازہ خود نہیں لگاتے۔')} />
        : <div className="state-panel"><div className="empty-art"><Icon name="LocateFixed" /></div></div>}
      <p className="note">{t('Your location is used for nearby matching. This is not live worker tracking.', 'آپ کی لوکیشن صرف قریبی کاریگر ڈھونڈنے کے لیے ہے۔ یہ کاریگر کی لائیو ٹریکنگ نہیں ہے۔')}</p>
      <button className="button full" type="button" onClick={() => go()} disabled={state === 'loading'}>
        <Icon name={state === 'loading' ? 'LoaderCircle' : state === 'error' ? 'RefreshCw' : 'LocateFixed'} />
        {state === 'error' ? t('Try again', 'دوبارہ کوشش') : t('Use my location', 'میری لوکیشن استعمال کریں')}
      </button>
      <button className="button outline full" type="button" onClick={() => go(true)} disabled={state === 'loading'}>{t('Use G-10, Islamabad instead', 'اس کی جگہ G-10، اسلام آباد استعمال کریں')}</button>
    </Task>
  )
}

export function WorkerList() {
  const me = useRole('customer')
  const t = useT()
  const [draft, setDraft] = useState(problemDraft.get())
  const { trade, lat, lng, radius_km } = draft
  const ready = trade !== null && lat !== null && lng !== null
  const workers = useLoad(() => (ready ? api.listWorkers({ trade, lat, lng, radius_km }) : Promise.resolve([])), [trade, lat, lng, radius_km])

  if (!me) return null
  if (!ready) return (
    <Task screen="13-worker-list" wide>
      <div className="section-heading"><h1>{t('Start with your problem', 'پہلے اپنا مسئلہ بتائیں')}</h1><p className="muted">{t('Tell us what needs fixing and where you are.', 'بتائیں کیا ٹھیک کروانا ہے اور آپ کہاں ہیں۔')}</p></div>
      <Link className="button full" href={trade ? '/app/find/location' : '/app/find'}><Icon name="ArrowRight" />{t('Continue', 'آگے بڑھیں')}</Link>
    </Task>
  )
  const tr = TRADES[trade]
  const s = workers.state

  return (
    <Task screen="13-worker-list" wide>
      {s.status === 'loading' ? (
        <>
          <StatePanel kind="loading" title={t('Checking availability', 'دستیاب کاریگر دیکھ رہے ہیں')} text={t('We only show workers who are free right now.', 'ہم صرف وہ کاریگر دکھاتے ہیں جو ابھی فارغ ہیں۔')} />
          <Skeletons />
        </>
      ) : s.status === 'error' ? (
        <>
          <StatePanel kind="error" title={t('Couldn’t load nearby workers', 'قریبی کاریگر نہیں مل سکے')} text={t('Your problem is saved on this screen. Retry to check current availability.', 'آپ کا مسئلہ محفوظ ہے۔ دوبارہ کوشش کریں تاکہ دستیاب کاریگر نظر آئیں۔')} />
          <button className="button full" type="button" onClick={workers.reload}><Icon name="RefreshCw" />{t('Try again', 'دوبارہ کوشش')}</button>
        </>
      ) : s.data.length === 0 ? (
        <>
          <div className="section-heading"><h1>{t('No one free nearby', 'قریب کوئی فارغ نہیں')}</h1><p className="muted">{t('Try a wider area. We only show workers who are available now.', 'ذرا دور تک ڈھونڈیں۔ ہم صرف وہ کاریگر دکھاتے ہیں جو ابھی دستیاب ہیں۔')}</p></div>
          <div className="empty-art"><Icon name="Search" /></div>
          {radius_km < 10 && (
            <button className="button full" type="button" onClick={() => setDraft(problemDraft.set({ radius_km: 10 }))}>
              <Icon name="Search" />{t('Search within 10 km', '10 کلومیٹر تک ڈھونڈیں')}
            </button>
          )}
        </>
      ) : (
        <>
          <div className="section-heading"><h1>{t(`${tr.en}s near you`, `آپ کے قریب ${tr.ur} کے کاریگر`)}</h1><p className="muted">{t('Only workers who are free right now', 'صرف وہ کاریگر جو ابھی فارغ ہیں')}</p></div>
          <div className="row between">
            <span className="row caption muted"><Icon name="MapPin" className="icon-sm" />{t(`Within ${radius_km} km`, `${radius_km} کلومیٹر کے اندر`)}</span>
            <span className="caption muted">{t('Nearest first', 'سب سے قریب پہلے')}</span>
          </div>
          {draft.price_range && (
            <section className="card card-soft">
              <div className="row between">
                <div><p className="caption muted">{t(`Typical ${tr.en} job`, `${tr.ur} کے کام کی عام قیمت`)}</p><h3>{rupees(draft.price_range.low)}–{draft.price_range.high.toLocaleString('en-PK')}</h3></div>
                <Icon name="Banknote" />
              </div>
            </section>
          )}
          <div className="results-grid">
            {s.data.map((w) => (
              <Link key={w.id} className="worker-link" href={`/app/workers/${w.id}`}>
                <div className="row">
                  <span className="avatar">{initials(w.name)}</span>
                  <div className="grow"><h3>{w.name}</h3><p className="caption muted">{[t(tr.en, `${tr.ur} کا کام`), w.area].filter(Boolean).join(' · ')}</p></div>
                  <Icon name="ChevronRight" />
                </div>
                <div className="worker-meta">
                  {w.distance_km !== null && <><Icon name="MapPin" /><span>{t(`${km(w.distance_km)} away`, `${w.distance_km.toFixed(1)} کلومیٹر دور`)}</span><span>·</span></>}
                  <Icon name="Star" /><span>{w.rating_avg?.toFixed(1) ?? t('New', 'نیا')} · {t(`${w.jobs_done} jobs`, `${w.jobs_done} کام`)}</span>
                </div>
                <div className="worker-footer"><Badge tone="success">{t('Available now', 'ابھی دستیاب')}</Badge><span className="caption green">{t('View profile', 'پروفائل دیکھیں')}</span></div>
              </Link>
            ))}
          </div>
        </>
      )}
    </Task>
  )
}
