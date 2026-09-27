import { Link } from 'react-router-dom'
import styles from './SiteHeader.module.css'

function SiteHeader() {
  return (
    <header className={styles.nav}>
      <Link to="/" className="brand-mark">
        <img src="/logo.svg" alt="" className="brand-mark__logo" />
        <span className="brand-mark__word">SCUTUM</span>
      </Link>
      <nav className={styles.navLinks}>
        <Link to="/whats-new">What's New</Link>
        <Link to="/about">About</Link>
        <Link to="/faq">FAQ</Link>
      </nav>
      <Link className={`btn btn--ghost ${styles.navCta}`} to="/login">
        Log In
      </Link>
    </header>
  )
}

export default SiteHeader
