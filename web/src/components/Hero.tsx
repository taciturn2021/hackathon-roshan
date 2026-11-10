import { Icon, type IconName } from './Icon'

export type HeroTone = 'idle' | 'wait' | 'go' | 'work' | 'done' | 'stop'

/**
 * The big coloured card at the top of a screen that says what state things are in and what to do next.
 * Each state has its own colour and icon so screens are told apart at a glance, without reading:
 * idle = grey, wait = amber, go = green, work = blue, done = deep green, stop = red.
 */
export function Hero({ tone, icon, title, hint }: { tone: HeroTone; icon: IconName; title: string; hint?: string }) {
  return (
    <section className={`hero-card ${tone}`} role="status">
      <span className="hero-icon"><Icon name={icon} /></span>
      <div><h1>{title}</h1>{hint && <p>{hint}</p>}</div>
    </section>
  )
}

/** Numbered steps of a flow with the current one marked: earlier steps show a tick. */
export function Steps({ steps, at, label }: { steps: string[]; at: number; label: string }) {
  return (
    <ol className="flow-steps" aria-label={label}>
      {steps.map((s, i) => (
        <li key={s} className={i < at ? 'done' : i === at ? 'current' : ''} aria-current={i === at ? 'step' : undefined}>
          <span className="digits">{i < at ? <Icon name="Check" className="icon-sm" /> : i + 1}</span>{s}
        </li>
      ))}
    </ol>
  )
}
