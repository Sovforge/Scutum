import { Link } from 'react-router-dom'
import SiteHeader from '../../components/SiteHeader/SiteHeader'
import headerStyles from '../../components/SiteHeader/SiteHeader.module.css'
import styles from './Landing.module.css'

const PRINCIPLES = [
  {
    num: '01',
    title: 'Absolute sovereignty',
    body: 'No cloud control plane sits between your nodes. No relay servers phone home. What you deploy answers to you and no one else.',
  },
  {
    num: '02',
    title: 'Container-first',
    body: 'Built for immutable hosts — Alpine, Talos-style images, ZimaOS. One binary, one image, nothing to assemble on-site.',
  },
  {
    num: '03',
    title: 'Universal networking',
    body: 'Kernel WireGuard where the host allows it, a userspace tunnel where it does not. The mesh forms either way.',
  },
  {
    num: '04',
    title: 'Zero external dependencies',
    body: "No managed database, no message broker, no third-party API required to boot. If it's not a core concern, it doesn't belong in the core — write a WASM-sandboxed plugin instead.",
  },
] as const

const MANIFEST = [
  ['Runtime', 'Go 1.24+ — static binary'],
  ['Networking', 'WireGuard, kernel + userspace fallback'],
  ['State storage', 'Encrypted SQLite by default · Postgres · MySQL'],
  ['Observability', 'OpenTelemetry — logs, metrics, traces'],
  ['Runtimes', 'Docker · Kubernetes'],
  ['GitOps', 'Git-native manifest reconciliation'],
  ['Storage', 'S3-compatible — MinIO, R2, AWS S3, B2, Ceph'],
  ['Distribution', 'GHCR, SBOM attached to every image'],
] as const

const DOCKER_CMD = `docker pull ghcr.io/sovforge/scutum:latest

docker run -d \\
  --name scutum \\
  --restart unless-stopped \\
  -v /app/data:/app/data \\
  -v /app/secrets:/app/secrets \\
  -p 8080:8080 \\
  ghcr.io/sovforge/scutum:latest`

// Echoes the Scutum shield mark: a shouldered silhouette tapering to a
// point, drawn as nested contour lines the way the logo's outline reads,
// with the mesh's hub sitting where the logo's "S" sits.
const SHIELD_PATH =
  'M88,58 C115,42 155,28 200,28 C245,28 285,42 312,58 L312,175 ' +
  'C312,255 300,300 270,335 C255,352 228,372 200,388 ' +
  'C172,372 145,352 130,335 C100,300 88,255 88,175 Z'

function MeshDiagram() {
  const edges: [number, number][] = [
    [0, 1], [0, 2], [0, 3], [0, 4], [0, 5], [0, 6],
  ]
  const dashedEdges: [number, number][] = [
    [1, 6], [2, 6], [3, 5], [4, 5],
  ]
  const nodes = [
    { x: 200, y: 195, r: 11, hub: true },
    { x: 120, y: 75, r: 6 },
    { x: 280, y: 75, r: 6 },
    { x: 290, y: 220, r: 6 },
    { x: 110, y: 220, r: 6 },
    { x: 200, y: 330, r: 6 },
    { x: 200, y: 100, r: 6 },
  ]

  return (
    <svg className={styles.mesh} viewBox="0 0 400 416" aria-hidden="true">
      <path className={styles.meshShield} d={SHIELD_PATH} />
      <path
        className={`${styles.meshShield} ${styles.meshShieldInner}`}
        d={SHIELD_PATH}
        transform="translate(200 208) scale(0.88) translate(-200 -208)"
      />
      <path className={styles.meshFlare} d="M170,382 C140,400 110,408 78,404" />
      <path className={styles.meshFlare} d="M230,382 C260,400 290,408 322,404" />
      {dashedEdges.map(([a, b], i) => (
        <line
          key={`d${i}`}
          className={`${styles.meshEdge} ${styles.meshEdgeDashed}`}
          x1={nodes[a].x}
          y1={nodes[a].y}
          x2={nodes[b].x}
          y2={nodes[b].y}
        />
      ))}
      {edges.map(([a, b], i) => (
        <line
          key={i}
          className={styles.meshEdge}
          x1={nodes[a].x}
          y1={nodes[a].y}
          x2={nodes[b].x}
          y2={nodes[b].y}
          style={{ animationDelay: `${i * 0.3}s` }}
        />
      ))}
      {nodes.map((n, i) => (
        <g key={i} transform={`translate(${n.x} ${n.y})`}>
          <circle
            className={n.hub ? `${styles.meshNode} ${styles.meshNodeHub}` : styles.meshNode}
            r={n.r}
          />
          <line className={styles.meshCross} x1={-n.r - 5} x2={-n.r + 1} y1="0" y2="0" />
          <line className={styles.meshCross} x1={n.r - 1} x2={n.r + 5} y1="0" y2="0" />
        </g>
      ))}
    </svg>
  )
}

function Landing() {
  return (
    <div className={styles.page}>
      <div className="grain" aria-hidden="true" />

      <SiteHeader />

      <section className={styles.hero}>
        <div>
          <p className={`eyebrow ${styles.anim}`} style={{ animationDelay: '0.05s' }}>
            Self-Hosted Orchestration
          </p>
          <h1 className={`${styles.heroHeadline} ${styles.anim}`} style={{ animationDelay: '0.15s' }}>
            Own the iron.
            <br />
            Command your <em>own</em> mesh.
          </h1>
          <p className={`${styles.heroSub} ${styles.anim}`} style={{ animationDelay: '0.28s' }}>
            Scutum is sovereign orchestration for people who build their own infrastructure —
            not rent it. No cloud control plane. No relays. No vendor lock-in.
          </p>
          <div className={`${styles.heroCta} ${styles.anim}`} style={{ animationDelay: '0.4s' }}>
            <Link className="btn btn--primary" to="/setup">
              Get Started
            </Link>
            <Link className="btn btn--ghost" to="/login">
              Log In
            </Link>
          </div>
          <div className={`${styles.heroBadges} ${styles.anim}`} style={{ animationDelay: '0.52s' }}>
            <span className={styles.stamp}>Zero Cloud Dependencies</span>
            <span className={`${styles.stamp} ${styles.stampAlt}`}>Open Core</span>
            <span className={styles.stamp}>Peer-to-Peer</span>
          </div>
        </div>
        <div className={`${styles.heroDiagram} ${styles.anim}`} style={{ animationDelay: '0.3s' }}>
          <MeshDiagram />
          <span className={styles.heroDiagramLabel}>FIG. 1 — MESH TOPOLOGY, UNMANAGED</span>
        </div>
      </section>

      <section id="doctrine" className={styles.doctrine}>
        <p className="eyebrow">The Doctrine</p>
        <h2 className={styles.doctrineTitle}>Four articles, non-negotiable.</h2>
        <div className={styles.doctrineGrid}>
          {PRINCIPLES.map((p) => (
            <article className={styles.doctrineCard} key={p.num}>
              <span className={styles.doctrineNum}>{p.num}</span>
              <h3 className={styles.doctrineCardTitle}>{p.title}</h3>
              <p className={styles.doctrineCardBody}>{p.body}</p>
            </article>
          ))}
        </div>
      </section>

      <section id="manifest" className={styles.manifest}>
        <div className={styles.manifestStamp}>Verified<br />Open Core</div>
        <p className="eyebrow eyebrow--dark">Cargo Manifest</p>
        <h2 className={styles.manifestTitle}>What's actually in the crate.</h2>
        <dl className={styles.manifestList}>
          {MANIFEST.map(([label, value]) => (
            <div className={styles.manifestRow} key={label}>
              <dt>{label}</dt>
              <dd>{value}</dd>
            </div>
          ))}
        </dl>
      </section>

      <section id="deploy" className={styles.deploy}>
        <div>
          <p className="eyebrow">Requisition</p>
          <h2 className={styles.deployTitle}>Ready when you are.</h2>
          <p className={styles.deploySub}>
            One image. One command. Every image ships with a full SBOM, so you can verify
            exactly what you're running before it touches your network.
          </p>
        </div>
        <div className={styles.terminal}>
          <div className={styles.terminalBar}>
            <span className={styles.terminalDot} />
            <span className={styles.terminalDot} />
            <span className={styles.terminalDot} />
            <span className={styles.terminalBarLabel}>requisition.sh</span>
          </div>
          <pre className={styles.terminalBody}>{DOCKER_CMD}</pre>
        </div>
      </section>

      <footer className={styles.footer}>
        <div className={styles.footerTop}>
          <span className="brand-mark">
            <img src="/logo.svg" alt="" className="brand-mark__logo" />
            <span className="brand-mark__word">SCUTUM</span>
          </span>
          <nav className={headerStyles.navLinks}>
            <a href="#doctrine">Doctrine</a>
            <a href="#manifest">Stack</a>
            <a href="#deploy">Self-hosting</a>
            <Link to="/whats-new">What's New</Link>
            <Link to="/about">About</Link>
            <Link to="/faq">FAQ</Link>
          </nav>
        </div>
        <p className={styles.footerTicker}>
          Secure — Peer-to-peer — Zero cloud dependencies — Open-core — Free for personal &amp;
          non-commercial use — No artificial node limits —
        </p>
      </footer>
    </div>
  )
}

export default Landing
