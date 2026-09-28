import { useEffect, useState, type FormEvent } from 'react'
import { Lock, RefreshCw, ShieldCheck } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import Callout from '../../../components/ui/Callout/Callout'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import { ApiError, getCertificates, rotateCertificate, type CertificatesResponse, type CertInfo } from '../../../lib/api'
import styles from '../Settings.module.css'

// Real inventory of the hub's own TLS server cert and, if mTLS is
// configured, the CA it uses to verify inbound client certs — this mesh
// doesn't issue per-node x509 certs (node identity is a WireGuard key), so
// there's nothing per-node to show here, only these two hub-level slots.
// Rotation writes straight to CERT_FILE/KEY_FILE/CA_CERT_FILE and swaps the
// live TLS listener's config — no restart required.
function fmtTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function roleLabel(role: CertInfo['role']): string {
  return role === 'server' ? 'Server certificate' : 'Client CA (mTLS)'
}

function CertificatesTab() {
  const toast = useToast()
  const [data, setData] = useState<CertificatesResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [apiError, setApiError] = useState('')
  const [rotating, setRotating] = useState<CertInfo['role'] | null>(null)

  const [certPem, setCertPem] = useState('')
  const [keyPem, setKeyPem] = useState('')
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  async function load() {
    setLoading(true)
    setApiError('')
    try {
      setData(await getCertificates())
    } catch (e) {
      setApiError(e instanceof ApiError ? e.message : 'Failed to load certificates')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  function startRotate(role: CertInfo['role']) {
    setRotating((prev) => (prev === role ? null : role))
    setCertPem('')
    setKeyPem('')
    setFormError('')
  }

  async function submitRotate(e: FormEvent, role: CertInfo['role']) {
    e.preventDefault()
    if (!certPem) {
      setFormError('Certificate PEM is required.')
      return
    }
    if (role === 'server' && !keyPem) {
      setFormError('Private key PEM is required when rotating the server certificate.')
      return
    }
    setFormError('')
    setSaving(true)
    try {
      await rotateCertificate(role, certPem, role === 'server' ? keyPem : undefined)
      toast(`${roleLabel(role)} rotated`)
      setRotating(null)
      load()
    } catch (e2) {
      setFormError(e2 instanceof ApiError ? e2.message : 'Rotation failed')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <Section>
        <div className={styles.loadingRow}>Loading…</div>
      </Section>
    )
  }

  if (apiError) {
    return (
      <Section>
        <Callout variant="danger">{apiError}</Callout>
      </Section>
    )
  }

  if (!data || data.mode === 'none') {
    return (
      <Section>
        <EmptyState icon={Lock} title="TLS is not enabled" description="Set CERT_FILE/KEY_FILE (or ACME_DOMAIN/ACME_EMAIL) to enable HTTPS on this hub." />
      </Section>
    )
  }

  if (data.mode === 'acme') {
    return (
      <Section>
        <Callout variant="success">
          Certificates are managed automatically via ACME/Let's Encrypt — the hub renews them itself well before
          expiry. There's nothing to rotate manually here.
        </Callout>
      </Section>
    )
  }

  return (
    <>
      <Callout variant="info">
        This mesh doesn't issue per-node certificates — node identity is a WireGuard key, verified alongside an
        HMAC signature on every hub↔node request, not an x509 cert. The two slots below are the hub's own HTTPS
        server certificate, and, if mTLS is enabled, the CA it uses to verify client certs on inbound connections.
        A cert expiring in under {data.warn_days} days is flagged and triggers a webhook.
      </Callout>

      {data.certificates.length === 0 ? (
        <Section>
          <EmptyState icon={ShieldCheck} title="No certificate loaded" description="CERT_FILE/KEY_FILE are configured but couldn't be read." />
        </Section>
      ) : (
        data.certificates.map((c) => (
          <Section key={c.role} title={roleLabel(c.role)} action={
            <Button variant="ghost" onClick={() => startRotate(c.role)}>
              <RefreshCw size={14} />
              Rotate
            </Button>
          }>
            <Table>
              <tbody>
                <tr>
                  <td className="cell-muted">Subject</td>
                  <td className="cell-name">{c.subject}</td>
                </tr>
                <tr>
                  <td className="cell-muted">Issuer</td>
                  <td>{c.issuer}</td>
                </tr>
                <tr>
                  <td className="cell-muted">Valid until</td>
                  <td>
                    {fmtTime(c.not_after)}{' '}
                    <Badge variant={c.expiring_soon ? 'danger' : 'success'}>
                      {c.expiring_soon ? `${c.days_remaining}d — expiring soon` : `${c.days_remaining}d remaining`}
                    </Badge>
                  </td>
                </tr>
                <tr>
                  <td className="cell-muted">SANs</td>
                  <td>{c.sans.length > 0 ? c.sans.join(', ') : '—'}</td>
                </tr>
                <tr>
                  <td className="cell-muted">Serial</td>
                  <td className={styles.mono}>{c.serial_number}</td>
                </tr>
              </tbody>
            </Table>

            {rotating === c.role && (
              <form className={styles.inlineForm} onSubmit={(e) => submitRotate(e, c.role)}>
                <div className="field">
                  <label htmlFor={`certPem-${c.role}`}>Certificate (PEM)</label>
                  <textarea
                    id={`certPem-${c.role}`}
                    className={styles.textarea}
                    rows={5}
                    value={certPem}
                    onChange={(e) => setCertPem(e.target.value)}
                    placeholder="-----BEGIN CERTIFICATE-----"
                  />
                </div>
                {c.role === 'server' && (
                  <div className="field">
                    <label htmlFor={`keyPem-${c.role}`}>Private key (PEM)</label>
                    <textarea
                      id={`keyPem-${c.role}`}
                      className={styles.textarea}
                      rows={5}
                      value={keyPem}
                      onChange={(e) => setKeyPem(e.target.value)}
                      placeholder="-----BEGIN PRIVATE KEY-----"
                    />
                  </div>
                )}
                {formError && <p className={styles.formError}>{formError}</p>}
                <div className={styles.formActions}>
                  <Button type="submit" disabled={saving}>
                    {saving ? 'Rotating…' : 'Rotate'}
                  </Button>
                </div>
              </form>
            )}
          </Section>
        ))
      )}
    </>
  )
}

export default CertificatesTab
