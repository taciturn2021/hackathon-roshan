import { useRef, type ReactNode } from 'react'
import { useToast } from '../lib/audio'
import { MOCK } from '../lib/api'
import { LangSwitch, useT } from '../lib/i18n'
import { Link, useLocation } from '../lib/router'
import { signOut, useMe } from '../lib/session'
import { Icon } from './Icon'
import { Wordmark } from './Wordmark'

function Brand() {
  const t = useT()
  return (
    <Link className="brand" href="/" aria-label={t('Roshan Rozgar home', 'روشن روزگار ہوم')}>
      <Wordmark />
    </Link>
  )
}

/** The centre links. Sign-in is never a generic link: it is one of the two role doors (RoleDoors). */
function NavLinks({ onNavigate, withApp = false }: { onNavigate?: () => void; withApp?: boolean }) {
  const t = useT()
  const me = useMe()
  return (
    <>
      <Link href="/#how-it-works" onClick={onNavigate}>{t('How it works', 'طریقہ کار')}</Link>
      <Link href="/#services" onClick={onNavigate}>{t('Trades', 'ہنر')}</Link>
      {withApp && me && <Link href="/app" onClick={onNavigate}>{t('Open app', 'ایپ کھولیں')}</Link>}
    </>
  )
}

/**
 * The two ways in, named by who they are for, with picture, Urdu and English together, so a worker never
 * lands on the customer page by tapping a generic "Sign in". `big` is the phone menu's full-width version.
 */
function RoleDoors({ onNavigate, big = false }: { onNavigate?: () => void; big?: boolean }) {
  return (
    <div className={big ? 'role-doors big' : 'role-doors'}>
      <Link className="role-door customer-role" href="/login" onClick={onNavigate}>
        <Icon name="House" />
        <span className="role-words"><span className="role-en" lang="en" dir="ltr">Find a worker</span><span className="role-ur" lang="ur" dir="rtl">کاریگر ڈھونڈیں</span></span>
      </Link>
      <Link className="role-door worker-role" href="/worker/login" onClick={onNavigate}>
        <Icon name="Wrench" />
        <span className="role-words"><span className="role-en" lang="en" dir="ltr">I’m a worker</span><span className="role-ur" lang="ur" dir="rtl">میں کاریگر ہوں</span></span>
      </Link>
    </div>
  )
}

export function Shell({ children }: { children: ReactNode }) {
  const t = useT()
  const me = useMe()
  const menu = useRef<HTMLDialogElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const toast = useToast()
  const { path } = useLocation()
  // Signing in and joining are single tasks: the marketing links and footer would only distract.
  const focused = /^\/(start|login|signup|worker)(\/|$)/.test(path)
  const close = () => { menu.current?.close(); trigger.current?.focus() }
  return (
    <>
      <a className="skip-link" href="#main-content">{t('Skip to content', 'سیدھا مواد پر جائیں')}</a>
      <header className={`site-header ${focused ? 'focused' : ''}`}>
        <div className="site-container header-inner">
          <Brand />
          {!focused && <nav className="desktop-links" aria-label={t('Main navigation', 'مین مینو')}><NavLinks /></nav>}
          <div className="row">
            <LangSwitch />
            {!focused && (me ? <Link className="header-pill" href="/app">{t('Open app', 'ایپ کھولیں')}</Link> : <RoleDoors />)}
            {!focused && <button ref={trigger} className="menu-button icon-button" type="button" aria-label={t('Open navigation', 'مینو کھولیں')} aria-controls="site-menu"
              aria-expanded="false" onClick={() => { menu.current?.showModal(); trigger.current?.setAttribute('aria-expanded', 'true') }}>
              <Icon name="Menu" />
            </button>}
          </div>
        </div>
      </header>
      <dialog id="site-menu" ref={menu} onClose={() => { trigger.current?.setAttribute('aria-expanded', 'false'); trigger.current?.focus() }}>
        <div className="menu-heading">
          <Brand />
          <button className="icon-button" type="button" aria-label={t('Close navigation', 'مینو بند کریں')} onClick={close}><Icon name="X" /></button>
        </div>
        {!me && <RoleDoors onNavigate={close} big />}
        <nav aria-label={t('Mobile navigation', 'موبائل مینو')}>
          <NavLinks onNavigate={close} withApp />
          {me && <a href="/" onClick={(e) => { e.preventDefault(); close(); signOut() }}>{t('Sign out', 'سائن آؤٹ')}</a>}
        </nav>
        <div style={{ marginTop: 24 }}><LangSwitch /></div>
      </dialog>
      {children}
      {!focused && <div className="footer-wrap"><footer className="site-footer">
        <div className="site-container footer-grid">
          <div>
            <Brand />
            <p>{t('Local skills. Clear prices.', 'مقامی ہنر۔ صاف قیمت۔')}<br />{t('Work that earns trust.', 'کام جو بھروسہ بنائے۔')}</p>
          </div>
          <div>
            <h3>{t('For your home', 'آپ کے گھر کے لیے')}</h3>
            <Link href="/app/find">{t('Find local help', 'قریب کا کاریگر ڈھونڈیں')}</Link>
            <Link href="/app/jobs">{t('Your jobs', 'آپ کے کام')}</Link>
          </div>
          <div>
            <h3>{t('For your work', 'آپ کے روزگار کے لیے')}</h3>
            <Link href="/worker/login">{t('Join as a worker', 'کاریگر بن کر شامل ہوں')}</Link>
            <Link href="/app">{t('Your availability', 'آپ کب فارغ ہیں')}</Link>
          </div>
          <div>
            <h3>{t('Explore', 'مزید')}</h3>
            <Link href="/#how-it-works">{t('How it works', 'طریقہ کار')}</Link>
            <Link href="/#services">{t('Trades', 'ہنر')}</Link>
            {me && <a href="/" onClick={(e) => { e.preventDefault(); signOut() }}>{t('Sign out', 'سائن آؤٹ')}</a>}
          </div>
        </div>
        <div className="site-container footer-bottom">
          <span lang="en" dir="ltr">© 2026 Roshan Rozgar</span>
          <span>{t('Islamabad & Rawalpindi', 'اسلام آباد اور راولپنڈی')}</span>
          {MOCK && <span>{t('Demo mode · Fictional people and job data · No live bookings', 'ڈیمو موڈ · فرضی لوگ اور فرضی کام · اصل بکنگ نہیں')}</span>}
        </div>
      </footer></div>}
      <div id="preview-toast" role="status" hidden={!toast}>{toast}</div>
    </>
  )
}
