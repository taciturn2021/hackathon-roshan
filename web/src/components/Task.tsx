import { useContext, type ReactNode } from 'react'
import type { Job } from '../contract'
import { useLang, useT } from '../lib/i18n'
import { initials, TRADES } from '../lib/trades'
import { Icon } from './Icon'
import { InApp } from './AppShell'
import { PromptBar } from './ui'

/** Task page: the working panel plus an optional context panel beside it on desktop. */
export function Task({ screen, worker = false, prompt, wide = false, aside, children }: {
  screen: string; worker?: boolean; prompt?: string; wide?: boolean; aside?: ReactNode; children: ReactNode
}) {
  const inApp = useContext(InApp)
  const ur = useLang() === 'ur'
  const Tag = inApp ? 'div' : 'main'
  return (
    <Tag {...(inApp ? {} : { id: 'main-content' })} className={`${inApp ? 'app-task' : 'site-container'} task-layout ${wide ? 'wide-task' : ''}`} data-screen={screen}>
      <section className={`task-panel ${ur ? 'worker' : ''}`}>
        {worker && prompt && <PromptBar prompt={prompt} />}
        <div className="main-content">{children}</div>
      </section>
      {!wide && (aside ?? (inApp && !worker && FLOW.includes(screen) ? <StepRail at={FLOW.indexOf(screen)} /> : <Guide worker={worker} compact={inApp} />))}
    </Tag>
  )
}

const FLOW = ['11-problem', '12-problem-confirm', '23-customer-location', '14-worker-profile']

/** Where the customer is in the find flow; above the panel on mobile, beside it on desktop. */
function StepRail({ at }: { at: number }) {
  const t = useT()
  const steps = [t('Describe problem', 'مسئلہ بتائیں'), t('Check trade', 'کام کی قسم دیکھیں'), t('Your location', 'آپ کی جگہ'), t('Choose worker', 'کاریگر چنیں')]
  return (
    <nav className="step-rail" aria-label={t('Request steps', 'درخواست کے مرحلے')}>
      <ol>
        {steps.map((s, i) => (
          <li key={s} className={i < at ? 'done' : ''} aria-current={i === at ? 'step' : undefined}>
            <span className="digits">{i < at ? <Icon name="Check" className="icon-sm" /> : i + 1}</span>{s}
          </li>
        ))}
      </ol>
    </nav>
  )
}

export function Guide({ worker, compact = false }: { worker: boolean; compact?: boolean }) {
  const t = useT()
  const lang = useLang()
  const c = worker
    ? {
        eyebrow: t('The easy way to work', 'کام کا آسان راستہ'), title: t('Your skill, your living', 'آپ کا ہنر، آپ کا روزگار'),
        steps: [t('Start with your voice', 'اپنی آواز سے آغاز کریں'), t('Agree the price', 'قیمت طے کریں'), t('Complete the work', 'کام مکمل کریں')],
      }
    : {
        eyebrow: t('How it works', 'یہ کیسے چلتا ہے'), title: t('Agree the work and the price before anyone arrives', 'کسی کے آنے سے پہلے کام اور قیمت طے کریں'),
        steps: [t('Tell us what needs fixing', 'بتائیں کیا ٹھیک کروانا ہے'), t('Agree the scope and price', 'کام اور قیمت طے کریں'), t('Review the completed work', 'کام مکمل ہونے پر ریویو دیں')],
      }
  return (
    <aside className="support-panel" lang={lang}>
      <p className="eyebrow">{c.eyebrow}</p>
      <h2>{c.title}</h2>
      {!compact && <img src="/craft-hero.png" width={1536} height={1024} alt={t('Tools of local skilled trades', 'مقامی کاریگروں کے اوزار')} loading="lazy" />}
      <ol>{c.steps.map((s) => <li key={s}>{s}</li>)}</ol>
    </aside>
  )
}

export function JobAside({ job, worker, children }: { job: Job; worker: boolean; children?: ReactNode }) {
  // Trade, area and distance are already on the JobCard; this panel only adds who the other person is.
  const t = useT()
  const lang = useLang()
  const name = (worker ? job.customer.name : job.worker.name) ?? ''
  return (
    <aside className="support-panel job-aside" lang={lang}>
      <div className="row">
        <span className="avatar">{initials(name || '?')}</span>
        <div className="grow"><h2 dir="auto">{name}</h2><p className="caption muted">{worker ? t('Customer', 'گاہک') : t(TRADES[job.trade].en, TRADES[job.trade].ur)}</p></div>
      </div>
      <p className="note">{worker
        ? t('Agree the price in chat. The job is confirmed when the customer accepts your offer.', 'قیمت بات چیت سے طے کریں۔ گاہک آپ کی پیشکش منظور کرے تو کام پکا۔')
        : t('Agree the scope in chat. A request is confirmed only when the other person accepts an offer.', 'بات چیت میں کام طے کریں۔ درخواست تب ہی پکی ہوتی ہے جب دوسرا شخص پیشکش منظور کرے۔')}</p>
      {children}
    </aside>
  )
}
