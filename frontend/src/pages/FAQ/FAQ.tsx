import { Link } from 'react-router-dom'
import SiteHeader from '../../components/SiteHeader/SiteHeader'
import styles from './FAQ.module.css'

const FAQS = [
  {
    q: 'Why was Scutum built in the first place?',
    a: "Immutable operating systems — Talos-style images, ZimaOS, read-only Alpine hosts — don't let you SSH in and hand-configure an agent or drop files wherever you like. Scutum was built to manage that kind of infrastructure without ever needing shell access to the host: one static binary embeds everything, and the web-based terminal gives you exec access into containers and pods directly through the mesh instead of SSH into the machine itself.",
  },
  {
    q: 'Who is this actually for?',
    a: 'People running their own hardware who are tired of vendor lock-in — security-conscious teams with compliance requirements, edge and IoT operators running air-gapped or high-latency sites, and anyone who wants one pane of glass instead of five separate dashboards.',
  },
  {
    q: 'How is this different from Tailscale or Headscale?',
    a: "Those are excellent at one thing: the network overlay. Scutum starts from the same encrypted WireGuard mesh, then adds what you'd otherwise need separate tools for — GitOps-driven container and Kubernetes deployment, state management, and observability — all in the same binary.",
  },
  {
    q: 'How is this different from running Kubernetes or Nomad?',
    a: "Kubernetes is powerful but heavy — etcd, a control plane, ingress controllers, all before you've deployed a workload. Scutum is a single static binary with no external control-plane dependencies, built for edge and sovereign infrastructure where operational simplicity matters more than infinite extensibility.",
  },
  {
    q: 'Do I need a public IP address to use it?',
    a: 'At least one node — the Hub — needs one, since it brokers the initial handshake between peers behind NAT or CGNAT. A $5/month VPS is enough. Once two nodes have exchanged WireGuard keys through the Hub, traffic flows directly between them; the Hub is never in the data path.',
  },
  {
    q: 'Does my traffic get routed through the Hub?',
    a: 'No. The Hub only signals — it introduces peers and helps them find each other through NAT. Once the handshake completes, all traffic moves peer-to-peer over the encrypted mesh.',
  },
  {
    q: 'Is Scutum free to use?',
    a: "Yes, for personal use, academic institutions, and small businesses (under 100 people and $1M revenue). It's licensed under PolyForm Small Business — larger organizations need a commercial agreement. There are no artificial node limits at any tier.",
  },
  {
    q: 'Can I contribute code?',
    a: "Scutum is solo-authored by design, so pull requests adding core features aren't accepted right now. Feature requests and bug reports genuinely shape the roadmap, though — open an issue on GitHub.",
  },
  {
    q: 'Does it need a database or other external services to run?',
    a: "No. It ships with an encrypted SQLite store by default and needs nothing else to boot. Postgres or MySQL are available if you need high availability across multiple replicas, but they're opt-in, not required.",
  },
  {
    q: "What if I don't want to run it in Docker?",
    a: 'You can build and run the Go binary directly, but Docker handles privilege and networking requirements Scutum needs — like WireGuard and iptables — automatically. Running outside it is really meant for development, and some features may need extra setup.',
  },
] as const

function FAQ() {
  return (
    <div className={styles.page}>
      <div className="grain" aria-hidden="true" />
      <SiteHeader />

      <section className={styles.hero}>
        <p className="eyebrow">Frequently Asked</p>
        <h1 className={styles.title}>Questions, answered plainly.</h1>
        <p className={styles.sub}>
          No marketing spin — just the questions people actually ask before running this on their
          own hardware.
        </p>
      </section>

      <section className={styles.list}>
        {FAQS.map(({ q, a }) => (
          <details className={styles.item} key={q}>
            <summary className={styles.question}>
              {q}
              <span className={styles.icon}>+</span>
            </summary>
            <p className={styles.answer}>{a}</p>
          </details>
        ))}
      </section>

      <footer className={styles.footer}>
        <span className="brand-mark">
          <img src="/logo.svg" alt="" className="brand-mark__logo" />
          <span className="brand-mark__word">SCUTUM</span>
        </span>
        <nav className={styles.footerLinks}>
          <Link to="/">Home</Link>
          <Link to="/whats-new">What's New</Link>
          <Link to="/about">About</Link>
        </nav>
      </footer>
    </div>
  )
}

export default FAQ
