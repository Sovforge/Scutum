import { Link } from 'react-router-dom'
import SiteHeader from '../../components/SiteHeader/SiteHeader'
import Table from '../../components/ui/Table/Table'
import styles from './About.module.css'

const ENDPOINTS = [
  ['GET', '/health', 'System heartbeat and uptime'],
  ['GET', '/version', 'Version, build hash, and commit ref'],
  ['POST', '/auth/login', 'Authenticate and receive a JWT'],
  ['GET', '/docker/containers', 'List all containers with state'],
  ['POST', '/docker/deploy-compose', 'Deploy a Docker Compose stack'],
  ['GET', '/docker/containers/{id}/stats', 'Live CPU / memory stats for a container'],
  ['GET', '/kubernetes/summary', 'Cluster pod, deployment, and node counts'],
  ['POST', '/kubernetes/apply', 'Apply a Kubernetes YAML manifest'],
  ['POST', '/network/peer', 'Enroll a node into the WireGuard mesh'],
  ['GET', '/network/mesh-summary', 'Healthy vs. total peer count'],
  ['GET', '/nodes', 'List registered mesh nodes'],
  ['GET', '/audit/logs', 'Paginated audit event log'],
  ['GET', '/admin/export', 'Full database export as JSON'],
  ['GET', '/storage/backends', 'List S3-compatible storage backends'],
  ['DELETE', '/plugins/{name}', 'Unload a running plugin'],
] as const

const METHOD_CLASS: Record<string, string> = {
  GET: 'methodGet',
  POST: 'methodPost',
  DELETE: 'methodDelete',
}

function About() {
  return (
    <div className={styles.page}>
      <div className="grain" aria-hidden="true" />
      <SiteHeader />

      <section className={styles.hero}>
        <p className="eyebrow">Dossier · About This Build</p>
        <h1 className={styles.title}>What Scutum actually is.</h1>
        <p className={styles.prose}>
          Scutum is a self-hosted orchestrator for people who run their own hardware — bare
          metal, a home rack, a fleet of mini PCs — and want one control plane across all of it
          without handing a cloud provider the keys.
        </p>
        <p className={styles.prose}>
          Under the hood it's a single Go binary. It meshes your nodes together over WireGuard,
          reconciles Docker and Kubernetes workloads from Git, and keeps an encrypted audit trail
          of everything it does — all without a managed database, message queue, or third-party
          API sitting in the critical path.
        </p>
        <p className={styles.prose}>This page exists mostly as a reference: what's actually running, and what it can do.</p>
      </section>

      <section className={styles.api}>
        <h2 className={styles.apiTitle}>API quick reference</h2>
        <div className={styles.tableFrame}>
          <Table>
            <thead>
              <tr>
                <th>Method</th>
                <th>Endpoint</th>
                <th>Description</th>
              </tr>
            </thead>
            <tbody>
              {ENDPOINTS.map(([method, path, desc]) => (
                <tr key={path}>
                  <td>
                    <span className={`${styles.methodBadge} ${styles[METHOD_CLASS[method]]}`}>{method}</span>
                  </td>
                  <td className={styles.path}>{path}</td>
                  <td>{desc}</td>
                </tr>
              ))}
            </tbody>
          </Table>
        </div>
      </section>

      <footer className={styles.footer}>
        <span className="brand-mark">
          <img src="/logo.svg" alt="" className="brand-mark__logo" />
          <span className="brand-mark__word">SCUTUM</span>
        </span>
        <nav className={styles.footerLinks}>
          <Link to="/">Home</Link>
          <Link to="/whats-new">What's New</Link>
          <Link to="/faq">FAQ</Link>
        </nav>
      </footer>
    </div>
  )
}

export default About
