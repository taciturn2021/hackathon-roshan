import type { Job } from '../contract'
import { Link } from '../lib/router'
import { rupees, STATUS, TRADES } from '../lib/trades'
import { Icon } from './Icon'
import { Badge } from './ui'

export const shortDate = (iso: string) => new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })
export const jobHref = (j: Job) => (j.status === 'requested' ? `/app/jobs/${j.id}` : `/app/jobs/${j.id}/status`)

/** Customer jobs as a table; rows stack into cards on narrow screens. */
export function JobsTable({ jobs }: { jobs: Job[] }) {
  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr><th scope="col">Job</th><th scope="col">Worker</th><th scope="col">Status</th><th scope="col" className="num">Price</th><th scope="col">Date</th></tr>
        </thead>
        <tbody>
          {jobs.map((j) => (
            <tr key={j.id}>
              <td data-label="Job">
                <Link className="row-link" href={jobHref(j)}>
                  <span className="trade-dot"><Icon name={TRADES[j.trade].icon} /></span>
                  <span><strong>{TRADES[j.trade].en}</strong><small className="muted clamp">{j.description}</small></span>
                </Link>
              </td>
              <td data-label="Worker">{j.worker.name}</td>
              <td data-label="Status"><Badge tone={j.status === 'completed' && !j.reviewed ? 'warning' : STATUS[j.status].tone}>{j.status === 'completed' && !j.reviewed ? 'Review due' : STATUS[j.status].en}</Badge></td>
              <td data-label="Price" className="num digits">{j.agreed_price !== null ? rupees(j.agreed_price) : '—'}</td>
              <td data-label="Date" className="digits">{shortDate(j.completed_at ?? j.created_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
