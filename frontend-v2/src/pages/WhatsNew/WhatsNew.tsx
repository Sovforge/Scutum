import { Link } from 'react-router-dom'
import { Command, Globe, Palette } from 'lucide-react'
import SiteHeader from '../../components/SiteHeader/SiteHeader'
import Callout from '../../components/ui/Callout/Callout'
import styles from './WhatsNew.module.css'

const HIGHLIGHTS = [
  {
    icon: Palette,
    title: 'A new visual language',
    body: "The interface was redrawn from scratch — Fraunces and Archivo for type, JetBrains Mono for anything technical, and a color system generated from Scutum's own logo instead of a generic template palette.",
  },
  {
    icon: Command,
    title: 'Faster ways to move',
    body: 'A command palette (⌘K) jumps to any page or action instantly, every change now confirms itself with a toast instead of succeeding silently, and empty states tell you what to do next instead of just being blank.',
  },
  {
    icon: Globe,
    title: 'Built for the ops center',
    body: 'The mesh topology view is now a rotatable, zoomable globe, a brand-new TV mode gives ops centers a dedicated always-on display, and every diagram panel got the same schematic corner framing.',
  },
] as const

const CHANGELOG = [
  {
    term: 'Command palette',
    detail: '⌘K opens a searchable list of every page plus quick actions like the theme toggle and log out — styled as another Scutum console, not a bolted-on widget.',
  },
  {
    term: 'Toast confirmations',
    detail: 'Approving a node, linking a federation peer, collecting traces, scraping metrics — every mutating action confirms itself now instead of just updating silently.',
  },
  {
    term: 'Mesh topology globe',
    detail: 'The old flat diagram is now a draggable, zoomable 3D globe with a hologram-projector base, on the Dashboard, Network, and TV pages.',
  },
  {
    term: 'TV mode',
    detail: 'A brand-new always-on display mode for ops centers — a side-by-side layout so the topology globe actually fills a real TV screen, with the drag/zoom hints hidden since there’s no mouse in the room.',
  },
  {
    term: 'Light & dark themes',
    detail: 'A real toggle in the account menu, not just a permanently dark shell.',
  },
  {
    term: 'Empty states',
    detail: 'Every list and table that can be empty now tells you what to do about it, instead of just rendering nothing.',
  },
  {
    term: 'Report headers & manifests',
    detail: 'Every app page opens with a live status readout instead of a bare title, and rosters/tables are numbered like a field manifest.',
  },
  {
    term: 'New component library',
    detail: 'Section, StatCard, Table, Tabs, Toast, Command Palette, Empty State — all built fresh for this rebuild instead of pulling in a generic UI kit.',
  },
] as const

const IN_PROGRESS = [
  'Terminal sessions simulate a shell locally — there’s no live WebSocket exec wired up yet.',
  'The Logs tab only tails the Application source for now; Kubernetes and Docker log tailing isn’t live yet.',
  '"Collect" and "Scrape" actions on the Observability page append a placeholder row rather than performing a real collection.',
]

function WhatsNew() {
  return (
    <div className={styles.page}>
      <div className="grain" aria-hidden="true" />
      <SiteHeader />

      <section className={styles.hero}>
        <p className="eyebrow">Release Notes · Frontend v2</p>
        <h1 className={styles.title}>New face. Same engine.</h1>
        <p className={styles.prose}>
          Scutum's web UI was rebuilt from the ground up — a fresh design system and a handful of
          new interactions, with no changes to the backend, the API, or your existing mesh.
        </p>
      </section>

      <section className={styles.highlights}>
        {HIGHLIGHTS.map(({ icon: Icon, title, body }) => (
          <div className={styles.card} key={title}>
            <span className={styles.cardIcon}>
              <Icon size={20} />
            </span>
            <h2 className={styles.cardTitle}>{title}</h2>
            <p className={styles.cardBody}>{body}</p>
          </div>
        ))}
      </section>

      <section className={styles.changelog}>
        <h2 className={styles.sectionTitle}>The full list</h2>
        <dl className={styles.changelogList}>
          {CHANGELOG.map(({ term, detail }) => (
            <div className={styles.changelogRow} key={term}>
              <dt>{term}</dt>
              <dd>{detail}</dd>
            </div>
          ))}
        </dl>
      </section>

      <section className={styles.inProgress}>
        <h2 className={styles.sectionTitle}>Still catching up</h2>
        <p className={styles.prose}>
          Not everything is wired to a live backend yet — this is a frontend rebuild first, and
          we'd rather say so than fake it:
        </p>
        <ul className={styles.inProgressList}>
          {IN_PROGRESS.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </section>

      <section className={styles.reassurance}>
        <div className={styles.calloutWrap}>
          <Callout variant="info">
            <strong>Nothing changed underneath.</strong> Same single Go binary, same API
            endpoints, same WireGuard mesh — this is a frontend rebuild, not a migration. Your
            nodes, containers, and audit history are exactly where you left them.
          </Callout>
        </div>
        <Link className="btn btn--primary" to="/login">
          See it for yourself
        </Link>
      </section>

      <footer className={styles.footer}>
        <span className="brand-mark">
          <img src="/logo.svg" alt="" className="brand-mark__logo" />
          <span className="brand-mark__word">SCUTUM</span>
        </span>
        <nav className={styles.footerLinks}>
          <Link to="/">Home</Link>
          <Link to="/about">About</Link>
          <Link to="/faq">FAQ</Link>
        </nav>
      </footer>
    </div>
  )
}

export default WhatsNew
