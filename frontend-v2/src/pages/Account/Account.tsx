import { useState, type FormEvent } from 'react'
import { Copy, KeyRound, Plus, ShieldCheck, Trash2 } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Badge from '../../components/ui/Badge/Badge'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import PasswordField from '../../components/ui/PasswordField/PasswordField'
import Select from '../../components/ui/Select/Select'
import Callout from '../../components/ui/Callout/Callout'
import Section from '../../components/ui/Section/Section'
import Table from '../../components/ui/Table/Table'
import EmptyState from '../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../components/ui/Toast/ToastProvider'
import styles from './Account.module.css'

// Real endpoints (verified against useApi.ts + Go backend): GET /auth/me,
// PUT /users/{id} (also used for password change — there's no dedicated
// change-password endpoint), GET/POST /auth/mfa/{setup,enable,disable},
// GET /auth/mfa/status, GET recovery-code status + POST regenerate, GET
// /auth/tokens + POST/DELETE. There's deliberately no "Active Sessions"
// card here — the old Nuxt frontend's version of that is a 100% fake
// hardcoded single session with no backend behind it at all, and
// replicating a placeholder of a placeholder isn't worth it. Not wired to
// a live backend yet.
type ApiToken = { id: string; name: string; createdAt: string; expiresAt: string | null }

const INITIAL_TOKENS: ApiToken[] = [{ id: 'tok_1', name: 'ci-deploy-key', createdAt: '2026-07-09 08:45:10', expiresAt: null }]

const MFA_SECRET = 'JBSWY3DPEHPK3PXP'
const MFA_URI = `otpauth://totp/Scutum:admin?secret=${MFA_SECRET}&issuer=Scutum`

function randomCode() {
  return Array.from({ length: 4 }, () => Math.random().toString(16).slice(2, 6).toUpperCase()).join('-')
}

function randomToken() {
  return `sk_live_${Array.from({ length: 24 }, () => Math.floor(Math.random() * 16).toString(16)).join('')}`
}

function Account() {
  const toast = useToast()

  // ── Profile ──────────────────────────────────────────────────────
  const [username, setUsername] = useState('admin')

  function saveProfile(e: FormEvent) {
    e.preventDefault()
    toast('Profile updated')
  }

  // ── Password ─────────────────────────────────────────────────────
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordError, setPasswordError] = useState('')

  function savePassword(e: FormEvent) {
    e.preventDefault()
    if (!newPassword || newPassword.length < 8) {
      setPasswordError('Password must be at least 8 characters.')
      return
    }
    if (newPassword !== confirmPassword) {
      setPasswordError("Passwords don't match.")
      return
    }
    setPasswordError('')
    setNewPassword('')
    setConfirmPassword('')
    toast('Password updated')
  }

  // ── MFA ──────────────────────────────────────────────────────────
  const [mfaEnabled, setMfaEnabled] = useState(true)
  const [mfaSetupOpen, setMfaSetupOpen] = useState(false)
  const [mfaCode, setMfaCode] = useState('')
  const [mfaDisableOpen, setMfaDisableOpen] = useState(false)
  const [mfaDisableCode, setMfaDisableCode] = useState('')

  function confirmEnableMfa(e: FormEvent) {
    e.preventDefault()
    if (mfaCode.trim().length !== 6) return
    setMfaEnabled(true)
    setMfaSetupOpen(false)
    setMfaCode('')
    toast('Two-factor authentication enabled')
  }

  function confirmDisableMfa(e: FormEvent) {
    e.preventDefault()
    if (mfaDisableCode.trim().length !== 6) return
    setMfaEnabled(false)
    setMfaDisableOpen(false)
    setMfaDisableCode('')
    toast('Two-factor authentication disabled', 'danger')
  }

  // ── Recovery codes ───────────────────────────────────────────────
  const [lastRegenerated, setLastRegenerated] = useState('2026-06-15 16:40:02')
  const [revealedCodes, setRevealedCodes] = useState<string[] | null>(null)

  function regenerateCodes() {
    setRevealedCodes(Array.from({ length: 10 }, randomCode))
    setLastRegenerated('just now')
    toast('Recovery codes regenerated')
  }

  // ── API tokens ────────────────────────────────────────────────────
  const [tokens, setTokens] = useState(INITIAL_TOKENS)
  const [showCreateToken, setShowCreateToken] = useState(false)
  const [tokenName, setTokenName] = useState('')
  const [tokenExpiry, setTokenExpiry] = useState('never')
  const [revealedToken, setRevealedToken] = useState<string | null>(null)

  function createToken(e: FormEvent) {
    e.preventDefault()
    if (!tokenName) return
    const expiresAt = tokenExpiry === 'never' ? null : `in ${tokenExpiry.replace('d', ' days').replace('y', ' year')}`
    setTokens((prev) => [...prev, { id: `tok_${prev.length + 1}`, name: tokenName, createdAt: 'just now', expiresAt }])
    setRevealedToken(randomToken())
    setTokenName('')
    setTokenExpiry('never')
    setShowCreateToken(false)
  }

  function revokeToken(t: ApiToken) {
    setTokens((prev) => prev.filter((x) => x.id !== t.id))
    toast(`${t.name} revoked`, 'danger')
  }

  return (
    <AppShell title="Account">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Your Account</p>
            <h2 className={styles.reportTitle}>Profile &amp; security</h2>
          </div>
          <span className={mfaEnabled ? 'stamp' : 'stamp stamp--alt'}>{mfaEnabled ? 'MFA enabled' : 'MFA disabled'}</span>
        </div>

        <Section title="Profile">
          <form className={styles.profileForm} onSubmit={saveProfile}>
            <TextField label="Username" id="username" value={username} onChange={(e) => setUsername(e.target.value)} />
            <div className={styles.profileMeta}>
              <div>
                <span className={styles.metaLabel}>Roles</span>
                <div className={styles.roleBadges}>
                  <Badge variant="success">admin</Badge>
                </div>
              </div>
              <div>
                <span className={styles.metaLabel}>Member since</span>
                <span className={styles.metaValue}>2026-04-02</span>
              </div>
            </div>
            <div className={styles.formActions}>
              <Button type="submit">Save profile</Button>
            </div>
          </form>
        </Section>

        <Section title="Change password">
          <form className={styles.profileForm} onSubmit={savePassword}>
            <div className={styles.passwordFields}>
              <PasswordField label="New password" id="newPassword" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
              <PasswordField label="Confirm password" id="confirmPassword" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} />
            </div>
            {passwordError && <p className={styles.formError}>{passwordError}</p>}
            <div className={styles.formActions}>
              <Button type="submit">Update password</Button>
            </div>
          </form>
        </Section>

        <Section title="Two-factor authentication">
          <div className={styles.mfaRow}>
            <div className={styles.mfaStatus}>
              <ShieldCheck size={18} className={mfaEnabled ? styles.mfaIconOn : styles.mfaIconOff} />
              <span>{mfaEnabled ? 'Two-factor authentication is enabled.' : 'Two-factor authentication is disabled.'}</span>
            </div>
            {mfaEnabled ? (
              <Button variant="ghost" onClick={() => setMfaDisableOpen((v) => !v)}>
                Disable
              </Button>
            ) : (
              <Button variant="ghost" onClick={() => setMfaSetupOpen((v) => !v)}>
                Enable
              </Button>
            )}
          </div>

          {mfaSetupOpen && !mfaEnabled && (
            <form className={styles.mfaSetup} onSubmit={confirmEnableMfa}>
              <p className={styles.mfaHint}>
                Scan this URI with your authenticator app, or enter the secret manually — there's no rendered QR code in this
                preview build.
              </p>
              <div className={styles.mfaSecretBlock}>
                <div>
                  <span className={styles.metaLabel}>Secret</span>
                  <span className={styles.mono}>{MFA_SECRET}</span>
                </div>
                <div>
                  <span className={styles.metaLabel}>otpauth URI</span>
                  <span className={styles.mono}>{MFA_URI}</span>
                </div>
              </div>
              <div className={styles.mfaCodeRow}>
                <TextField label="6-digit code" id="mfaCode" value={mfaCode} onChange={(e) => setMfaCode(e.target.value)} placeholder="000000" maxLength={6} />
                <Button type="submit" disabled={mfaCode.trim().length !== 6}>
                  Confirm
                </Button>
              </div>
            </form>
          )}

          {mfaDisableOpen && mfaEnabled && (
            <form className={styles.mfaSetup} onSubmit={confirmDisableMfa}>
              <div className={styles.mfaCodeRow}>
                <TextField
                  label="Enter a 6-digit code to confirm"
                  id="mfaDisableCode"
                  value={mfaDisableCode}
                  onChange={(e) => setMfaDisableCode(e.target.value)}
                  placeholder="000000"
                  maxLength={6}
                />
                <Button type="submit" disabled={mfaDisableCode.trim().length !== 6}>
                  Disable MFA
                </Button>
              </div>
            </form>
          )}
        </Section>

        <Section title="Recovery codes" action={<Button variant="ghost" onClick={regenerateCodes}>Regenerate</Button>}>
          <p className={styles.recoveryStatus}>10 codes generated · last regenerated {lastRegenerated}</p>
          {revealedCodes && (
            <div className={styles.calloutWrap}>
              <Callout variant="warning">
                These codes are shown once and won't be shown again — store them somewhere safe. (This is a preview build: these
                codes are generated locally for display and aren't real recovery material.)
              </Callout>
              <div className={styles.codeGrid}>
                {revealedCodes.map((c) => (
                  <span className={styles.codeChip} key={c}>
                    {c}
                  </span>
                ))}
              </div>
            </div>
          )}
        </Section>

        <Section
          title="API tokens"
          action={
            <Button variant="ghost" onClick={() => setShowCreateToken((v) => !v)}>
              <Plus size={14} />
              Create token
            </Button>
          }
        >
          {showCreateToken && (
            <form className={styles.tokenForm} onSubmit={createToken}>
              <div className={styles.tokenFields}>
                <TextField label="Name" id="tokenName" value={tokenName} onChange={(e) => setTokenName(e.target.value)} placeholder="ci-deploy-key" />
                <Select
                  label="Expiry"
                  id="tokenExpiry"
                  value={tokenExpiry}
                  onChange={(e) => setTokenExpiry(e.target.value)}
                  options={[
                    { value: 'never', label: 'Never' },
                    { value: '30d', label: '30 days' },
                    { value: '90d', label: '90 days' },
                    { value: '1y', label: '1 year' },
                  ]}
                />
              </div>
              <div className={styles.formActions}>
                <Button type="submit" disabled={!tokenName}>
                  Create
                </Button>
              </div>
            </form>
          )}

          {revealedToken && (
            <div className={styles.calloutWrap}>
              <Callout variant="warning">
                Copy this token now — it won't be shown again.
              </Callout>
              <div className={styles.tokenReveal}>
                <span className={styles.mono}>{revealedToken}</span>
                <button
                  type="button"
                  className={styles.copyBtn}
                  onClick={() => {
                    navigator.clipboard?.writeText(revealedToken)
                    toast('Copied to clipboard')
                    setRevealedToken(null)
                  }}
                  aria-label="Copy token"
                >
                  <Copy size={14} />
                </button>
              </div>
            </div>
          )}

          {tokens.length === 0 ? (
            <EmptyState icon={KeyRound} title="No API tokens" description="Create a token to authenticate scripts or CI pipelines against the API." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Created</th>
                  <th>Expires</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {tokens.map((t) => (
                  <tr key={t.id}>
                    <td className="cell-name">
                      <KeyRound size={13} className={styles.tokenIcon} />
                      {t.name}
                    </td>
                    <td className="cell-muted">{t.createdAt}</td>
                    <td className="cell-muted">{t.expiresAt ?? 'Never'}</td>
                    <td>
                      <button type="button" className={styles.revokeBtn} onClick={() => revokeToken(t)} aria-label={`Revoke ${t.name}`}>
                        <Trash2 size={14} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </Section>
      </div>
    </AppShell>
  )
}

export default Account
