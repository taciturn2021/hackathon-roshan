import { useContext, type ReactNode } from 'react'
import type { Job } from '../contract'
import { initials, TRADES } from '../lib/trades'
import { Icon } from './Icon'
import { InApp } from './AppShell'
import { PromptBar } from './ui'

/** Task page: the working panel plus an optional context panel beside it on desktop. */
export function Task({ screen, worker = false, prompt, wide = false, aside, children }: {
  screen: string; worker?: boolean; prompt?: string; wide?: boolean; aside?: ReactNode; children: ReactNode
}) {
  const inApp = useContext(InApp)
  const Tag = inApp ? 'div' : 'main'
  return (
    <Tag {...(inApp ? {} : { id: 'main-content' })} className={`${inApp ? 'app-task' : 'site-container'} task-layout ${wide ? 'wide-task' : ''}`} data-screen={screen}>
      <section className={`task-panel ${worker ? 'worker' : ''}`} {...(worker ? { lang: 'ur', dir: 'rtl' } : {})}>
        {worker && prompt && <PromptBar prompt={prompt} />}
        <div className="main-content">{children}</div>
      </section>
      {!wide && (aside ?? (inApp && !worker && FLOW.includes(screen) ? <StepRail at={FLOW.indexOf(screen)} /> : <Guide worker={worker} compact={inApp} />))}
    </Tag>
  )
}

const FLOW = ['11-problem', '12-problem-confirm', '23-customer-location', '14-worker-profile']
const FLOW_STEPS = ['Describe problem', 'Check trade', 'Your location', 'Choose worker']

/** Where the customer is in the find flow; above the panel on mobile, beside it on desktop. */
function StepRail({ at }: { at: number }) {
  return (
    <nav className="step-rail" aria-label="Request steps">
      <ol>
        {FLOW_STEPS.map((s, i) => (
          <li key={s} className={i < at ? 'done' : ''} aria-current={i === at ? 'step' : undefined}>
            <span className="digits">{i < at ? <Icon name="Check" className="icon-sm" /> : i + 1}</span>{s}
          </li>
        ))}
      </ol>
    </nav>
  )
}

export function Guide({ worker, compact = false }: { worker: boolean; compact?: boolean }) {
  const c = worker
    ? { eyebrow: 'کام کا آسان راستہ', title: 'آپ کا ہنر، آپ کا روزگار', steps: ['اپنی آواز سے آغاز کریں', 'قیمت طے کریں', 'کام مکمل کریں'] }
    : { eyebrow: 'How it works', title: 'Agree the work and the price before anyone arrives', steps: ['Tell us what needs fixing', 'Agree the scope and price', 'Review the completed work'] }
  return (
    <aside className="support-panel" {...(worker ? { lang: 'ur', dir: 'rtl' } : {})}>
      <p className="eyebrow">{c.eyebrow}</p>
      <h2>{c.title}</h2>
      {!compact && <img src="/craft-hero.png" width={1536} height={1024} alt="Tools of local skilled trades" loading="lazy" />}
      <ol>{c.steps.map((s) => <li key={s}>{s}</li>)}</ol>
    </aside>
  )
}

export function JobAside({ job, worker, children }: { job: Job; worker: boolean; children?: ReactNode }) {
  // Trade, area and distance are already on the JobCard; this panel only adds who the other person is.
  const name = (worker ? job.customer.name : job.worker.name) ?? ''
  return (
    <aside className="support-panel job-aside" {...(worker ? { lang: 'ur', dir: 'rtl' } : {})}>
      <div className="row">
        <span className="avatar">{initials(name || '?')}</span>
        <div className="grow"><h2 dir="auto">{name}</h2><p className="caption muted">{worker ? 'گاہک' : TRADES[job.trade].en}</p></div>
      </div>
      <p className="note">{worker ? 'قیمت بات چیت سے طے کریں' : 'Agree the scope in chat. A request is confirmed only when the other person accepts an offer.'}</p>
      {children}
    </aside>
  )
}
