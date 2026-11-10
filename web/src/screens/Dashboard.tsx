import { Icon } from '../components/Icon'
import { JobsTable, jobHref, shortDate } from '../components/JobsTable'
import { Skeletons, StatePanel, StatTiles } from '../components/ui'
import type { Job, Trade } from '../contract'
import { api } from '../lib/api'
import { problemDraft } from '../lib/draft'
import { useT } from '../lib/i18n'
import { Link } from '../lib/router'
import { useLoad, useMe } from '../lib/session'
import { isActive, rupees, TRADES } from '../lib/trades'
import { WorkerHome } from './WorkerHome'

export function Dashboard() {
  const me = useMe()
  return me?.user.role === 'worker' ? <WorkerHome /> : <CustomerDashboard />
}

type Event = { at: string; text: string; icon: 'Clock' | 'CircleCheck' | 'X'; job: Job }
function activity(jobs: Job[], t: ReturnType<typeof useT>): Event[] {
  const events: Event[] = []
  for (const j of jobs) {
    const en = `${TRADES[j.trade].en} · ${j.worker.name}`
    const ur = `${TRADES[j.trade].ur} · ${j.worker.name}`
    events.push({ at: j.created_at, text: t(`Requested ${en}`, `درخواست بھیجی · ${ur}`), icon: 'Clock', job: j })
    if (j.completed_at) events.push({ at: j.completed_at, text: t(`Completed ${en}`, `کام مکمل ہوا · ${ur}`), icon: 'CircleCheck', job: j })
    if (j.status === 'cancelled') events.push({ at: j.created_at, text: t(`Cancelled ${en}`, `کام منسوخ ہوا · ${ur}`), icon: 'X', job: j })
  }
  return events.sort((a, b) => b.at.localeCompare(a.at)).slice(0, 6)
}

function CustomerDashboard() {
  const t = useT()
  const jobs = useLoad(() => api.listJobs(), [])
  const s = jobs.state
  if (s.status === 'loading') return <div className="dash"><Skeletons count={3} /></div>
  if (s.status === 'error') return (
    <div className="dash">
      <StatePanel kind="error" title={t('Couldn’t load your jobs', 'کام نہیں مل سکے')} text={t('Check your connection and try again.', 'انٹرنیٹ چیک کریں اور دوبارہ کوشش کریں۔')} />
      <button className="button" type="button" onClick={jobs.reload}><Icon name="RefreshCw" />{t('Try again', 'دوبارہ کوشش')}</button>
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
        <h2>{t('No jobs yet', 'ابھی کوئی کام نہیں')}</h2>
        <p className="muted">{t('Describe what needs fixing and we’ll show workers who are free near you.', 'بتائیں کیا ٹھیک کروانا ہے، ہم آپ کے قریب فارغ کاریگر دکھائیں گے۔')}</p>
        <Link className="button" href="/app/find"><Icon name="ArrowRight" />{t('Describe your problem', 'اپنا مسئلہ بتائیں')}</Link>
      </section>
    </div>
  )

  return (
    <div className="dash">
      <StatTiles tiles={[
        { label: t('Active jobs', 'جاری کام'), value: String(list.filter((j) => isActive(j.status)).length), icon: 'BriefcaseBusiness', accent: true },
        { label: t('Completed', 'مکمل'), value: String(done.length), icon: 'CircleCheck' },
        { label: t('Paid to workers', 'کاریگروں کو ادا کیا'), value: rupees(paid), icon: 'Banknote', note: t('Agreed prices of completed jobs', 'مکمل کاموں کی طے شدہ قیمتیں') },
        { label: t('Reviews due', 'باقی ریویو'), value: String(toReview.length), icon: 'Star' },
      ]} />

      {toReview[0] && (
        <div className="alert success" role="status">
          <Icon name="Star" />
          <div>
            <strong>{t(`Rate ${toReview[0].worker.name}`, `${toReview[0].worker.name} کو ریٹنگ دیں`)}</strong>
            {/* The date stays Latin, so it sits in its own left-to-right span inside the Urdu sentence. */}
            <p>{t(`Your ${TRADES[toReview[0].trade].en.toLowerCase()} job from `, '')}<span className="digits">{shortDate(toReview[0].completed_at!)}</span>{t(' is waiting for a review.', ` کا ${TRADES[toReview[0].trade].ur} کا کام آپ کے ریویو کے انتظار میں ہے۔`)}</p>
          </div>
          <Link className="button outline" href={`/app/jobs/${toReview[0].id}/review`}>{t('Leave a review', 'ریویو دیں')}</Link>
        </div>
      )}

      <div className="dash-grid">
        <section className="panel span-2">
          <header className="panel-head"><h2>{t('Recent jobs', 'حالیہ کام')}</h2><Link className="text-link" href="/app/jobs">{t('All jobs', 'تمام کام')}</Link></header>
          <JobsTable jobs={list.slice(0, 5)} />
        </section>

        <section className="panel">
          <header className="panel-head"><h2>{t('Activity', 'سرگرمی')}</h2></header>
          <ol className="feed">
            {activity(list, t).map((e, i) => (
              <li key={i}>
                <span className={`feed-icon ${e.icon === 'CircleCheck' ? 'done' : e.icon === 'X' ? 'stop' : ''}`}><Icon name={e.icon} /></span>
                <Link href={jobHref(e.job)}>{e.text}</Link>
                <time className="digits">{shortDate(e.at)}</time>
              </li>
            ))}
          </ol>
        </section>

        <section className="panel span-2">
          <header className="panel-head"><h2>{t('Paid by trade', 'کام کے حساب سے ادائیگی')}</h2><span className="muted">{t('Total', 'کل')} <span className="digits">{rupees(paid)}</span></span></header>
          {byTrade.length === 0 ? <p className="muted">{t('Completed jobs will appear here.', 'مکمل کام یہاں نظر آئیں گے۔')}</p> : (
            <ul className="bars">
              {byTrade.map(([trade, sum]) => (
                <li key={trade}>
                  <span className="bar-label"><Icon name={TRADES[trade].icon} />{t(TRADES[trade].en, TRADES[trade].ur)}</span>
                  <span className="bar-track"><span className="bar-fill" style={{ width: `${Math.max(6, (sum / top) * 100)}%` }} /></span>
                  <span className="bar-value digits">{rupees(sum)}</span>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="panel">
          <header className="panel-head"><h2>{t('Need something fixed?', 'کچھ ٹھیک کروانا ہے؟')}</h2></header>
          <div className="quick-trades">
            {(Object.keys(TRADES) as Trade[]).map((k) => (
              <Link key={k} href="/app/find/confirm" className="quick-trade" data-trade={k} onClick={() => problemDraft.set({ trade: k, trade_confidence: 1, description: '', price_range: null })}><Icon name={TRADES[k].icon} />{t(TRADES[k].en, TRADES[k].ur)}</Link>
            ))}
          </div>
        </section>
      </div>
    </div>
  )
}
