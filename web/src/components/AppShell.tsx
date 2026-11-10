import { createContext, useRef, type ReactNode } from 'react'
import { useToast } from '../lib/audio'
import { MOCK } from '../lib/api'
import { Link, useLocation } from '../lib/router'
import { signOut, useMe } from '../lib/session'
import { initials } from '../lib/trades'
import { Icon, type IconName } from './Icon'

/** True inside the signed-in app, so shared page layouts drop their marketing container. */
export const InApp = createContext(false)

type NavItem = { href: string; label: string; icon: IconName }
const CUSTOMER_NAV: NavItem[] = [
  { href: '/app', label: 'Dashboard', icon: 'House' },
  { href: '/app/find', label: 'Find a worker', icon: 'Search' },
  { href: '/app/jobs', label: 'Jobs', icon: 'BriefcaseBusiness' },
]
const WORKER_NAV: NavItem[] = [
  { href: '/app', label: 'کام', icon: 'House' },
  { href: '/app/jobs', label: 'میرے کام', icon: 'BriefcaseBusiness' },
  { href: '/app/profile', label: 'پروفائل', icon: 'User' },
]

const isCurrent = (href: string, path: string) => (href === '/app' ? path === '/app' : path.startsWith(href))

function SidebarContent({ nav, path, worker, name, onNavigate }: { nav: NavItem[]; path: string; worker: boolean; name: string; onNavigate?: () => void }) {
  return (
    <>
      <Link className="app-brand" href="/app" onClick={onNavigate}>
        <span className="brand-mark"><Icon name="Sunrise" /></span>
        <span lang="en" dir="ltr">Roshan Rozgar</span>
      </Link>
      <nav className="app-nav" aria-label={worker ? 'مینو' : 'App navigation'}>
        {nav.map((item) => (
          <Link key={item.href} href={item.href} onClick={onNavigate} aria-current={isCurrent(item.href, path) ? 'page' : undefined}>
            <Icon name={item.icon} />{item.label}
          </Link>
        ))}
      </nav>
      <div className="app-user">
        <span className="avatar">{initials(name || '?')}</span>
        <div className="grow">
          <strong dir="auto">{name}</strong>
          <small>{worker ? 'کاریگر' : 'Customer'}</small>
        </div>
        <button className="icon-button" type="button" aria-label={worker ? 'سائن آؤٹ' : 'Sign out'} onClick={() => { onNavigate?.(); signOut() }}>
          <Icon name="LogOut" />
        </button>
      </div>
    </>
  )
}

export function AppShell({ title, children }: { title: string; children: ReactNode }) {
  const me = useMe()
  const { path } = useLocation()
  const drawer = useRef<HTMLDialogElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const toast = useToast()
  if (!me) return null
  const worker = me.user.role === 'worker'
  const nav = worker ? WORKER_NAV : CUSTOMER_NAV
  const name = me.worker?.name ?? me.user.name ?? me.user.email ?? ''
  const close = () => { drawer.current?.close(); trigger.current?.focus() }

  return (
    <div className={`app-shell ${worker ? 'worker-shell' : ''}`} {...(worker ? { lang: 'ur', dir: 'rtl' } : {})}>
      <a className="skip-link" href="#main-content">Skip to content</a>
      <aside className="app-sidebar"><SidebarContent nav={nav} path={path} worker={worker} name={name} /></aside>
      <dialog className="app-drawer" ref={drawer} onClose={() => trigger.current?.setAttribute('aria-expanded', 'false')}>
        <button className="icon-button app-drawer-close" type="button" aria-label={worker ? 'مینو بند کریں' : 'Close menu'} onClick={close}><Icon name="X" /></button>
        <SidebarContent nav={nav} path={path} worker={worker} name={name} onNavigate={close} />
      </dialog>
      <div className="app-body">
        <header className="app-topbar">
          <button ref={trigger} className="icon-button app-menu" type="button" aria-label={worker ? 'مینو کھولیں' : 'Open menu'} aria-expanded="false"
            onClick={() => { drawer.current?.showModal(); trigger.current?.setAttribute('aria-expanded', 'true') }}>
            <Icon name="Menu" />
          </button>
          <h1>{title}</h1>
          {!worker && !path.startsWith('/app/find') && (
            <Link className="button app-action" href="/app/find" aria-label="New request"><Icon name="Plus" /><span>New request</span></Link>
          )}
        </header>
        <main id="main-content" className="app-main">
          <InApp.Provider value={true}>{children}</InApp.Provider>
        </main>
        {MOCK && <p className="app-demo-note" lang="en" dir="ltr">Demo mode · fictional people and job data</p>}
      </div>
      <div id="preview-toast" role="status" hidden={!toast}>{toast}</div>
    </div>
  )
}
