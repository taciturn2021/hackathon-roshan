import { useEffect, useRef, useState } from 'react'
import { Hero, Steps } from '../components/Hero'
import { Icon } from '../components/Icon'
import { Task } from '../components/Task'
import { Alert, JobCard, Skeletons, StatePanel, StatTiles } from '../components/ui'
import { api } from '../lib/api'
import { playPrompt, toast } from '../lib/audio'
import { useT } from '../lib/i18n'
import { Link, navigate } from '../lib/router'
import { setMe, useLoad, useRole } from '../lib/session'
import { rupees } from '../lib/trades'

export function WorkerHome() {
  const t = useT()
  const me = useRole('worker')
  const jobs = useLoad(() => api.listJobs(), [], 5000)
  const [saving, setSaving] = useState(false)
  const [weekAgo] = useState(() => new Date(Date.now() - 7 * 864e5).toISOString())
  const [error, setError] = useState(false)
  const seen = useRef<Set<number> | null>(null)

  useEffect(() => { if (me && !me.worker) navigate('/worker/onboard', true) }, [me])

  const list = jobs.state.status === 'ready' ? jobs.state.data : []
  const active = list.find((j) => j.status === 'confirmed' || j.status === 'in_progress')
  const incoming = list.find((j) => j.status === 'requested')

  // Ringtone once for each job that arrives while this page is open.
  useEffect(() => {
    if (jobs.state.status !== 'ready') return
    const ids = jobs.state.data.filter((j) => j.status === 'requested').map((j) => j.id)
    if (seen.current && ids.some((id) => !seen.current!.has(id))) playPrompt('ring').catch(() => undefined)
    seen.current = new Set(ids)
  }, [jobs.state])

  if (!me?.worker) return null
  const online = me.worker.is_online
  const earned = list.filter((j) => j.status === 'completed' && (j.completed_at ?? '') >= weekAgo).reduce((n, j) => n + (j.agreed_price ?? 0), 0)

  async function toggle() {
    setSaving(true); setError(false)
    try {
      const worker = await api.setOnline(!online)
      setMe({ ...me!, worker })
      toast(worker.is_online ? t('You are online. Jobs can reach you now.', 'آپ آن لائن ہو گئے۔ اب کام آ سکتا ہے۔') : t('You are offline. No jobs will come.', 'آپ آف لائن ہو گئے۔ اب کام نہیں آئے گا۔'))
    } catch {
      setError(true)
    } finally {
      setSaving(false)
    }
  }

  // One card says where the worker stands; its colour and icon change with the state.
  const hero = active ? <Hero tone="work" icon="Wrench" title={t('You are on a job', 'آپ ایک کام پر ہیں')} hint={t('Open the job to see the next step.', 'اگلا قدم دیکھنے کے لیے کام کھولیں۔')} />
    : !online ? <Hero tone="idle" icon="Power" title={t('You are offline', 'آپ آف لائن ہیں')} hint={t('Turn the button below on to get work.', 'کام لینے کے لیے نیچے والا بٹن آن کریں۔')} />
    : incoming ? <Hero tone="wait" icon="Bell" title={t('A new job has come', 'نیا کام آیا ہے')} hint={t('Open it, talk to the customer and send your price.', 'کام کھولیں، گاہک سے بات کریں اور قیمت بھیجیں۔')} />
    : <Hero tone="go" icon="Headphones" title={t('You are online', 'آپ آن لائن ہیں')} hint={t('A bell rings when a job comes. Keep this page open.', 'کام آتے ہی گھنٹی بجے گی۔ یہ صفحہ کھلا رکھیں۔')} />
  const step = active ? 3 : !online ? 0 : incoming ? 2 : 1

  return (
    <Task screen="10-worker-home" worker prompt="home">
      {hero}
      <Steps label={t('How a job goes', 'کام کیسے ہوتا ہے')} at={step} steps={[
        t('Go online', 'آن لائن ہوں'), t('Job comes', 'کام آئے'), t('Agree price', 'قیمت طے'), t('Do the work', 'کام کریں'), t('Get paid', 'پیسے لیں'),
      ]} />
      {active ? (
        <>
          <JobCard job={active} worker />
          <Link className="button full" href={`/app/jobs/${active.id}/status`}><Icon name="BriefcaseBusiness" />{t('Open job', 'کام کھولیں')}</Link>
        </>
      ) : (
        <>
          <button className={`online-toggle ${online ? '' : 'off'}`} type="button" role="switch" aria-checked={online} onClick={toggle} disabled={saving}>
            <span>{online ? t('Online', 'آن لائن') : t('Offline', 'آف لائن')}</span>
            <span className="toggle-track"><span className="toggle-thumb" /></span>
          </button>
          {error && <Alert tone="error" title={t('That did not work. Try again.', 'نہیں ہو سکا۔ دوبارہ کوشش کریں۔')} />}
          {jobs.state.status === 'loading' ? <Skeletons count={1} />
            : jobs.state.status === 'error' ? (
              <>
                <StatePanel kind="error" title={t('Could not load jobs', 'کام نہیں مل سکے')} />
                <button className="button full" type="button" onClick={jobs.reload}><Icon name="RefreshCw" />{t('Try again', 'دوبارہ کوشش')}</button>
              </>
            ) : online && incoming && (
              <>
                <JobCard job={incoming} worker />
                <Link className="button full" href={`/app/jobs/${incoming.id}`}><Icon name="MessageCircle" />{t('Open job', 'کام کھولیں')}</Link>
              </>
            )}
        </>
      )}
      <StatTiles tiles={[
        { label: t('This week', 'اس ہفتے'), value: rupees(earned), icon: 'Banknote', accent: true },
        { label: t('Jobs done', 'مکمل کام'), value: String(me.worker.jobs_done), icon: 'CircleCheck' },
        { label: t('Rating', 'ریٹنگ'), value: me.worker.rating_avg?.toFixed(1) ?? '—', icon: 'Star' },
      ]} />
    </Task>
  )
}
