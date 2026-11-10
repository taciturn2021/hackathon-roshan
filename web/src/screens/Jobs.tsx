import { useState } from 'react'
import { Icon } from '../components/Icon'
import { JobsTable, jobHref } from '../components/JobsTable'
import { Task } from '../components/Task'
import { Badge, JobCard, Skeletons, StatePanel } from '../components/ui'
import type { JobStatus } from '../contract'
import { api } from '../lib/api'
import { useT } from '../lib/i18n'
import { Link } from '../lib/router'
import { useLoad, useMe } from '../lib/session'
import { isActive, rupees, STATUS, TRADES } from '../lib/trades'

type Filter = 'current' | 'completed' | 'cancelled' | 'all'
const FILTERS: { key: Filter; en: string; ur: string }[] = [
  { key: 'all', en: 'All', ur: 'سب' },
  { key: 'current', en: 'Current', ur: 'جاری' },
  { key: 'completed', en: 'Completed', ur: 'مکمل' },
  { key: 'cancelled', en: 'Cancelled', ur: 'منسوخ' },
]
const fits = (f: Filter, s: JobStatus) => (f === 'all' ? true : f === 'current' ? isActive(s) : s === f)

export function Jobs() {
  const me = useMe()
  const t = useT()
  const [filter, setFilter] = useState<Filter>('all')
  const jobs = useLoad(() => api.listJobs(), [me?.user.id])
  if (!me) return null
  const worker = me.user.role === 'worker'
  const s = jobs.state
  const all = s.status === 'ready' ? s.data : []
  const shown = all.filter((j) => fits(filter, j.status))

  return (
    <Task screen={worker ? '25-worker-jobs' : '24-customer-jobs'} worker={worker} prompt="home" wide>
      <div className="segment" role="tablist" aria-label={t('Filter jobs', 'کام فلٹر کریں')}>
        {FILTERS.map((f) => (
          <button key={f.key} type="button" role="tab" aria-selected={filter === f.key} className={`button outline ${filter === f.key ? 'selected' : ''}`} onClick={() => setFilter(f.key)}>
            {t(f.en, f.ur)} <span className="count digits">{all.filter((j) => fits(f.key, j.status)).length}</span>
          </button>
        ))}
      </div>
      {s.status === 'loading' ? <Skeletons />
        : s.status === 'error' ? (
          <>
            <StatePanel kind="error" title={t('Couldn’t load your jobs', 'کام نہیں مل سکے')} />
            <button className="button full" type="button" onClick={jobs.reload}><Icon name="RefreshCw" />{t('Try again', 'دوبارہ کوشش')}</button>
          </>
        ) : shown.length === 0 ? (
          <section className="empty-panel">
            <div className="empty-art"><Icon name="BriefcaseBusiness" /></div>
            <h2>{t('No jobs here', 'یہاں کوئی کام نہیں')}</h2>
            {filter === 'current' && <Link className="button" href={worker ? '/app' : '/app/find'}><Icon name="ArrowRight" />{worker ? t('Go online', 'آن لائن ہوں') : t('Describe your problem', 'اپنا مسئلہ بتائیں')}</Link>}
          </section>
        ) : worker ? (
          <div className="card-grid">
            {shown.map((job) => isActive(job.status) ? (
              <div className="stack" key={job.id}>
                <JobCard job={job} worker />
                <Link className="button full" href={jobHref(job)}><Icon name="ArrowRight" />{t('Open job', 'کام کھولیں')}</Link>
              </div>
            ) : (
              <Link className="card" key={job.id} href={`/app/jobs/${job.id}/status`}>
                <div className="row between">
                  <span>{t(`${TRADES[job.trade].en} job`, `${TRADES[job.trade].ur} کا کام`)}</span>
                  <Badge tone={STATUS[job.status].tone}>{t(STATUS[job.status].en, STATUS[job.status].ur)}</Badge>
                </div>
                {job.agreed_price !== null && <p className="price">{rupees(job.agreed_price)}</p>}
              </Link>
            ))}
          </div>
        ) : <JobsTable jobs={shown} />}
    </Task>
  )
}
