import { Icon } from '../components/Icon'
import { JobsTable, jobHref, shortDate } from '../components/JobsTable'
import { Skeletons, StatePanel, StatTiles } from '../components/ui'
import type { Job, Trade } from '../contract'
import { api } from '../lib/api'
import { problemDraft } from '../lib/draft'
import { Link } from '../lib/router'
import { useLoad, useMe } from '../lib/session'
import { isActive, rupees, TRADES } from '../lib/trades'
import { WorkerHome } from './WorkerHome'

export function Dashboard() {
  const me = useMe()
  return me?.user.role === 'worker' ? <WorkerHome /> : <CustomerDashboard />
}

type Event = { at: string; text: string; icon: 'Clock' | 'CircleCheck' | 'X'; job: Job }
function activity(jobs: Job[]): Event[] {
  const events: Event[] = []
  for (const j of jobs) {
    const what = `${TRADES[j.trade].en} · ${j.worker.name}`
    events.push({ at: j.created_at, text: `Requested ${what}`, icon: 'Clock', job: j })
    if (j.completed_at) events.push({ at: j.completed_at, text: `Completed ${what}`, icon: 'CircleCheck', job: j })
    if (j.status === 'cancelled') events.push({ at: j.created_at, text: `Cancelled ${what}`, icon: 'X', job: j })
  }
  return events.sort((a, b) => b.at.localeCompare(a.at)).slice(0, 6)
}

function CustomerDashboard() {
  const jobs = useLoad(() => api.listJobs(), [])
  const s = jobs.state
  if (s.status === 'loading') return <div className="dash"><Skeletons count={3} /></div>
  if (s.status === 'error') return (
    <div className="dash">
      <StatePanel kind="error" title="Couldn’t load your jobs" text="Check your connection and try again." />
      <button className="button" type="button" onClick={jobs.reload}><Icon name="RefreshCw" />Try again</button>
    </div>
  )

  const list = s.data
  const done = list.filter((j) => j.status === 'completed')
  const paid = done.reduce((n, j) => n + (j.agreed_price ?? 0), 0)
  const toReview = done.filter((j) => !j.reviewed)
  const byTrade = Object.entries(done.reduce<Record<string, number>>((m, j) => ({ ...m, [j.trade]: (m[j.trade] ?? 0) + (j.agreed_price ?? 0) }), {}))
    .sort((a, b) => b[1] - a[1]) as [Trade, number][]
  const top = byTrade[0]?.[1] ?? 1

  if (list.length === 0) return (
    <div className="dash">
      <section className="empty-panel">
        <div className="empty-art"><Icon name="Search" /></div>
        <h2>No jobs yet</h2>
        <p className="muted">Describe what needs fixing and we’ll show workers who are free near you.</p>
        <Link className="button" href="/app/find"><Icon name="ArrowRight" />Describe your problem</Link>
      </section>
    </div>
  )

  return (
    <div className="dash">
      <StatTiles tiles={[
        { label: 'Active jobs', value: String(list.filter((j) => isActive(j.status)).length), icon: 'BriefcaseBusiness', accent: true },
        { label: 'Completed', value: String(done.length), icon: 'CircleCheck' },
        { label: 'Paid to workers', value: rupees(paid), icon: 'Banknote', note: 'Agreed prices of completed jobs' },
        { label: 'Reviews due', value: String(toReview.length), icon: 'Star' },
      ]} />

      {toReview[0] && (
        <div className="alert success" role="status">
          <Icon name="Star" />
          <div><strong>Rate {toReview[0].worker.name}</strong><p>Your {TRADES[toReview[0].trade].en.toLowerCase()} job from {shortDate(toReview[0].completed_at!)} is waiting for a review.</p></div>
          <Link className="button outline" href={`/app/jobs/${toReview[0].id}/review`}>Leave a review</Link>
        </div>
      )}

      <div className="dash-grid">
        <section className="panel span-2">
          <header className="panel-head"><h2>Recent jobs</h2><Link className="text-link" href="/app/jobs">All jobs</Link></header>
          <JobsTable jobs={list.slice(0, 5)} />
        </section>

        <section className="panel">
          <header className="panel-head"><h2>Activity</h2></header>
          <ol className="feed">
            {activity(list).map((e, i) => (
              <li key={i}>
                <span className={`feed-icon ${e.icon === 'CircleCheck' ? 'done' : e.icon === 'X' ? 'stop' : ''}`}><Icon name={e.icon} /></span>
                <Link href={jobHref(e.job)}>{e.text}</Link>
                <time className="digits">{shortDate(e.at)}</time>
              </li>
            ))}
          </ol>
        </section>

        <section className="panel span-2">
          <header className="panel-head"><h2>Paid by trade</h2><span className="muted digits">{rupees(paid)} total</span></header>
          {byTrade.length === 0 ? <p className="muted">Completed jobs will appear here.</p> : (
            <ul className="bars">
              {byTrade.map(([trade, sum]) => (
                <li key={trade}>
                  <span className="bar-label"><Icon name={TRADES[trade].icon} />{TRADES[trade].en}</span>
                  <span className="bar-track"><span className="bar-fill" style={{ width: `${Math.max(6, (sum / top) * 100)}%` }} /></span>
                  <span className="bar-value digits">{rupees(sum)}</span>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="panel">
          <header className="panel-head"><h2>Need something fixed?</h2></header>
          <div className="quick-trades">
            {(Object.keys(TRADES) as Trade[]).map((t) => (
              <Link key={t} href="/app/find/confirm" className="quick-trade" onClick={() => problemDraft.set({ trade: t, trade_confidence: 1, description: '', price_range: null })}><Icon name={TRADES[t].icon} />{TRADES[t].en}</Link>
            ))}
          </div>
        </section>
      </div>
    </div>
  )
}
