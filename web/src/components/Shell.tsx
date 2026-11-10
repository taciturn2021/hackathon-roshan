import { useRef, type ReactNode } from 'react'
import { useToast } from '../lib/audio'
import { MOCK } from '../lib/api'
import { Link } from '../lib/router'
import { signOut, useMe } from '../lib/session'
import { Icon } from './Icon'

function Brand() {
  return (
    <Link className="brand" href="/" aria-label="Roshan Rozgar home">
      <span className="brand-mark"><Icon name="Sunrise" /></span>
      <span>Roshan Rozgar<span className="brand-sub">Good help, close by</span></span>
    </Link>
  )
}

function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
  const me = useMe()
  return (
    <>
      <Link href="/#how-it-works" onClick={onNavigate}>How it works</Link>
      <Link href="/#services" onClick={onNavigate}>Trades</Link>
      <Link href="/worker/login" onClick={onNavigate}>For workers</Link>
      {me ? <Link href="/app" onClick={onNavigate}>Open app</Link> : <Link href="/login" onClick={onNavigate}>Sign in</Link>}
    </>
  )
}

export function Shell({ children }: { children: ReactNode }) {
  const me = useMe()
  const menu = useRef<HTMLDialogElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const toast = useToast()
  const close = () => { menu.current?.close(); trigger.current?.focus() }
  return (
    <>
      <a className="skip-link" href="#main-content">Skip to content</a>
      <header className="site-header">
        <div className="site-container header-inner">
          <Brand />
          <nav className="desktop-links" aria-label="Main navigation"><NavLinks /></nav>
          {!me && <Link className="button header-cta" href="/start">Get started</Link>}
          <button ref={trigger} className="menu-button icon-button" type="button" aria-label="Open navigation" aria-controls="site-menu"
            aria-expanded="false" onClick={() => { menu.current?.showModal(); trigger.current?.setAttribute('aria-expanded', 'true') }}>
            <Icon name="Menu" />
          </button>
        </div>
      </header>
      <dialog id="site-menu" ref={menu} onClose={() => { trigger.current?.setAttribute('aria-expanded', 'false'); trigger.current?.focus() }}>
        <div className="menu-heading">
          <Brand />
          <button className="icon-button" type="button" aria-label="Close navigation" onClick={close}><Icon name="X" /></button>
        </div>
        <nav aria-label="Mobile navigation">
          <NavLinks onNavigate={close} />
          {me && <a href="/" onClick={(e) => { e.preventDefault(); close(); signOut() }}>Sign out</a>}
        </nav>
      </dialog>
      {children}
      <footer className="site-footer">
        <div className="site-container footer-grid">
          <div><Brand /><p>Local skills. Clear prices.<br />Work that earns trust.</p><p>Islamabad &amp; Rawalpindi</p></div>
          <div><h3>For your home</h3><Link href="/app/find">Find local help</Link><Link href="/app/jobs">Your jobs</Link></div>
          <div><h3>For your work</h3><Link href="/worker/login">Join as a worker</Link><Link href="/app">Your availability</Link></div>
          <div>
            <h3>Explore</h3><Link href="/#how-it-works">How it works</Link>
            {me && <a href="/" onClick={(e) => { e.preventDefault(); signOut() }}>Sign out</a>}
          </div>
        </div>
        <div className="site-container footer-bottom">
          <span>© 2026 Roshan Rozgar</span>
          {MOCK && <span>Demo mode · Fictional people and job data · No live bookings</span>}
        </div>
      </footer>
      <div id="preview-toast" role="status" hidden={!toast}>{toast}</div>
    </>
  )
}
