import type { Job } from '../contract'
import { useT } from '../lib/i18n'
import { Link } from '../lib/router'
import { rupees, STATUS, TRADES } from '../lib/trades'
import { Icon } from './Icon'
import { Badge } from './ui'

export const shortDate = (iso: string) => new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })
export const jobHref = (j: Job) => (j.status === 'requested' ? `/app/jobs/${j.id}` : `/app/jobs/${j.id}/status`)

/** Customer jobs as a table; rows stack into cards on narrow screens. */
export function JobsTable({ jobs }: { jobs: Job[] }) {
  const t = useT()
  const cols = { job: t('Job', 'کام'), worker: t('Worker', 'کاریگر'), status: t('Status', 'حالت'), price: t('Price', 'قیمت'), date: t('Date', 'تاریخ') }
  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr><th scope="col">{cols.job}</th><th scope="col">{cols.worker}</th><th scope="col">{cols.status}</th><th scope="col" className="num">{cols.price}</th><th scope="col">{cols.date}</th></tr>
        </thead>
        <tbody>
          {jobs.map((j) => (
            <tr key={j.id}>
              <td data-label={cols.job}>
                <Link className="row-link" href={jobHref(j)}>
                  <span className="trade-dot" data-trade={j.trade}><Icon name={TRADES[j.trade].icon} /></span>
                  <span><strong>{t(TRADES[j.trade].en, TRADES[j.trade].ur)}</strong><small className="muted clamp">{j.description}</small></span>
                </Link>
              </td>
              <td data-label={cols.worker}>{j.worker.name}</td>
              <td data-label={cols.status}><Badge tone={j.status === 'completed' && !j.reviewed ? 'warning' : STATUS[j.status].tone}>{j.status === 'completed' && !j.reviewed ? t('Review due', 'ریویو باقی') : t(STATUS[j.status].en, STATUS[j.status].ur)}</Badge></td>
              <td data-label={cols.price} className="num digits">{j.agreed_price !== null ? rupees(j.agreed_price) : '—'}</td>
              <td data-label={cols.date} className="digits">{shortDate(j.completed_at ?? j.created_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
