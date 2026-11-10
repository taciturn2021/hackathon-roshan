import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Hero } from '../components/Hero'
import { Icon } from '../components/Icon'
import { JobAside, Task } from '../components/Task'
import { Alert, AudioMessage, Badge, JobCard, Keypad, Skeletons, StatePanel, Timeline } from '../components/ui'
import type { Job, Message } from '../contract'
import { api, isWorkerBusy, MOCK, type Me } from '../lib/api'
import { play, toast, useRecorder } from '../lib/audio'
import { errorText } from '../lib/errors'
import { MOCK_WORKER_ID } from '../lib/mocks'
import { useT } from '../lib/i18n'
import { Link, navigate, Redirect } from '../lib/router'
import { signIn, useLoad, useMe } from '../lib/session'
import { isActive, rupees } from '../lib/trades'

type Loaded = { job: Job; messages: Message[] }

/** Loads a job for whoever is signed in, with shared loading and error states. */
function JobPage({ id, screen, poll, children }: { id: number; screen: string; poll?: number; children: (data: Loaded, me: Me, reload: () => void) => ReactNode }) {
  const me = useMe()
  const job = useLoad(() => api.getJob(id), [id, me?.user.id], poll)
  const t = useT()
  if (!me) return <Redirect to={`/login?next=${encodeURIComponent(location.pathname)}`} />
  const worker = me.user.role === 'worker'
  if (job.state.status === 'loading') return <Task screen={screen} worker={worker}><Skeletons count={2} /></Task>
  if (job.state.status === 'error') return (
    <Task screen={screen} worker={worker}>
      <StatePanel kind="error" title={t('Couldn’t load this job', 'کام نہیں مل سکا')} text={errorText(job.state.error, t)} />
      {(job.state.error as { status?: number }).status === 404
        ? <Link className="button full" href="/app/jobs"><Icon name="BriefcaseBusiness" />{t('See your jobs', 'اپنے کام دیکھیں')}</Link>
        : <button className="button full" type="button" onClick={job.reload}><Icon name="RefreshCw" />{t('Try again', 'دوبارہ کوشش کریں')}</button>}
    </Task>
  )
  return <>{children(job.state.data, me, job.reload)}</>
}

function Bubble({ m, mine, worker }: { m: Message; mine: boolean; worker: boolean }) {
  const t = useT()
  const time = new Date(m.created_at).toLocaleTimeString('en-PK', { hour: '2-digit', minute: '2-digit' })
  if (worker && !mine) {
    const url = m.kind === 'voice' ? m.audio_url : m.speech_url
    return (
      <div className="bubble">
        <button className="icon-button" type="button" disabled={!url} aria-label={url ? t('Listen to the customer’s message', 'گاہک کا پیغام سنیں') : t('Audio is being prepared', 'آواز تیار ہو رہی ہے')}
          onClick={() => url && play(url).catch(() => toast(t('Audio is not available yet.', 'آواز ابھی دستیاب نہیں۔')))}>
          <Icon name={url ? 'Play' : 'LoaderCircle'} />
        </button>
        {m.body_ur ? <p className="urdu-message">{m.body_ur}</p> : m.body && <p dir="auto">{m.body}</p>}
        <time className="digits">{time}</time>
      </div>
    )
  }
  return (
    <div className={`bubble ${mine ? 'mine' : ''}`}>
      {m.kind === 'voice' && <AudioMessage url={m.audio_url} />}
      {m.body ? <p className="caption" dir="auto">{m.body}</p> : m.kind === 'voice' && !worker && <p className="caption muted">{t('Transcript is processing', 'آواز کو لکھا جا رہا ہے')}</p>}
      <time className="digits">{time}</time>
    </div>
  )
}

function Offer({ m, job, me, onDone }: { m: Message; job: Job; me: Me; onDone: (e?: unknown) => void }) {
  const t = useT()
  const worker = me.user.role === 'worker'
  const mine = m.sender_id === me.user.id
  const [busy, setBusy] = useState(false)
  const first = job.worker.name
  async function answer(accept: boolean) {
    setBusy(true)
    try {
      if (accept) {
        await api.acceptOffer(job.id, m.id)
        toast(t('Price accepted. The job is confirmed.', 'قیمت منظور۔ کام پکا ہو گیا۔'))
        return navigate(`/app/jobs/${job.id}/status`)
      }
      await api.rejectOffer(job.id, m.id)
      toast(t('Offer declined', 'پیشکش نامنظور کر دی'))
      onDone()
    } catch (e) {
      onDone(e)
    } finally {
      setBusy(false)
    }
  }
  const status = { accepted: t('Accepted', 'منظور'), rejected: t('Declined', 'نامنظور'), pending: t('Waiting for reply', 'جواب کا انتظار') }
  return (
    <div className="offer">
      <p className="muted">{mine ? t('Your offer', 'آپ کی پیشکش') : worker ? t('Customer’s offer', 'گاہک کی پیشکش') : t(`${first}’s price offer`, `${first} کی قیمت کی پیشکش`)}</p>
      <p className="price">{rupees(m.offer_price ?? 0)}</p>
      {m.offer_status === 'pending' && !mine && job.status === 'requested' ? (
        <div className="offer-actions">
          <button className="button" type="button" disabled={busy} onClick={() => answer(true)}><Icon name="Check" />{t('Accept price', 'قیمت منظور')}</button>
          <button className="button destructive-outline" type="button" disabled={busy} onClick={() => answer(false)}><Icon name="X" />{t('Decline', 'نہیں')}</button>
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
  const t = useT()
  const rec = useRecorder((clip) => {
    toast(t('Sending voice note…', 'آواز بھیجی جا رہی ہے…'))
    api.sendVoice(id, clip).then(() => { setSendError(false); toast(t('Voice note sent', 'آواز چلی گئی')) }, () => setSendError(true))
  })
  const role = useMe()?.user.role
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if (role === 'worker') rec.prepare() }, [role])

  return (
    <JobPage id={id} screen="15-customer-chat" poll={4000}>
      {({ job, messages }, me, reload) => {
        const worker = me.user.role === 'worker'
        const first = job.worker.name
        if (busy) return (
          <Task screen="15-customer-chat" worker={worker}>
            <div className="section-heading">
              <h1>{worker ? t('You are already on a job', 'آپ پہلے سے ایک کام پر ہیں') : t(`${first} took another job`, `${first} نے دوسرا کام لے لیا`)}</h1>
              {!worker && <p className="muted">{t('This offer could not be accepted. Your request is not confirmed.', 'یہ پیشکش منظور نہیں ہو سکی۔ آپ کی درخواست پکی نہیں ہوئی۔')}</p>}
            </div>
            <Alert title={t('No booking was created', 'یہ کام پکا نہیں ہوا')} />
            <Link className="button full" href={worker ? '/app' : '/app/workers'}><Icon name="Search" />{worker ? t('Back', 'واپس') : t('Find another worker', 'دوسرا کاریگر ڈھونڈیں')}</Link>
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
            toast(t('Message sent', 'پیغام چلا گیا'))
            reload()
          } catch {
            setSendError(true)
          }
        }
        const recording = rec.state === 'recording'
        const mic = (
          <button className={`mic ${recording ? 'recording' : ''}`} type="button" onClick={rec.toggle} aria-pressed={recording}
            aria-label={recording ? t('Stop and send voice note', 'روکیں اور آواز بھیجیں') : t('Record a voice note', 'بول کر پیغام دیں')}>
            <Icon name="Mic" />
          </button>
        )
        const offerLink = job.status === 'requested' && (
          <Link className="icon-button" aria-label={t('Make price offer', 'قیمت بھیجیں')} href={`/app/jobs/${id}/offer`}><Icon name="Banknote" /></Link>
        )
        return (
          <Task screen={worker ? '16-worker-chat' : '15-customer-chat'} worker={worker} prompt="chat" aside={<JobAside job={job} worker={worker} />}>
            <Timeline status={job.status} />
            {job.status === 'requested' && (worker
              ? <Hero tone="wait" icon="Banknote" title={t('Send your price', 'اپنی قیمت بھیجیں')}
                  hint={t('Press the money button to type a price. The job is confirmed when the customer accepts.', 'قیمت لکھنے کے لیے پیسوں والا بٹن دبائیں۔ گاہک منظور کرے تو کام پکا۔')} />
              : <Hero tone="wait" icon="Banknote" title={t('Agree a price', 'قیمت طے کریں')}
                  hint={t('Chat about the work, then accept the worker’s offer or send your own.', 'کام کے بارے میں بات کریں، پھر کاریگر کی پیشکش منظور کریں یا اپنی قیمت بھیجیں۔')} />)}
            <JobCard job={job} worker={worker} />
            {job.status !== 'requested' && (
              <Link className="button outline full" href={`/app/jobs/${id}/status`}>
                <Icon name="BriefcaseBusiness" />{t('View job', 'کام دیکھیں')}
              </Link>
            )}
            {messages.map((m) => m.kind === 'offer'
              ? <Offer key={m.id} m={m} job={job} me={me} onDone={(e) => {
                  if (isWorkerBusy(e)) setBusy('busy')
                  else if (e) toast(t('That didn’t go through. Try again.', 'نہیں ہو سکا۔ دوبارہ کوشش کریں۔'))
                  else reload()
                }} />
              : <Bubble key={m.id} m={m} mine={m.sender_id === me.user.id} worker={worker} />)}
            {sendError && <Alert tone="error" title={t('Message not sent', 'پیغام نہیں گیا')} text={worker ? t('Try again.', 'دوبارہ کوشش کریں۔') : t('Your message is kept. Try again.', 'آپ کا پیغام محفوظ ہے۔ دوبارہ کوشش کریں۔')} />}
            {rec.state === 'denied' && <Alert tone="error" title={t('Microphone blocked. Allow the microphone.', 'مائیک بند ہے۔ مائیک کی اجازت دیں۔')} />}
            {open ? (
              worker ? (
                <div className="chat-composer worker-composer">{offerLink}{mic}</div>
              ) : (
                <form className="chat-composer" onSubmit={send}>
                  {offerLink}
                  <input aria-label={t('Message', 'پیغام')} placeholder={t('Type a message…', 'پیغام لکھیں…')} value={text} onChange={(e) => setText(e.target.value)} />
                  {text.trim() ? <button className="icon-button" type="submit" aria-label={t('Send message', 'پیغام بھیجیں')}><Icon name="Send" /></button> : mic}
                </form>
              )
            ) : <p className="note center">{t('This job is closed. Messages are read-only.', 'یہ کام بند ہو چکا ہے۔ اب پیغام نہیں بھیجے جا سکتے۔')}</p>}
          </Task>
        )
      }}
    </JobPage>
  )
}

export function MakeOffer({ id }: { id: number }) {
  const [value, setValue] = useState('')
  const [state, setState] = useState<'idle' | 'sending' | 'error'>('idle')
  const t = useT()
  return (
    <JobPage id={id} screen="17-customer-offer">
      {({ job }, me) => {
        const worker = me.user.role === 'worker'
        if (job.status !== 'requested') return (
          <Task screen="17-customer-offer" worker={worker}>
            <StatePanel kind="success" title={t('The price is already agreed', 'قیمت طے ہو چکی ہے')} />
            <Link className="button full" href={`/app/jobs/${id}`}><Icon name="MessageCircle" />{t('Open chat', 'بات کھولیں')}</Link>
          </Task>
        )
        async function send() {
          setState('sending')
          try {
            await api.sendOffer(id, Number(value))
            toast(t('Offer sent. Waiting for a reply.', 'قیمت بھیج دی۔ جواب کا انتظار کریں۔'))
            navigate(`/app/jobs/${id}`)
          } catch {
            setState('error')
          }
        }
        return (
          <Task screen={worker ? '18-worker-offer' : '17-customer-offer'} worker={worker} prompt="offer" aside={<JobAside job={job} worker={worker} />}>
            <div className="section-heading">
              <h1>{t('Offer a price', 'قیمت بتائیں')}</h1>
              {!worker && <p className="muted">{t('Agree on the scope in chat. The other person must accept.', 'کام بات چیت میں طے کریں۔ دوسرے شخص کا منظور کرنا ضروری ہے۔')}</p>}
            </div>
            <section className="card card-soft">
              <p className="muted">{t('Pakistani rupees', 'روپے')}</p>
              <p className="price" aria-live="polite">{rupees(Number(value || 0))}</p>
            </section>
            <Keypad onChange={setValue} />
            {state === 'error' && <Alert tone="error" title={t('Offer not sent', 'پیشکش نہیں گئی')} text={t('Your amount is kept. Try again.', 'آپ کی رقم محفوظ ہے۔ دوبارہ کوشش کریں۔')} />}
            <button className="button full" type="button" onClick={send} disabled={!Number(value) || state === 'sending'}>
              <Icon name={state === 'sending' ? 'LoaderCircle' : 'Send'} />{t('Send offer', 'پیشکش بھیجیں')}
            </button>
            {!worker && <p className="note">{t('Sending an offer does not reserve the worker.', 'پیشکش بھیجنے سے کاریگر بُک نہیں ہوتا۔')}</p>}
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
  const t = useT()
  return (
    <JobPage id={id} screen="19-customer-job" poll={5000}>
      {({ job }, me, reload) => {
        const worker = me.user.role === 'worker'
        const first = job.worker.name
        async function move(status: 'in_progress' | 'completed') {
          setState('saving')
          try {
            await api.setStatus(id, status)
            setConfirming(false)
            setState('idle')
            toast(status === 'in_progress' ? t('Work started', 'کام شروع ہو گیا') : t('Job marked complete', 'کام مکمل ہو گیا'))
            reload()
          } catch {
            setState('error')
          }
        }
        const price = job.agreed_price !== null && (
          <section className="card card-accent"><p className="muted">{t('Agreed price', 'طے شدہ قیمت')}</p><p className="price">{rupees(job.agreed_price)}</p></section>
        )
        const chat = <Link className={`button ${worker ? 'outline' : ''} full`} href={`/app/jobs/${id}`}><Icon name="MessageCircle" />{t('Open chat', 'بات کھولیں')}</Link>
        const cancel = isActive(job.status) && (
          <Link className={`button ${worker ? 'ghost' : 'outline full'}`} href={`/app/jobs/${id}/cancel`}><Icon name="X" />{t('Cancel job', 'کام منسوخ کریں')}</Link>
        )
        const saveError = state === 'error' && <Alert tone="error" title={t('That didn’t go through. Try again.', 'نہیں ہو سکا۔ دوبارہ کوشش کریں۔')} />

        if (job.status === 'cancelled') return (
          <Task screen={worker ? '27-worker-cancel' : '26-customer-cancel'} worker={worker}>
            <Hero tone="stop" icon="X" title={t('This job was cancelled', 'کام منسوخ ہو گیا')}
              hint={worker ? t('This work will not take place.', 'اب یہ کام نہیں ہوگا۔') : t('No work will take place. You can request help again.', 'اب کوئی کام نہیں ہوگا۔ آپ دوبارہ مدد مانگ سکتے ہیں۔')} />
            <Link className="button full" href={worker ? '/app' : '/app/find'}><Icon name="ArrowRight" />{worker ? t('Back', 'واپس') : t('Find help again', 'دوبارہ مدد ڈھونڈیں')}</Link>
          </Task>
        )

        if (worker) {
          const hero = job.status === 'requested'
            ? <Hero tone="wait" icon="MessageCircle" title={t('Agree the price', 'قیمت طے کریں')} hint={t('Talk to the customer and send your price.', 'گاہک سے بات کریں اور اپنی قیمت بھیجیں۔')} />
            : job.status === 'confirmed'
              ? <Hero tone="go" icon="CircleCheck" title={t('Job confirmed', 'کام پکا ہو گیا')} hint={t('Go to the customer, then press Start work.', 'گاہک کے پاس پہنچ کر ’کام شروع کریں‘ دبائیں۔')} />
              : job.status === 'in_progress'
                ? confirming
                  ? <Hero tone="work" icon="Wrench" title={t('Is the work complete?', 'کام مکمل ہو گیا؟')} hint={t('If the work is finished, press Yes, complete.', 'کام ختم ہو گیا ہے تو ’ہاں، مکمل‘ دبائیں۔')} />
                  : <Hero tone="work" icon="Wrench" title={t('Work in progress', 'کام جاری ہے')} hint={t('When the work is finished, press Work complete.', 'کام ختم ہو جائے تو ’کام مکمل‘ دبائیں۔')} />
                : <Hero tone="done" icon="CheckCheck" title={t('Job complete', 'کام مکمل ہو گیا')} hint={t('Well done. Collect the agreed price from the customer.', 'شاباش! طے شدہ قیمت گاہک سے لے لیں۔')} />
          return (
            <Task screen={job.status === 'in_progress' ? '21-worker-complete' : '20-worker-job'} worker prompt="done"
              aside={<JobAside job={job} worker>{job.status === 'completed' && !job.reviewed &&
                <DemoSwitch job={job} to="customer" label="Continue as customer → Leave a review" text="The customer reviews the completed job." />}</JobAside>}>
              <Timeline status={job.status} />
              {hero}
              <JobCard job={job} worker />
              {price}
              {saveError}
              {job.status === 'confirmed' && (
                <button className="button full" type="button" disabled={state === 'saving'} onClick={() => move('in_progress')}><Icon name="Play" />{t('Start work', 'کام شروع کریں')}</button>
              )}
              {job.status === 'in_progress' && (confirming ? (
                <>
                  <button className="button full" type="button" disabled={state === 'saving'} onClick={() => move('completed')}><Icon name="Check" />{t('Yes, complete', 'ہاں، مکمل')}</button>
                  <button className="button outline full" type="button" onClick={() => setConfirming(false)}><Icon name="ArrowRight" />{t('Not yet', 'ابھی نہیں')}</button>
                </>
              ) : <button className="button full" type="button" onClick={() => setConfirming(true)}><Icon name="CircleCheck" />{t('Work complete', 'کام مکمل')}</button>)}
              {job.status === 'completed' ? <Link className="button outline full" href="/app"><Icon name="ArrowRight" />{t('Back', 'واپس')}</Link> : chat}
              {!confirming && cancel}
            </Task>
          )
        }

        const hero = job.status === 'requested'
          ? <Hero tone="wait" icon="MessageCircle" title={t('Agree a price to book', 'بُکنگ کے لیے قیمت طے کریں')}
              hint={t('Nothing is booked until one of you accepts the other’s price offer.', 'جب تک آپ دونوں میں سے کوئی ایک دوسرے کی قیمت منظور نہ کرے، کچھ بھی پکا نہیں۔')} />
          : job.status === 'confirmed'
            ? <Hero tone="go" icon="CircleCheck" title={t(`Booked with ${first}`, `${first} کے ساتھ کام پکا ہو گیا`)}
                hint={t('The worker is coming to you. The price is fixed.', 'کاریگر آپ کے پاس آ رہا ہے۔ قیمت طے ہو چکی ہے۔')} />
            : job.status === 'in_progress'
              ? <Hero tone="work" icon="Wrench" title={t('Work is in progress', 'کام جاری ہے')}
                  hint={t('The worker will mark the job complete when the work is done.', 'کام ختم ہونے پر کاریگر اسے مکمل کر دے گا۔')} />
              : <Hero tone="done" icon="CheckCheck" title={t('Job complete', 'کام مکمل ہو گیا')}
                  hint={job.reviewed
                    ? t('Your agreed scope and price stay in the chat.', 'طے شدہ کام اور قیمت بات چیت میں محفوظ ہیں۔')
                    : t('Please leave a review. It helps the next customer.', 'ریویو ضرور دیں۔ اس سے اگلے گاہک کو مدد ملتی ہے۔')} />
        return (
          <Task screen="19-customer-job" aside={<JobAside job={job} worker={false}>{(job.status === 'confirmed' || job.status === 'in_progress') &&
            <DemoSwitch job={job} to="worker" label="View worker’s next step" text="The worker starts and completes the job. Switch roles to see that step." />}</JobAside>}>
            <Timeline status={job.status} />
            {hero}
            <JobCard job={job} />
            {price}
            {job.status === 'completed' && (job.reviewed
              ? <Alert tone="success" title={t('Thank you for your review', 'ریویو کا شکریہ')} />
              : <Link className="button full" href={`/app/jobs/${id}/review`}><Icon name="Star" />{t('Leave a review', 'ریویو دیں')}</Link>)}
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
  const t = useT()
  return (
    <JobPage id={id} screen="26-customer-cancel">
      {({ job }, me) => {
        const worker = me.user.role === 'worker'
        if (!isActive(job.status)) return <Redirect to={`/app/jobs/${id}/status`} />
        async function cancel() {
          setState('saving')
          try {
            await api.setStatus(id, 'cancelled')
            toast(t('Job cancelled', 'کام منسوخ ہو گیا'))
            navigate(`/app/jobs/${id}/status`, true)
          } catch {
            setState('error')
          }
        }
        return (
          <Task screen={worker ? '27-worker-cancel' : '26-customer-cancel'} worker={worker}>
            <div className="section-heading">
              <h1>{t('Cancel this job?', 'کام منسوخ کریں؟')}</h1>
              {!worker && <p className="muted">{t('The other person will see the cancellation. You can request help again.', 'دوسرے شخص کو منسوخی نظر آئے گی۔ آپ دوبارہ مدد مانگ سکتے ہیں۔')}</p>}
            </div>
            <JobCard job={job} worker={worker} />
            {state === 'error' && <Alert tone="error" title={t('The job was not cancelled. Try again.', 'کام منسوخ نہیں ہوا۔ دوبارہ کوشش کریں۔')} />}
            <button className="button danger full" type="button" onClick={cancel} disabled={state === 'saving'}><Icon name="X" />{t('Yes, cancel job', 'ہاں، منسوخ کریں')}</button>
            <Link className="button outline full" href={`/app/jobs/${id}/status`}><Icon name="ArrowLeft" />{t('Keep this job', 'نہیں، کام رہنے دیں')}</Link>
          </Task>
        )
      }}
    </JobPage>
  )
}

const RATING = [['', ''], ['Poor', 'خراب'], ['Fair', 'ٹھیک'], ['Good', 'اچھا'], ['Very good', 'بہت اچھا'], ['Excellent', 'بہترین']] as const

export function ReviewJob({ id }: { id: number }) {
  const [rating, setRating] = useState(0)
  const [body, setBody] = useState('')
  const [clip, setClip] = useState<Blob | null>(null)
  const [state, setState] = useState<'idle' | 'saving' | 'error' | 'done'>('idle')
  const rec = useRecorder(setClip)
  const t = useT()
  return (
    <JobPage id={id} screen="22-customer-review">
      {({ job }, me) => {
        if (me.user.role !== 'customer' || job.customer.id !== me.user.id) return <Redirect to={`/app/jobs/${id}/status`} />
        if (state === 'done' || job.reviewed) return (
          <Task screen="22-customer-review" aside={<JobAside job={job} worker={false} />}>
            <StatePanel kind="success" title={t('Thank you for your review', 'ریویو کا شکریہ')} text={t('Your experience will help the next customer. Review tags and summary may take a moment.', 'آپ کا تجربہ اگلے گاہک کے کام آئے گا۔ ریویو کا خلاصہ آنے میں تھوڑا وقت لگ سکتا ہے۔')} />
            <Link className="button full" href="/app/jobs"><Icon name="ArrowRight" />{t('Continue', 'آگے بڑھیں')}</Link>
          </Task>
        )
        if (job.status !== 'completed') return (
          <Task screen="22-customer-review" aside={<JobAside job={job} worker={false} />}>
            <StatePanel kind="loading" icon="Clock" title={t('Reviews open after the job is complete', 'ریویو کام مکمل ہونے کے بعد دیا جا سکتا ہے')} text={t('The worker marks the job complete when the work is done.', 'کام ختم ہونے پر کاریگر اسے مکمل کرتا ہے۔')} />
            <Link className="button full" href={`/app/jobs/${id}/status`}><Icon name="ArrowLeft" />{t('Back to job', 'کام پر واپس')}</Link>
          </Task>
        )
        async function submit() {
          setState('saving')
          try {
            await api.review(id, { rating, body: body.trim() || undefined, audio: clip ?? undefined })
            setState('done')
            toast(t('Review sent. Thank you.', 'ریویو بھیج دیا۔ شکریہ۔'))
          } catch {
            setState('error')
          }
        }
        const recording = rec.state === 'recording'
        const first = job.worker.name
        return (
          <Task screen="22-customer-review" aside={<JobAside job={job} worker={false} />}>
            <Timeline status={job.status} />
            <div className="section-heading"><h1>{t(`Rate ${first}’s work`, `${first} کا کام کیسا رہا؟`)}</h1><p className="muted">{t('Stars are required. A voice note or written review is optional.', 'ستارے دینا ضروری ہے۔ آواز یا لکھا ہوا ریویو آپ کی مرضی ہے۔')}</p></div>
            <div className="review-stars" role="group" aria-label={t('Rating', 'ریٹنگ')}>
              {[1, 2, 3, 4, 5].map((n) => (
                <button key={n} type="button" className={n <= rating ? 'selected' : ''} aria-pressed={n <= rating} aria-label={t(`${n} star rating`, `${n} ستارے`)} onClick={() => setRating(n)}>
                  <Icon name="Star" />
                </button>
              ))}
            </div>
            <p className="center caption muted">{rating ? t(`${rating} out of 5 · ${RATING[rating][0]}`, `5 میں سے ${rating} · ${RATING[rating][1]}`) : t('Choose a rating', 'ستارے چنیں')}</p>
            <div className="mic-zone">
              <button className={`mic ${recording ? 'recording' : ''}`} type="button" onClick={rec.toggle} aria-pressed={recording}
                aria-label={recording ? t('Stop recording', 'ریکارڈنگ روکیں') : t('Record a voice review', 'بول کر ریویو دیں')}><Icon name="Mic" /></button>
              <p className="caption muted">{recording ? t('Recording… tap again to finish', 'ریکارڈنگ جاری ہے… ختم کرنے کے لیے دوبارہ دبائیں')
                : clip ? t('Voice review added · tap to record again', 'آواز شامل ہو گئی · دوبارہ ریکارڈ کرنے کے لیے دبائیں')
                : t('Tell us in your own words · optional', 'اپنے الفاظ میں بتائیں · آپ کی مرضی')}</p>
            </div>
            <label className="field" htmlFor="review">{t('Or write a review', 'یا ریویو لکھیں')}
              <textarea id="review" placeholder={t('What should the next customer know?', 'اگلے گاہک کو کیا جاننا چاہیے؟')} value={body} onChange={(e) => setBody(e.target.value)} />
            </label>
            {state === 'error' && <Alert tone="error" title={t('Review not sent', 'ریویو نہیں گیا')} text={t('Your rating and words are kept. Try again.', 'آپ کی ریٹنگ اور الفاظ محفوظ ہیں۔ دوبارہ کوشش کریں۔')} />}
            <button className="button full" type="button" onClick={submit} disabled={!rating || state === 'saving'}>
              <Icon name={state === 'saving' ? 'LoaderCircle' : 'Check'} />{t('Submit review', 'ریویو بھیجیں')}
            </button>
            <p className="note center">{t('Only reviews from completed jobs appear on profiles.', 'صرف مکمل کاموں کے ریویو پروفائل پر نظر آتے ہیں۔')}</p>
          </Task>
        )
      }}
    </JobPage>
  )
}
