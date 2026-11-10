import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Icon } from '../components/Icon'
import { JobAside, Task } from '../components/Task'
import { Alert, AudioMessage, Badge, JobCard, Keypad, Skeletons, StatePanel, Timeline } from '../components/ui'
import type { Job, Message } from '../contract'
import { api, isWorkerBusy, MOCK, type Me } from '../lib/api'
import { play, toast, useRecorder } from '../lib/audio'
import { MOCK_WORKER_ID } from '../lib/mocks'
import { Link, navigate, Redirect } from '../lib/router'
import { signIn, useLoad, useMe } from '../lib/session'
import { isActive, rupees } from '../lib/trades'

type Loaded = { job: Job; messages: Message[] }

/** Loads a job for whoever is signed in, with shared loading and error states. */
function JobPage({ id, screen, poll, children }: { id: number; screen: string; poll?: number; children: (data: Loaded, me: Me, reload: () => void) => ReactNode }) {
  const me = useMe()
  const job = useLoad(() => api.getJob(id), [id, me?.user.id], poll)
  if (!me) return <Redirect to={`/login?next=${encodeURIComponent(location.pathname)}`} />
  const worker = me.user.role === 'worker'
  if (job.state.status === 'loading') return <Task screen={screen} worker={worker}><Skeletons count={2} /></Task>
  if (job.state.status === 'error') return (
    <Task screen={screen} worker={worker}>
      <StatePanel kind="error" title={worker ? 'کام نہیں مل سکا' : 'Couldn’t load this job'} text={worker ? undefined : 'Check your connection and try again.'} />
      <button className="button full" type="button" onClick={job.reload}><Icon name="RefreshCw" />{worker ? 'دوبارہ کوشش' : 'Try again'}</button>
    </Task>
  )
  return <>{children(job.state.data, me, job.reload)}</>
}

function Bubble({ m, mine, worker }: { m: Message; mine: boolean; worker: boolean }) {
  const time = new Date(m.created_at).toLocaleTimeString('en-PK', { hour: '2-digit', minute: '2-digit' })
  if (worker && !mine) {
    const url = m.kind === 'voice' ? m.audio_url : m.speech_url
    return (
      <div className="bubble">
        <button className="icon-button" type="button" disabled={!url} aria-label={url ? 'گاہک کا پیغام سنیں' : 'آواز تیار ہو رہی ہے'}
          onClick={() => url && play(url).catch(() => toast('آواز ابھی دستیاب نہیں۔'))}>
          <Icon name={url ? 'Play' : 'LoaderCircle'} />
        </button>
        {m.body_ur ? <p className="urdu-message">{m.body_ur}</p> : m.body && <p dir="auto">{m.body}</p>}
        <time className="digits">{time}</time>
      </div>
    )
  }
  return (
    <div className={`bubble ${mine ? 'mine' : ''}`}>
      {m.kind === 'voice' && <AudioMessage url={m.audio_url} ur={worker} />}
      {m.body ? <p className="caption" dir="auto">{m.body}</p> : m.kind === 'voice' && !worker && <p className="caption muted">Transcript is processing</p>}
      <time className="digits">{time}</time>
    </div>
  )
}

function Offer({ m, job, me, onDone }: { m: Message; job: Job; me: Me; onDone: (e?: unknown) => void }) {
  const worker = me.user.role === 'worker'
  const mine = m.sender_id === me.user.id
  const [busy, setBusy] = useState(false)
  const first = job.worker.name.split(' ').slice(-1)[0]
  async function answer(accept: boolean) {
    setBusy(true)
    try {
      if (accept) {
        await api.acceptOffer(job.id, m.id)
        return navigate(`/app/jobs/${job.id}/status`)
      }
      await api.rejectOffer(job.id, m.id)
      onDone()
    } catch (e) {
      onDone(e)
    } finally {
      setBusy(false)
    }
  }
  const status = { accepted: worker ? 'منظور' : 'Accepted', rejected: worker ? 'نامنظور' : 'Declined', pending: worker ? 'جواب کا انتظار' : 'Waiting for reply' }
  return (
    <div className="offer">
      <p className="muted">{mine ? (worker ? 'آپ کی پیشکش' : 'Your offer') : worker ? 'گاہک کی پیشکش' : `${first}’s price offer`}</p>
      <p className="price">{rupees(m.offer_price ?? 0)}</p>
      {m.offer_status === 'pending' && !mine && job.status === 'requested' ? (
        <div className="offer-actions">
          <button className="button" type="button" disabled={busy} onClick={() => answer(true)}><Icon name="Check" />{worker ? 'منظور' : 'Accept price'}</button>
          <button className="button destructive-outline" type="button" disabled={busy} onClick={() => answer(false)}><Icon name="X" />{worker ? 'نہیں' : 'Decline'}</button>
        </div>
      ) : m.offer_status && (
        <Badge tone={m.offer_status === 'accepted' ? 'success' : m.offer_status === 'rejected' ? 'error' : 'warning'}>{status[m.offer_status]}</Badge>
      )}
    </div>
  )
}

export function Chat({ id }: { id: number }) {
  const [text, setText] = useState('')
  const [sendError, setSendError] = useState(false)
  const [busy, setBusy] = useState<string | null>(null)
  const rec = useRecorder((clip) => {
    toast('Sending voice note…')
    api.sendVoice(id, clip).then(() => setSendError(false), () => setSendError(true))
  })
  const role = useMe()?.user.role
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if (role === 'worker') rec.prepare() }, [role])

  return (
    <JobPage id={id} screen="15-customer-chat" poll={4000}>
      {({ job, messages }, me, reload) => {
        const worker = me.user.role === 'worker'
        const first = job.worker.name.split(' ').slice(-1)[0]
        if (busy) return (
          <Task screen="15-customer-chat" worker={worker}>
            <div className="section-heading">
              <h1>{worker ? 'آپ پہلے سے ایک کام پر ہیں' : `${first} took another job`}</h1>
              {!worker && <p className="muted">This offer could not be accepted. Your request is not confirmed.</p>}
            </div>
            <Alert title={worker ? 'یہ کام پکا نہیں ہوا' : 'No booking was created'} />
            <Link className="button full" href={worker ? '/app' : '/app/workers'}><Icon name="Search" />{worker ? 'واپس' : 'Find another worker'}</Link>
          </Task>
        )
        const open = isActive(job.status)
        async function send(e: FormEvent) {
          e.preventDefault()
          if (!text.trim()) return
          try {
            await api.sendText(id, text.trim())
            setText('')
            setSendError(false)
            reload()
          } catch {
            setSendError(true)
          }
        }
        const recording = rec.state === 'recording'
        const mic = (
          <button className={`mic ${recording ? 'recording' : ''}`} type="button" onClick={rec.toggle} aria-pressed={recording}
            aria-label={recording ? (worker ? 'پیغام بھیجیں' : 'Stop and send voice note') : worker ? 'بول کر پیغام دیں' : 'Record a voice note'}>
            <Icon name="Mic" />
          </button>
        )
        const offerLink = job.status === 'requested' && (
          <Link className="icon-button" aria-label={worker ? 'قیمت دیں' : 'Make price offer'} href={`/app/jobs/${id}/offer`}><Icon name="Banknote" /></Link>
        )
        return (
          <Task screen={worker ? '16-worker-chat' : '15-customer-chat'} worker={worker} prompt="chat" aside={<JobAside job={job} worker={worker} />}>
            <Timeline status={job.status} ur={worker} />
            <JobCard job={job} worker={worker} />
            {job.status !== 'requested' && (
              <Link className="button outline full" href={`/app/jobs/${id}/status`}>
                <Icon name="BriefcaseBusiness" />{worker ? 'کام دیکھیں' : 'View job'}
              </Link>
            )}
            {messages.map((m) => m.kind === 'offer'
              ? <Offer key={m.id} m={m} job={job} me={me} onDone={(e) => {
                  if (isWorkerBusy(e)) setBusy('busy')
                  else if (e) toast(worker ? 'دوبارہ کوشش کریں' : 'That didn’t go through. Try again.')
                  else reload()
                }} />
              : <Bubble key={m.id} m={m} mine={m.sender_id === me.user.id} worker={worker} />)}
            {sendError && <Alert tone="error" title={worker ? 'پیغام نہیں گیا' : 'Message not sent'} text={worker ? undefined : 'Your message is kept. Try again.'} />}
            {rec.state === 'denied' && <Alert tone="error" title={worker ? 'مائیک کی اجازت دیں' : 'Microphone blocked'} />}
            {open ? (
              worker ? (
                <div className="chat-composer worker-composer">{offerLink}{mic}</div>
              ) : (
                <form className="chat-composer" onSubmit={send}>
                  {offerLink}
                  <input aria-label="Message" placeholder="Type a message…" value={text} onChange={(e) => setText(e.target.value)} />
                  {text.trim() ? <button className="icon-button" type="submit" aria-label="Send message"><Icon name="Send" /></button> : mic}
                </form>
              )
            ) : <p className="note center">{worker ? 'یہ کام بند ہو چکا ہے' : 'This job is closed. Messages are read-only.'}</p>}
          </Task>
        )
      }}
    </JobPage>
  )
}

export function MakeOffer({ id }: { id: number }) {
  const [value, setValue] = useState('')
  const [state, setState] = useState<'idle' | 'sending' | 'error'>('idle')
  return (
    <JobPage id={id} screen="17-customer-offer">
      {({ job }, me) => {
        const worker = me.user.role === 'worker'
        if (job.status !== 'requested') return (
          <Task screen="17-customer-offer" worker={worker}>
            <StatePanel kind="success" title={worker ? 'قیمت طے ہو چکی ہے' : 'The price is already agreed'} />
            <Link className="button full" href={`/app/jobs/${id}`}><Icon name="MessageCircle" />{worker ? 'بات کھولیں' : 'Open chat'}</Link>
          </Task>
        )
        async function send() {
          setState('sending')
          try {
            await api.sendOffer(id, Number(value))
            navigate(`/app/jobs/${id}`)
          } catch {
            setState('error')
          }
        }
        return (
          <Task screen={worker ? '18-worker-offer' : '17-customer-offer'} worker={worker} prompt="offer" aside={<JobAside job={job} worker={worker} />}>
            <div className="section-heading">
              <h1>{worker ? 'قیمت بتائیں' : 'Offer a price'}</h1>
              {!worker && <p className="muted">Agree on the scope in chat. The other person must accept.</p>}
            </div>
            <section className="card card-soft">
              <p className="muted">{worker ? 'روپے' : 'Pakistani rupees'}</p>
              <p className="price" aria-live="polite">{rupees(Number(value || 0))}</p>
            </section>
            <Keypad onChange={setValue} />
            {state === 'error' && <Alert tone="error" title={worker ? 'پیشکش نہیں گئی' : 'Offer not sent'} text={worker ? undefined : 'Your amount is kept. Try again.'} />}
            <button className="button full" type="button" onClick={send} disabled={!Number(value) || state === 'sending'}>
              <Icon name={state === 'sending' ? 'LoaderCircle' : 'Send'} />{worker ? 'پیشکش بھیجیں' : 'Send offer'}
            </button>
            {!worker && <p className="note">Sending an offer does not reserve the worker.</p>}
          </Task>
        )
      }}
    </JobPage>
  )
}

function DemoSwitch({ job, to, label, text }: { job: Job; to: 'worker' | 'customer'; label: string; text: string }) {
  if (!MOCK || job.worker.id !== MOCK_WORKER_ID) return null
  return (
    <div className="demo-step" dir="ltr">
      <p className="eyebrow">Demo mode</p>
      <p>{text}</p>
      <button className="button outline" type="button" onClick={async () => {
        await signIn(to)
        navigate(to === 'worker' ? `/app/jobs/${job.id}/status` : `/app/jobs/${job.id}/review`)
      }}>{label}</button>
    </div>
  )
}

export function JobStatus({ id }: { id: number }) {
  const [confirming, setConfirming] = useState(false)
  const [state, setState] = useState<'idle' | 'saving' | 'error'>('idle')
  return (
    <JobPage id={id} screen="19-customer-job" poll={5000}>
      {({ job }, me, reload) => {
        const worker = me.user.role === 'worker'
        const first = job.worker.name.split(' ').slice(-1)[0]
        async function move(status: 'in_progress' | 'completed') {
          setState('saving')
          try {
            await api.setStatus(id, status)
            setConfirming(false)
            setState('idle')
            reload()
          } catch {
            setState('error')
          }
        }
        const price = job.agreed_price !== null && (
          <section className="card card-accent"><p className="muted">{worker ? 'طے شدہ قیمت' : 'Agreed price'}</p><p className="price">{rupees(job.agreed_price)}</p></section>
        )
        const chat = <Link className={`button ${worker ? 'outline' : ''} full`} href={`/app/jobs/${id}`}><Icon name="MessageCircle" />{worker ? 'بات کھولیں' : 'Open chat'}</Link>
        const cancel = isActive(job.status) && (
          <Link className={`button ${worker ? 'ghost' : 'outline full'}`} href={`/app/jobs/${id}/cancel`}><Icon name="X" />{worker ? 'منسوخ' : 'Cancel job'}</Link>
        )
        const saveError = state === 'error' && <Alert tone="error" title={worker ? 'دوبارہ کوشش کریں' : 'That didn’t go through. Try again.'} />

        if (job.status === 'cancelled') return (
          <Task screen={worker ? '27-worker-cancel' : '26-customer-cancel'} worker={worker}>
            <StatePanel kind="error" icon="X" title={worker ? 'کام منسوخ ہو گیا' : 'This job was cancelled'} text={worker ? undefined : 'No work will take place. You can request help again.'} />
            <Link className="button full" href={worker ? '/app' : '/app/find'}><Icon name="ArrowRight" />{worker ? 'واپس' : 'Find help again'}</Link>
          </Task>
        )

        if (worker) {
          const heading = job.status === 'requested' ? 'قیمت طے کریں' : job.status === 'confirmed' ? 'کام پکا ہو گیا'
            : job.status === 'in_progress' ? (confirming ? 'کام مکمل ہو گیا؟' : 'کام جاری ہے') : 'کام مکمل ہو گیا'
          return (
            <Task screen={job.status === 'in_progress' ? '21-worker-complete' : '20-worker-job'} worker prompt="done"
              aside={<JobAside job={job} worker>{job.status === 'completed' && !job.reviewed &&
                <DemoSwitch job={job} to="customer" label="Continue as customer → Leave a review" text="The customer reviews the completed job." />}</JobAside>}>
              <Timeline status={job.status} ur />
              <div className="section-heading"><h1>{heading}</h1></div>
              {job.status === 'completed' && <StatePanel kind="success" title="شکریہ" />}
              <JobCard job={job} worker />
              {price}
              {saveError}
              {job.status === 'confirmed' && (
                <button className="button full" type="button" disabled={state === 'saving'} onClick={() => move('in_progress')}><Icon name="Play" />کام شروع کریں</button>
              )}
              {job.status === 'in_progress' && (confirming ? (
                <>
                  <button className="button full" type="button" disabled={state === 'saving'} onClick={() => move('completed')}><Icon name="Check" />ہاں، مکمل</button>
                  <button className="button outline full" type="button" onClick={() => setConfirming(false)}><Icon name="ArrowRight" />ابھی نہیں</button>
                </>
              ) : <button className="button full" type="button" onClick={() => setConfirming(true)}><Icon name="CircleCheck" />کام مکمل</button>)}
              {job.status === 'completed' ? <Link className="button outline full" href="/app"><Icon name="ArrowRight" />واپس</Link> : chat}
              {!confirming && cancel}
            </Task>
          )
        }

        const heading = job.status === 'requested' ? 'Agree a price to book' : job.status === 'confirmed' ? `Booked with ${first}`
          : job.status === 'in_progress' ? 'Work is in progress' : 'Job complete'
        return (
          <Task screen="19-customer-job" aside={<JobAside job={job} worker={false}>{(job.status === 'confirmed' || job.status === 'in_progress') &&
            <DemoSwitch job={job} to="worker" label="View worker’s next step" text="The worker starts and completes the job. Switch roles to see that step." />}</JobAside>}>
            <Timeline status={job.status} />
            <div className="section-heading">
              <h1>{heading}</h1>
              <p className="muted">{job.status === 'requested' ? 'Nothing is booked until one of you accepts the other’s price offer.' : 'Your agreed scope and price stay in the chat.'}</p>
            </div>
            <JobCard job={job} />
            {price}
            {job.status === 'completed' && (job.reviewed
              ? <Alert tone="success" title="Thank you for your review" />
              : <Link className="button full" href={`/app/jobs/${id}/review`}><Icon name="Star" />Leave a review</Link>)}
            {chat}
            {cancel}
          </Task>
        )
      }}
    </JobPage>
  )
}

export function CancelJob({ id }: { id: number }) {
  const [state, setState] = useState<'idle' | 'saving' | 'error'>('idle')
  return (
    <JobPage id={id} screen="26-customer-cancel">
      {({ job }, me) => {
        const worker = me.user.role === 'worker'
        if (!isActive(job.status)) return <Redirect to={`/app/jobs/${id}/status`} />
        async function cancel() {
          setState('saving')
          try {
            await api.setStatus(id, 'cancelled')
            navigate(`/app/jobs/${id}/status`, true)
          } catch {
            setState('error')
          }
        }
        return (
          <Task screen={worker ? '27-worker-cancel' : '26-customer-cancel'} worker={worker}>
            <div className="section-heading">
              <h1>{worker ? 'کام منسوخ کریں؟' : 'Cancel this job?'}</h1>
              {!worker && <p className="muted">The other person will see the cancellation. You can request help again.</p>}
            </div>
            <JobCard job={job} worker={worker} />
            {state === 'error' && <Alert tone="error" title={worker ? 'دوبارہ کوشش کریں' : 'The job was not cancelled. Try again.'} />}
            <button className="button danger full" type="button" onClick={cancel} disabled={state === 'saving'}><Icon name="X" />{worker ? 'منسوخ کریں' : 'Yes, cancel job'}</button>
            <Link className="button outline full" href={`/app/jobs/${id}/status`}><Icon name="ArrowLeft" />{worker ? 'واپس' : 'Keep this job'}</Link>
          </Task>
        )
      }}
    </JobPage>
  )
}

const RATING = ['', 'Poor', 'Fair', 'Good', 'Very good', 'Excellent']

export function ReviewJob({ id }: { id: number }) {
  const [rating, setRating] = useState(0)
  const [body, setBody] = useState('')
  const [clip, setClip] = useState<Blob | null>(null)
  const [state, setState] = useState<'idle' | 'saving' | 'error' | 'done'>('idle')
  const rec = useRecorder(setClip)
  return (
    <JobPage id={id} screen="22-customer-review">
      {({ job }, me) => {
        if (me.user.role !== 'customer' || job.customer.id !== me.user.id) return <Redirect to={`/app/jobs/${id}/status`} />
        if (state === 'done' || job.reviewed) return (
          <Task screen="22-customer-review" aside={<JobAside job={job} worker={false} />}>
            <StatePanel kind="success" title="Thank you for your review" text="Your experience will help the next customer. Review tags and summary may take a moment." />
            <Link className="button full" href="/app/jobs"><Icon name="ArrowRight" />Continue</Link>
          </Task>
        )
        if (job.status !== 'completed') return (
          <Task screen="22-customer-review" aside={<JobAside job={job} worker={false} />}>
            <StatePanel kind="loading" icon="Clock" title="Reviews open after the job is complete" text="The worker marks the job complete when the work is done." />
            <Link className="button full" href={`/app/jobs/${id}/status`}><Icon name="ArrowLeft" />Back to job</Link>
          </Task>
        )
        async function submit() {
          setState('saving')
          try {
            await api.review(id, { rating, body: body.trim() || undefined, audio: clip ?? undefined })
            setState('done')
          } catch {
            setState('error')
          }
        }
        const recording = rec.state === 'recording'
        return (
          <Task screen="22-customer-review" aside={<JobAside job={job} worker={false} />}>
            <Timeline status={job.status} />
            <div className="section-heading"><h1>Rate {job.worker.name.split(' ').slice(-1)[0]}’s work</h1><p className="muted">Stars are required. A voice note or written review is optional.</p></div>
            <div className="review-stars" role="group" aria-label="Rating">
              {[1, 2, 3, 4, 5].map((n) => (
                <button key={n} type="button" className={n <= rating ? 'selected' : ''} aria-pressed={n <= rating} aria-label={`${n} star rating`} onClick={() => setRating(n)}>
                  <Icon name="Star" />
                </button>
              ))}
            </div>
            <p className="center caption muted">{rating ? `${rating} out of 5 · ${RATING[rating]}` : 'Choose a rating'}</p>
            <div className="mic-zone">
              <button className={`mic ${recording ? 'recording' : ''}`} type="button" onClick={rec.toggle} aria-pressed={recording}
                aria-label={recording ? 'Stop recording' : 'Record a voice review'}><Icon name="Mic" /></button>
              <p className="caption muted">{recording ? 'Recording… tap again to finish' : clip ? 'Voice review added · tap to record again' : 'Tell us in your own words · optional'}</p>
            </div>
            <label className="field" htmlFor="review">Or write a review
              <textarea id="review" placeholder="What should the next customer know?" value={body} onChange={(e) => setBody(e.target.value)} />
            </label>
            {state === 'error' && <Alert tone="error" title="Review not sent" text="Your rating and words are kept. Try again." />}
            <button className="button full" type="button" onClick={submit} disabled={!rating || state === 'saving'}>
              <Icon name={state === 'saving' ? 'LoaderCircle' : 'Check'} />Submit review
            </button>
            <p className="note center">Only reviews from completed jobs appear on profiles.</p>
          </Task>
        )
      }}
    </JobPage>
  )
}
