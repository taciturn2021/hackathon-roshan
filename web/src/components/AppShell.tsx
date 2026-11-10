import { createContext, useRef, type ReactNode } from 'react'
import { useToast } from '../lib/audio'
import { MOCK } from '../lib/api'
import { LangSwitch, useLang, useT } from '../lib/i18n'
import { Link, navigate, useLocation } from '../lib/router'
import { signOut, useMe } from '../lib/session'
import { initials } from '../lib/trades'
import { Icon, type IconName } from './Icon'

/** True inside the signed-in app, so shared page layouts drop their marketing container. */
export const InApp = createContext(false)

type NavItem = { href: string; en: string; ur: string; icon: IconName }
const CUSTOMER_NAV: NavItem[] = [
  { href: '/app', en: 'Dashboard', ur: 'ڈیش بورڈ', icon: 'House' },
  { href: '/app/find', en: 'Find a worker', ur: 'کاریگر ڈھونڈیں', icon: 'Search' },
  { href: '/app/jobs', en: 'Jobs', ur: 'کام', icon: 'BriefcaseBusiness' },
]
const WORKER_NAV: NavItem[] = [
  { href: '/app', en: 'Work', ur: 'کام', icon: 'House' },
  { href: '/app/jobs', en: 'My jobs', ur: 'میرے کام', icon: 'BriefcaseBusiness' },
  { href: '/app/profile', en: 'Profile', ur: 'پروفائل', icon: 'User' },
]

const isCurrent = (href: string, path: string) => (href === '/app' ? path === '/app' : path.startsWith(href))

function SidebarContent({ nav, path, worker, name, onNavigate }: { nav: NavItem[]; path: string; worker: boolean; name: string; onNavigate?: () => void }) {
  const t = useT()
  return (
    <>
      <Link className="app-brand" href="/app" onClick={onNavigate}>
        <span className="brand-mark"><Icon name="Sunrise" /></span>
        <span lang="en" dir="ltr">Roshan Rozgar</span>
      </Link>
      <nav className="app-nav" aria-label={t('App navigation', 'مینو')}>
        {nav.map((item) => (
          <Link key={item.href} href={item.href} onClick={onNavigate} aria-current={isCurrent(item.href, path) ? 'page' : undefined}>
            <Icon name={item.icon} />{t(item.en, item.ur)}
          </Link>
        ))}
      </nav>
      <LangSwitch />
      <div className="app-user">
        <span className="avatar">{initials(name || '?')}</span>
        <div className="grow">
          <strong dir="auto">{name}</strong>
          <small>{worker ? t('Worker', 'کاریگر') : t('Customer', 'گاہک')}</small>
        </div>
        <button className="icon-button" type="button" aria-label={t('Sign out', 'سائن آؤٹ')} onClick={() => { onNavigate?.(); signOut() }}>
          <Icon name="LogOut" />
        </button>
      </div>
    </>
  )
}

export function AppShell({ title, children }: { title: string; children: ReactNode }) {
  const t = useT()
  const ur = useLang() === 'ur'
  const me = useMe()
  const { path } = useLocation()
  const drawer = useRef<HTMLDialogElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const toast = useToast()
  if (!me) return null
  const worker = me.user.role === 'worker'
  const nav = worker ? WORKER_NAV : CUSTOMER_NAV
  // Screens below a top-level one get a back button in place of the menu button.
  const sub = !nav.some((item) => item.href === path)
  const name = me.worker?.name ?? me.user.name ?? me.user.email ?? ''
  const close = () => { drawer.current?.close(); trigger.current?.focus() }

  return (
    <div className={`app-shell ${ur ? 'worker-shell' : ''}`}>
      <a className="skip-link" href="#main-content">{t('Skip to content', 'سیدھا مواد پر جائیں')}</a>
      <aside className="app-sidebar"><SidebarContent nav={nav} path={path} worker={worker} name={name} /></aside>
      <dialog className="app-drawer" ref={drawer} onClose={() => trigger.current?.setAttribute('aria-expanded', 'false')}>
        <button className="icon-button app-drawer-close" type="button" aria-label={t('Close menu', 'مینو بند کریں')} onClick={close}><Icon name="X" /></button>
        <SidebarContent nav={nav} path={path} worker={worker} name={name} onNavigate={close} />
      </dialog>
      <div className="app-body">
        <header className="app-topbar">
          {sub ? (
            <button className="icon-button app-back" type="button" aria-label={t('Back', 'واپس')} onClick={() => (history.length > 1 ? history.back() : navigate('/app'))}>
              <Icon name={ur ? 'ArrowRight' : 'ArrowLeft'} />
            </button>
          ) : (
            <button ref={trigger} className="icon-button app-menu" type="button" aria-label={t('Open menu', 'مینو کھولیں')} aria-expanded="false"
            onClick={() => { drawer.current?.showModal(); trigger.current?.setAttribute('aria-expanded', 'true') }}>
            <Icon name="Menu" />
          </button>
          )}
          <h1>{title}</h1>
          <LangSwitch />
          {!worker && !path.startsWith('/app/find') && (
            <Link className="button app-action" href="/app/find" aria-label={t('New request', 'نئی درخواست')}><Icon name="Plus" /><span>{t('New request', 'نئی درخواست')}</span></Link>
          )}
        </header>
        <main id="main-content" className="app-main">
          <InApp.Provider value={true}>{children}</InApp.Provider>
        </main>
        {MOCK && <p className="app-demo-note">{t('Demo mode · fictional people and job data', 'ڈیمو موڈ · فرضی لوگ اور فرضی کام')}</p>}
      </div>
      <div id="preview-toast" role="status" hidden={!toast}>{toast}</div>
    </div>
  )
}
