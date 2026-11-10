import { useEffect, useRef, useState } from 'react'
import { Icon } from '../components/Icon'
import { Task } from '../components/Task'
import { Alert, JobCard, Skeletons, StatePanel, StatTiles } from '../components/ui'
import { api } from '../lib/api'
import { playPrompt } from '../lib/audio'
import { Link, navigate } from '../lib/router'
import { setMe, useLoad, useRole } from '../lib/session'
import { rupees } from '../lib/trades'

export function WorkerHome() {
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
  const tiles = (
    <StatTiles tiles={[
      { label: 'اس ہفتے', value: rupees(earned), icon: 'Banknote', accent: true },
      { label: 'مکمل کام', value: String(me.worker.jobs_done), icon: 'CircleCheck' },
      { label: 'ریٹنگ', value: me.worker.rating_avg?.toFixed(1) ?? '—', icon: 'Star' },
    ]} />
  )

  async function toggle() {
    setSaving(true); setError(false)
    try {
      setMe({ ...me!, worker: await api.setOnline(!online) })
    } catch {
      setError(true)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Task screen="10-worker-home" worker prompt="home">
      {tiles}
      {active ? (
        <>
          <Alert title="کام جاری ہے" />
          <JobCard job={active} worker />
          <Link className="button outline full" href={`/app/jobs/${active.id}/status`}><Icon name="BriefcaseBusiness" />کام کھولیں</Link>
        </>
      ) : (
        <>
          <button className={`online-toggle ${online ? '' : 'off'}`} type="button" role="switch" aria-checked={online} onClick={toggle} disabled={saving}>
            <span>{online ? 'آن لائن' : 'آف لائن'}</span>
            <span className="toggle-track"><span className="toggle-thumb" /></span>
          </button>
          {error && <Alert tone="error" title="دوبارہ کوشش کریں" />}
          {jobs.state.status === 'loading' ? <Skeletons count={1} />
            : jobs.state.status === 'error' ? (
              <>
                <StatePanel kind="error" title="کام نہیں مل سکے" />
                <button className="button full" type="button" onClick={jobs.reload}><Icon name="RefreshCw" />دوبارہ کوشش</button>
              </>
            ) : !online ? (
              <div className="state-panel"><div className="empty-art"><Icon name="Power" /></div><h2>کام کے لیے آن کریں</h2></div>
            ) : incoming ? (
              <>
                <div className="section-heading"><h1>نیا کام آیا ہے</h1></div>
                <JobCard job={incoming} worker />
                <Link className="button outline full" href={`/app/jobs/${incoming.id}`}><Icon name="MessageCircle" />کام کھولیں</Link>
              </>
            ) : (
              <div className="state-panel"><div className="empty-art"><Icon name="Bell" /></div><h2>نئے کام کا انتظار</h2></div>
            )}
        </>
      )}
    </Task>
  )
}
