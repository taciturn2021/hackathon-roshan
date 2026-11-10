import { useRef, type ReactNode } from 'react'
import { useToast } from '../lib/audio'
import { MOCK } from '../lib/api'
import { LangSwitch, useT } from '../lib/i18n'
import { Link } from '../lib/router'
import { signOut, useMe } from '../lib/session'
import { Icon } from './Icon'

function Brand() {
  const t = useT()
  return (
    <Link className="brand" href="/" aria-label={t('Roshan Rozgar home', 'روشن روزگار ہوم')}>
      <span className="brand-mark"><Icon name="Sunrise" /></span>
      <span><span lang="en" dir="ltr">Roshan Rozgar</span><span className="brand-sub">{t('Good help, close by', 'اچھی مدد، آپ کے قریب')}</span></span>
    </Link>
  )
}

function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
  const t = useT()
  const me = useMe()
  return (
    <>
      <Link href="/#how-it-works" onClick={onNavigate}>{t('How it works', 'طریقہ کار')}</Link>
      <Link href="/#services" onClick={onNavigate}>{t('Trades', 'ہنر')}</Link>
      <Link href="/worker/login" onClick={onNavigate}>{t('For workers', 'کاریگروں کے لیے')}</Link>
      {me
        ? <Link href="/app" onClick={onNavigate}>{t('Open app', 'ایپ کھولیں')}</Link>
        : <Link href="/login" onClick={onNavigate}>{t('Sign in', 'سائن اِن')}</Link>}
    </>
  )
}

export function Shell({ children }: { children: ReactNode }) {
  const t = useT()
  const me = useMe()
  const menu = useRef<HTMLDialogElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const toast = useToast()
  const close = () => { menu.current?.close(); trigger.current?.focus() }
  return (
    <>
      <a className="skip-link" href="#main-content">{t('Skip to content', 'سیدھا مواد پر جائیں')}</a>
      <header className="site-header">
        <div className="site-container header-inner">
          <Brand />
          <nav className="desktop-links" aria-label={t('Main navigation', 'مین مینو')}><NavLinks /></nav>
          <div className="row">
            <LangSwitch />
            {!me && <Link className="button header-cta" href="/start">{t('Get started', 'شروع کریں')}</Link>}
            <button ref={trigger} className="menu-button icon-button" type="button" aria-label={t('Open navigation', 'مینو کھولیں')} aria-controls="site-menu"
              aria-expanded="false" onClick={() => { menu.current?.showModal(); trigger.current?.setAttribute('aria-expanded', 'true') }}>
              <Icon name="Menu" />
            </button>
          </div>
        </div>
      </header>
      <dialog id="site-menu" ref={menu} onClose={() => { trigger.current?.setAttribute('aria-expanded', 'false'); trigger.current?.focus() }}>
        <div className="menu-heading">
          <Brand />
          <button className="icon-button" type="button" aria-label={t('Close navigation', 'مینو بند کریں')} onClick={close}><Icon name="X" /></button>
        </div>
        <nav aria-label={t('Mobile navigation', 'موبائل مینو')}>
          <NavLinks onNavigate={close} />
          {me && <a href="/" onClick={(e) => { e.preventDefault(); close(); signOut() }}>{t('Sign out', 'سائن آؤٹ')}</a>}
        </nav>
        <div style={{ marginTop: 24 }}><LangSwitch /></div>
      </dialog>
      {children}
      <footer className="site-footer">
        <div className="site-container footer-grid">
          <div>
            <Brand />
            <p>{t('Local skills. Clear prices.', 'مقامی ہنر۔ صاف قیمت۔')}<br />{t('Work that earns trust.', 'کام جو بھروسہ بنائے۔')}</p>
            <p>{t('Islamabad & Rawalpindi', 'اسلام آباد اور راولپنڈی')}</p>
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
            <h3>{t('Explore', 'مزید')}</h3><Link href="/#how-it-works">{t('How it works', 'طریقہ کار')}</Link>
            {me && <a href="/" onClick={(e) => { e.preventDefault(); signOut() }}>{t('Sign out', 'سائن آؤٹ')}</a>}
          </div>
        </div>
        <div className="site-container footer-bottom">
          <span lang="en" dir="ltr">© 2026 Roshan Rozgar</span>
          {MOCK && <span>{t('Demo mode · Fictional people and job data · No live bookings', 'ڈیمو موڈ · فرضی لوگ اور فرضی کام · اصل بکنگ نہیں')}</span>}
        </div>
      </footer>
      <div id="preview-toast" role="status" hidden={!toast}>{toast}</div>
    </>
  )
}
