import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Check, Copy, Download, KeyRound } from 'lucide-react'
import AuthShell from '../AuthShell/AuthShell'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import PasswordField from '../../components/ui/PasswordField/PasswordField'
import Select from '../../components/ui/Select/Select'
import Callout from '../../components/ui/Callout/Callout'
import Checkbox from '../../components/ui/Checkbox/Checkbox'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import { ApiError, getSetupStatus, runSetup, type SetupRequest, type SetupResponse } from '../../lib/api'
import styles from './Setup.module.css'

type InstallType = 'hub' | 'remote' | 'combined'
type KmsProvider = 'local' | 'vault' | 'aws' | 'gcp' | 'azure'

const HUB_FLOW = [
  { id: 'welcome', label: 'Welcome' },
  { id: 'mesh', label: 'Mesh' },
  { id: 'account', label: 'Admin account' },
  { id: 'kms', label: 'Key management' },
  { id: 'recovery', label: 'Recovery keys' },
  { id: 'done', label: 'Done' },
] as const
const REMOTE_FLOW = [
  { id: 'welcome', label: 'Welcome' },
  { id: 'mesh', label: 'Mesh' },
  { id: 'done', label: 'Done' },
] as const

const INSTALL_TYPE_OPTIONS = [
  { value: 'hub', label: 'Hub — accepts inbound peers, must have a public IP/port' },
  { value: 'remote', label: 'Remote — connects outbound to a hub, no public IP needed' },
  { value: 'combined', label: 'Combined — acts as a hub and can connect upstream to another hub' },
] as const

const KMS_PROVIDER_OPTIONS = [
  { value: 'local', label: 'Local — key stored on disk, auto-generated' },
  { value: 'vault', label: 'HashiCorp Vault' },
  { value: 'aws', label: 'AWS KMS' },
  { value: 'gcp', label: 'GCP Cloud KMS' },
  { value: 'azure', label: 'Azure Key Vault' },
] as const

function randomMeshAddress(): string {
  const b = Math.floor(Math.random() * 101) + 100
  const c = Math.floor(Math.random() * 254) + 1
  return `10.${b}.${c}.1/24`
}

function networkFromCIDR(cidr: string): string {
  const m = cidr.match(/^(\d+\.\d+\.\d+)\.\d+\/(\d+)$/)
  return m ? `${m[1]}.0/${m[2]}` : ''
}

function Setup() {
  const navigate = useNavigate()
  const toast = useToast()
  const [step, setStep] = useState(0)

  // Already-configured instances shouldn't re-run the wizard.
  useEffect(() => {
    getSetupStatus()
      .then((res) => {
        if (res.complete) navigate('/login', { replace: true })
      })
      .catch(() => {
        // Backend unreachable — let the user proceed; submit will surface the real error.
      })
  }, [navigate])

  // ── Mesh ─────────────────────────────────────────────────────────────────
  const [installType, setInstallType] = useState<InstallType>('hub')
  const [address, setAddress] = useState(randomMeshAddress())
  const [listenPort, setListenPort] = useState('51820')
  const [mtu, setMtu] = useState('')
  const [hubEndpoint, setHubEndpoint] = useState('')
  const [hubPublicKey, setHubPublicKey] = useState('')
  const [hubAllowedIPs, setHubAllowedIPs] = useState('')
  const [hubHMACKey, setHubHMACKey] = useState('')
  const [hubAPIAddress, setHubAPIAddress] = useState('')
  const [meshError, setMeshError] = useState('')

  function onInstallTypeChange(type: InstallType) {
    setInstallType(type)
    if (type === 'remote') {
      setAddress('')
    } else if (!address) {
      setAddress(randomMeshAddress())
    }
  }

  function onAddressChange(value: string) {
    setAddress(value)
    const net = networkFromCIDR(value)
    if (net) setHubAllowedIPs(net)
  }

  const steps = installType === 'remote' ? REMOTE_FLOW : HUB_FLOW
  const currentStepId = steps[step]?.id ?? 'welcome'

  function nextMesh() {
    setMeshError('')
    if (!address) {
      setMeshError('Mesh IP address is required.')
      return
    }
    if (installType !== 'remote' && !listenPort) {
      setMeshError('Listen port is required for hub and combined installs.')
      return
    }
    if (installType === 'remote') {
      if (!hubEndpoint) return setMeshError('Hub endpoint is required.')
      if (!hubPublicKey) return setMeshError('Hub public key is required.')
      if (!hubAllowedIPs) return setMeshError('Hub allowed IPs are required.')
      void doFinish(setMeshError) // remote skips straight to "done"
      return
    }
    setStep((s) => s + 1)
  }

  // ── Admin account ────────────────────────────────────────────────────────
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [adminError, setAdminError] = useState('')

  function nextAdmin() {
    setAdminError('')
    if (!username || !password) return setAdminError('Username and password are required.')
    if (password.length < 12) return setAdminError('Password must be at least 12 characters.')
    if (password !== confirm) return setAdminError('Passwords do not match.')
    setStep((s) => s + 1)
  }

  // ── KMS ──────────────────────────────────────────────────────────────────
  const [kmsProvider, setKmsProvider] = useState<KmsProvider>('local')
  const [vaultAddr, setVaultAddr] = useState('')
  const [vaultKeyName, setVaultKeyName] = useState('')
  const [vaultTokenFile, setVaultTokenFile] = useState('')
  const [awsRegion, setAwsRegion] = useState('')
  const [awsKeyId, setAwsKeyId] = useState('')
  const [awsAccessKey, setAwsAccessKey] = useState('')
  const [awsSecretKey, setAwsSecretKey] = useState('')
  const [gcpProjectId, setGcpProjectId] = useState('')
  const [gcpLocationId, setGcpLocationId] = useState('')
  const [gcpKeyRingId, setGcpKeyRingId] = useState('')
  const [gcpKeyId, setGcpKeyId] = useState('')
  const [gcpTokenFile, setGcpTokenFile] = useState('')
  const [azureVaultUrl, setAzureVaultUrl] = useState('')
  const [azureKeyName, setAzureKeyName] = useState('')
  const [azureTenantId, setAzureTenantId] = useState('')
  const [azureClientId, setAzureClientId] = useState('')
  const [azureTokenFile, setAzureTokenFile] = useState('')
  const [kmsError, setKmsError] = useState('')

  function nextKMS() {
    setKmsError('')
    switch (kmsProvider) {
      case 'vault':
        if (!vaultAddr) return setKmsError('Vault address is required.')
        if (!vaultKeyName) return setKmsError('Transit key name is required.')
        break
      case 'aws':
        if (!awsRegion) return setKmsError('AWS region is required.')
        if (!awsKeyId) return setKmsError('KMS key ID is required.')
        break
      case 'gcp':
        if (!gcpProjectId) return setKmsError('Project ID is required.')
        if (!gcpKeyRingId) return setKmsError('Key ring ID is required.')
        if (!gcpKeyId) return setKmsError('Key ID is required.')
        break
      case 'azure':
        if (!azureVaultUrl) return setKmsError('Key Vault URL is required.')
        if (!azureTenantId) return setKmsError('Tenant ID is required.')
        if (!azureClientId) return setKmsError('Client ID is required.')
        break
    }
    setStep((s) => s + 1)
  }

  // ── Recovery ─────────────────────────────────────────────────────────────
  const [nShares, setNShares] = useState(5)
  const [threshold, setThreshold] = useState(3)

  function clampThreshold(n: number) {
    setNShares(n)
    setThreshold((t) => Math.min(Math.max(t, 2), n))
  }

  // ── Submission ───────────────────────────────────────────────────────────
  const [submitting, setSubmitting] = useState(false)
  const [restarting, setRestarting] = useState(false)
  const [finishError, setFinishError] = useState('')
  const [result, setResult] = useState<SetupResponse | null>(null)
  const [recoveryShares, setRecoveryShares] = useState<string[]>([])
  const [sharesAcknowledged, setSharesAcknowledged] = useState(false)
  const [copiedShare, setCopiedShare] = useState<number | null>(null)
  const [copiedPubkey, setCopiedPubkey] = useState(false)

  function buildKmsPayload(): SetupRequest['kms'] {
    switch (kmsProvider) {
      case 'vault':
        return { provider: 'vault', vault: { addr: vaultAddr, key_name: vaultKeyName, token_file: vaultTokenFile || undefined } }
      case 'aws':
        return {
          provider: 'aws',
          aws: { region: awsRegion, key_id: awsKeyId, access_key: awsAccessKey || undefined, secret_key: awsSecretKey || undefined },
        }
      case 'gcp':
        return {
          provider: 'gcp',
          gcp: {
            project_id: gcpProjectId,
            location_id: gcpLocationId || undefined,
            key_ring_id: gcpKeyRingId,
            key_id: gcpKeyId,
            token_file: gcpTokenFile || undefined,
          },
        }
      case 'azure':
        return {
          provider: 'azure',
          azure: {
            vault_url: azureVaultUrl,
            key_name: azureKeyName || undefined,
            tenant_id: azureTenantId,
            client_id: azureClientId,
            token_file: azureTokenFile || undefined,
          },
        }
      default:
        return { provider: 'local' }
    }
  }

  function buildPayload(): SetupRequest {
    return {
      install_type: installType,
      kms: buildKmsPayload(),
      wireguard: {
        address: address,
        listen_port: installType !== 'remote' && listenPort ? Number(listenPort) : undefined,
        mtu: mtu ? Number(mtu) : undefined,
        hub_endpoint: installType !== 'hub' && hubEndpoint ? hubEndpoint : undefined,
        hub_public_key: installType !== 'hub' && hubPublicKey ? hubPublicKey : undefined,
        hub_allowed_ips: installType !== 'hub' && hubAllowedIPs ? hubAllowedIPs : undefined,
        hub_hmac_key: installType !== 'hub' && hubHMACKey ? hubHMACKey : undefined,
        hub_api_address: installType !== 'hub' && hubAPIAddress ? hubAPIAddress : undefined,
      },
      admin: { username, password },
      recovery: kmsProvider === 'local' ? { n_shares: nShares, threshold } : undefined,
    }
  }

  function applySuccess(res: SetupResponse) {
    setResult(res)
    setRecoveryShares(res.recovery_shares ?? [])
    setStep(steps.length - 1)
  }

  async function pollUntilAlive() {
    // eslint-disable-next-line no-constant-condition
    while (true) {
      await new Promise((r) => setTimeout(r, 2000))
      try {
        await getSetupStatus()
        return
      } catch {
        // backend still restarting — keep polling
      }
    }
  }

  async function doFinish(setError: (msg: string) => void) {
    setError('')
    setSubmitting(true)
    try {
      const res = await runSetup(buildPayload())
      if (res.status === 'restarting') {
        // wireguard-go was just installed; the server is restarting itself.
        setRestarting(true)
        setSubmitting(false)
        await pollUntilAlive()
        setRestarting(false)
        setSubmitting(true)
        const retry = await runSetup(buildPayload())
        applySuccess(retry)
        return
      }
      applySuccess(res)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Setup failed. Please try again.')
    } finally {
      setSubmitting(false)
      setRestarting(false)
    }
  }

  function finishSetup(e: FormEvent) {
    e.preventDefault()
    void doFinish(setFinishError)
  }

  async function copyShare(share: string, index: number) {
    try {
      await navigator.clipboard.writeText(share)
      setCopiedShare(index)
      toast('Share copied to clipboard')
      setTimeout(() => setCopiedShare(null), 1800)
    } catch {
      // clipboard permission denied — the download button remains available
    }
  }

  function downloadShare(share: string, index: number, total: number) {
    const blob = new Blob([share], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `scutum-erk-share-${index + 1}-of-${total}.erk`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  async function copyPubkey(key: string) {
    try {
      await navigator.clipboard.writeText(key)
      setCopiedPubkey(true)
      toast('Public key copied to clipboard')
      setTimeout(() => setCopiedPubkey(false), 1800)
    } catch {
      // clipboard permission denied
    }
  }

  const isRemote = installType === 'remote'

  return (
    <AuthShell eyebrow="First Deployment" title="Stand up your instance." wide>
      <div className={styles.stepTrack}>
        {steps.map((s, i) => (
          <div key={s.id} className={styles.stepTrackItem}>
            <div
              className={
                i < step
                  ? `${styles.stepNode} ${styles.stepNodeDone}`
                  : i === step
                    ? `${styles.stepNode} ${styles.stepNodeActive}`
                    : styles.stepNode
              }
            >
              {i < step ? <Check size={12} /> : i + 1}
            </div>
            {i < steps.length - 1 && (
              <div className={i < step ? `${styles.stepLine} ${styles.stepLineDone}` : styles.stepLine} />
            )}
          </div>
        ))}
      </div>

      {currentStepId === 'welcome' && (
        <>
          <h2 className={styles.stepHeading}>Welcome to Scutum</h2>
          <p className={styles.stepDesc}>
            This wizard configures your Scutum control node. It runs once. Scutum forms a fully
            encrypted P2P mesh across your infrastructure — no SaaS control plane, no relays, no
            telemetry.
          </p>
          <Callout variant="info">
            Scutum uses <strong>manual peer enrollment only</strong> — nodes are never
            auto-discovered or auto-trusted.
          </Callout>
          <div className={styles.actions}>
            <Button onClick={() => setStep(1)}>Begin setup →</Button>
          </div>
        </>
      )}

      {currentStepId === 'mesh' && (
        <>
          <h2 className={styles.stepHeading}>Mesh configuration</h2>
          <p className={styles.stepDesc}>
            Configure WireGuard for this node. The public key will be shown after setup completes.
          </p>

          <Select
            label="Node role"
            id="installType"
            value={installType}
            onChange={(e) => onInstallTypeChange(e.target.value as InstallType)}
            options={INSTALL_TYPE_OPTIONS}
          />

          <TextField
            label="Mesh IP address (CIDR)"
            id="address"
            value={address}
            onChange={(e) => onAddressChange(e.target.value)}
            placeholder={isRemote ? "e.g. 10.100.5.2/24 — must be on the hub's subnet" : '10.100.5.1/24'}
          />

          {!isRemote && (
            <TextField
              label="WireGuard listen port (UDP)"
              id="listenPort"
              inputMode="numeric"
              value={listenPort}
              onChange={(e) => setListenPort(e.target.value.replace(/\D/g, ''))}
              placeholder="51820"
            />
          )}

          {installType !== 'hub' && (
            <>
              {installType === 'combined' && (
                <Callout variant="info">
                  For combined mode, upstream hub connection is <strong>optional</strong>. Leave
                  blank to run as a standalone hub.
                </Callout>
              )}
              <TextField
                label={`Hub endpoint (host:port${installType === 'combined' ? ' — optional' : ''})`}
                id="hubEndpoint"
                value={hubEndpoint}
                onChange={(e) => setHubEndpoint(e.target.value)}
                placeholder="1.2.3.4:51820"
              />
              <TextField
                label={`Hub public key${installType === 'combined' ? ' (optional)' : ''}`}
                id="hubPublicKey"
                value={hubPublicKey}
                onChange={(e) => setHubPublicKey(e.target.value)}
                placeholder="Base64-encoded WireGuard public key"
              />
              <TextField
                label={`Hub allowed IPs${installType === 'combined' ? ' (optional)' : ''}`}
                id="hubAllowedIPs"
                value={hubAllowedIPs}
                onChange={(e) => setHubAllowedIPs(e.target.value)}
                placeholder="e.g. 10.100.5.0/24"
              />
              <TextField
                label="Hub API address (optional)"
                id="hubAPIAddress"
                value={hubAPIAddress}
                onChange={(e) => setHubAPIAddress(e.target.value)}
                placeholder="auto-derived from Hub allowed IPs"
              />
              <TextField
                label="Hub proxy key (from hub's Enroll Peer dialog)"
                id="hubHMACKey"
                value={hubHMACKey}
                onChange={(e) => setHubHMACKey(e.target.value)}
                placeholder="hex key shown in the hub's enrollment dialog"
              />
            </>
          )}

          <TextField
            label="MTU (optional, leave blank for default 1420)"
            id="mtu"
            inputMode="numeric"
            value={mtu}
            onChange={(e) => setMtu(e.target.value.replace(/\D/g, ''))}
            placeholder="0"
          />

          {restarting && (
            <Callout variant="info">WireGuard was just installed and the server is restarting — retrying automatically once it's back.</Callout>
          )}
          {meshError && <Callout variant="danger">{meshError}</Callout>}

          <div className={styles.actions}>
            <Button variant="ghost" onClick={() => setStep((s) => s - 1)} disabled={submitting}>
              Back
            </Button>
            <Button onClick={nextMesh} disabled={isRemote && submitting}>
              {isRemote ? (submitting ? 'Connecting…' : 'Connect to hub →') : 'Continue →'}
            </Button>
          </div>
        </>
      )}

      {currentStepId === 'account' && (
        <>
          <h2 className={styles.stepHeading}>Create admin account</h2>
          <p className={styles.stepDesc}>
            This will be the primary superadmin. Additional accounts can be created after setup.
          </p>
          <TextField
            label="Username"
            id="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="admin"
            autoComplete="username"
          />
          <PasswordField
            label="Password (min 12 characters)"
            id="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
          />
          <TextField
            label="Confirm password"
            id="confirm"
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            autoComplete="new-password"
          />
          {adminError && <Callout variant="danger">{adminError}</Callout>}
          <div className={styles.actions}>
            <Button variant="ghost" onClick={() => setStep((s) => s - 1)}>
              Back
            </Button>
            <Button onClick={nextAdmin}>Continue →</Button>
          </div>
        </>
      )}

      {currentStepId === 'kms' && (
        <>
          <h2 className={styles.stepHeading}>Key management</h2>
          <p className={styles.stepDesc}>
            Scutum encrypts secrets at rest using a master key. Choose where to store that key.
            For most self-hosted deployments <strong>Local</strong> is the right choice.
          </p>

          <Select
            label="KMS provider"
            id="kmsProvider"
            value={kmsProvider}
            onChange={(e) => setKmsProvider(e.target.value as KmsProvider)}
            options={KMS_PROVIDER_OPTIONS}
          />

          {kmsProvider === 'local' && (
            <Callout variant="info">
              A 256-bit master key is generated automatically and stored on disk. The key is split
              into recovery shares in the next step.
            </Callout>
          )}

          {kmsProvider === 'vault' && (
            <>
              <TextField label="Vault address" id="vaultAddr" value={vaultAddr} onChange={(e) => setVaultAddr(e.target.value)} placeholder="https://vault.example.com:8200" />
              <TextField label="Transit key name" id="vaultKeyName" value={vaultKeyName} onChange={(e) => setVaultKeyName(e.target.value)} placeholder="scutum" />
              <TextField label="Token file (optional — path on server)" id="vaultTokenFile" value={vaultTokenFile} onChange={(e) => setVaultTokenFile(e.target.value)} placeholder="/run/secrets/vault-token" />
            </>
          )}

          {kmsProvider === 'aws' && (
            <>
              <TextField label="AWS region" id="awsRegion" value={awsRegion} onChange={(e) => setAwsRegion(e.target.value)} placeholder="us-east-1" />
              <TextField label="KMS key ID or ARN" id="awsKeyId" value={awsKeyId} onChange={(e) => setAwsKeyId(e.target.value)} placeholder="arn:aws:kms:us-east-1:123456:key/..." />
              <TextField label="Access key ID (optional — omit to use instance role)" id="awsAccessKey" value={awsAccessKey} onChange={(e) => setAwsAccessKey(e.target.value)} placeholder="AKIAIOSFODNN7EXAMPLE" />
              <TextField label="Secret access key (optional)" id="awsSecretKey" type="password" value={awsSecretKey} onChange={(e) => setAwsSecretKey(e.target.value)} />
            </>
          )}

          {kmsProvider === 'gcp' && (
            <>
              <TextField label="GCP project ID" id="gcpProjectId" value={gcpProjectId} onChange={(e) => setGcpProjectId(e.target.value)} placeholder="my-project-id" />
              <TextField label="Location ID (optional, defaults to global)" id="gcpLocationId" value={gcpLocationId} onChange={(e) => setGcpLocationId(e.target.value)} placeholder="global" />
              <TextField label="Key ring ID" id="gcpKeyRingId" value={gcpKeyRingId} onChange={(e) => setGcpKeyRingId(e.target.value)} placeholder="scutum-ring" />
              <TextField label="Key ID" id="gcpKeyId" value={gcpKeyId} onChange={(e) => setGcpKeyId(e.target.value)} placeholder="master-key" />
              <TextField label="Service account token file (optional)" id="gcpTokenFile" value={gcpTokenFile} onChange={(e) => setGcpTokenFile(e.target.value)} placeholder="/run/secrets/gcp-sa.json" />
            </>
          )}

          {kmsProvider === 'azure' && (
            <>
              <TextField label="Key Vault URL" id="azureVaultUrl" value={azureVaultUrl} onChange={(e) => setAzureVaultUrl(e.target.value)} placeholder="https://myvault.vault.azure.net" />
              <TextField label="Key name (optional, defaults to scutum)" id="azureKeyName" value={azureKeyName} onChange={(e) => setAzureKeyName(e.target.value)} placeholder="scutum" />
              <TextField label="Tenant ID" id="azureTenantId" value={azureTenantId} onChange={(e) => setAzureTenantId(e.target.value)} placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" />
              <TextField label="Client ID" id="azureClientId" value={azureClientId} onChange={(e) => setAzureClientId(e.target.value)} placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" />
              <TextField label="Token file (optional)" id="azureTokenFile" value={azureTokenFile} onChange={(e) => setAzureTokenFile(e.target.value)} placeholder="/run/secrets/azure-token" />
            </>
          )}

          {kmsError && <Callout variant="danger">{kmsError}</Callout>}
          <div className={styles.actions}>
            <Button variant="ghost" onClick={() => setStep((s) => s - 1)}>
              Back
            </Button>
            <Button onClick={nextKMS}>Continue →</Button>
          </div>
        </>
      )}

      {currentStepId === 'recovery' && (
        <form onSubmit={finishSetup}>
          <h2 className={styles.stepHeading}>Emergency recovery keys</h2>
          <p className={styles.stepDesc}>
            Scutum uses Shamir's Secret Sharing to split your encryption master key into recovery
            shares. If you ever lose access to the server, you can reconstruct the key using the
            required number of shares.
          </p>

          <Callout variant="warning">
            Shares are shown <strong>once</strong> after setup and cannot be retrieved again.
            Distribute them to trusted people and store offline.
          </Callout>

          {kmsProvider !== 'local' ? (
            <Callout variant="info">
              Recovery shares only apply to the <strong>local</strong> KMS provider. Since you
              chose <strong>{kmsProvider}</strong>, the master key is managed externally — recovery
              shares will not be generated.
            </Callout>
          ) : (
            <>
              <div className="field">
                <label htmlFor="nShares">Total shares ({nShares})</label>
                <input
                  id="nShares"
                  type="range"
                  min={3}
                  max={10}
                  value={nShares}
                  onChange={(e) => clampThreshold(Number(e.target.value))}
                  className={styles.rangeInput}
                />
                <div className={styles.rangeLabels}>
                  <span>3</span>
                  <span>10</span>
                </div>
              </div>
              <div className="field">
                <label htmlFor="threshold">
                  Shares required to recover ({threshold} of {nShares})
                </label>
                <input
                  id="threshold"
                  type="range"
                  min={2}
                  max={nShares}
                  value={threshold}
                  onChange={(e) => setThreshold(Number(e.target.value))}
                  className={styles.rangeInput}
                />
                <div className={styles.rangeLabels}>
                  <span>2</span>
                  <span>{nShares}</span>
                </div>
              </div>
              <div className={styles.recoveryPreview}>
                <KeyRound size={14} />
                <span>
                  <strong>
                    {threshold} of {nShares}
                  </strong>{' '}
                  shares needed to recover access. You can lose up to{' '}
                  <strong>{nShares - threshold}</strong> share{nShares - threshold !== 1 ? 's' : ''}{' '}
                  and still recover.
                </span>
              </div>
            </>
          )}

          {restarting && (
            <Callout variant="info">WireGuard was just installed and the server is restarting — retrying automatically once it's back.</Callout>
          )}
          {finishError && <Callout variant="danger">{finishError}</Callout>}

          <div className={styles.actions}>
            <Button type="button" variant="ghost" onClick={() => setStep((s) => s - 1)} disabled={submitting}>
              Back
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Setting up…' : 'Finish setup →'}
            </Button>
          </div>
        </form>
      )}

      {currentStepId === 'done' && (
        <>
          <div className={styles.doneIcon}>
            <Check size={32} />
          </div>
          <h2 className={styles.stepHeading}>Setup complete</h2>

          {result?.wireguard?.warning && (
            <Callout variant="warning">
              <strong>WireGuard interface not active.</strong> The admin account and keys were
              saved, but the mesh interface could not be started on this host. To activate it,
              load the kernel module (<code>modprobe wireguard</code>) or install{' '}
              <code>wireguard-go</code>, then restart Scutum.
            </Callout>
          )}

          {recoveryShares.length > 0 && (
            <>
              <Callout variant="danger">
                <strong>Save these recovery shares now.</strong> They will not be shown again.
                Store each share separately in a secure offline location.
              </Callout>
              <div className={styles.sharesGrid}>
                {recoveryShares.map((share, i) => (
                  <div key={i} className={styles.shareCard}>
                    <div className={styles.shareCardHeader}>
                      <span className={styles.shareCardLabel}>
                        Share {i + 1} of {recoveryShares.length}
                      </span>
                      <div className={styles.shareCardActions}>
                        <button
                          type="button"
                          className={styles.copyBtn}
                          onClick={() => copyShare(share, i)}
                          aria-label="Copy share to clipboard"
                        >
                          {copiedShare === i ? <Check size={12} /> : <Copy size={12} />}
                        </button>
                        <button
                          type="button"
                          className={styles.copyBtn}
                          onClick={() => downloadShare(share, i, recoveryShares.length)}
                          aria-label="Download share as file"
                        >
                          <Download size={12} />
                        </button>
                      </div>
                    </div>
                    <div className={styles.shareCardValue}>{share}</div>
                  </div>
                ))}
              </div>
              <Checkbox checked={sharesAcknowledged} onChange={(e) => setSharesAcknowledged(e.target.checked)}>
                I have saved all {recoveryShares.length} recovery shares in a secure location.
              </Checkbox>
            </>
          )}

          <div className={styles.summaryList}>
            {username && (
              <div className={styles.summaryRow}>
                <span className={styles.summaryLabel}>Admin account</span>
                <span className={styles.summaryVal}>{username}</span>
              </div>
            )}
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Install type</span>
              <span className={styles.summaryVal}>{result?.install_type ?? installType}</span>
            </div>
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>KMS provider</span>
              <span className={styles.summaryVal}>{result?.kms_provider ?? kmsProvider}</span>
            </div>
            {result?.wireguard?.public_key && (
              <div className={styles.summaryRow}>
                <span className={styles.summaryLabel}>WireGuard public key</span>
                <span className={`${styles.summaryVal} ${styles.summaryValKey}`}>{result.wireguard.public_key}</span>
                <button
                  type="button"
                  className={styles.copyBtn}
                  onClick={() => copyPubkey(result.wireguard!.public_key)}
                  aria-label="Copy public key"
                >
                  {copiedPubkey ? <Check size={12} /> : <Copy size={12} />}
                </button>
              </div>
            )}
            <div className={styles.summaryRow}>
              <span className={styles.summaryLabel}>Mesh IP</span>
              <span className={styles.summaryVal}>{result?.wireguard?.address ?? address}</span>
            </div>
            {(result?.wireguard?.listen_port ?? listenPort) && (
              <div className={styles.summaryRow}>
                <span className={styles.summaryLabel}>Listen port</span>
                <span className={styles.summaryVal}>{result?.wireguard?.listen_port ?? listenPort}</span>
              </div>
            )}
          </div>

          {isRemote ? (
            <Callout variant="info">
              This node is running as a <strong>remote peer</strong>. There is no local account to
              sign in to. Visit your <strong>hub's UI</strong> and enroll this node using the
              WireGuard public key above to complete the mesh connection.
            </Callout>
          ) : (
            <div className={styles.actions}>
              <Button
                onClick={() => navigate('/login')}
                disabled={recoveryShares.length > 0 && !sharesAcknowledged}
              >
                Sign in →
              </Button>
            </div>
          )}
        </>
      )}

      <p className={styles.stepLabel}>{steps[step]?.label}</p>
    </AuthShell>
  )
}

export default Setup
