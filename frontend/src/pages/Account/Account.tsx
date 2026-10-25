import { useEffect, useState, type FormEvent } from 'react'
import { useLocation } from 'react-router-dom'
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
import {
  ApiError,
  createToken,
  deleteToken,
  disableMfa,
  enableMfa,
  getMe,
  getMfaStatus,
  getRecoveryCodeStatus,
  listTokens,
  regenerateRecoveryCodes,
  setupMfa,
  updateUser,
  type APIKeyRecord,
  type UserProfile,
} from '../../lib/api'
import styles from './Account.module.css'

// Real endpoints (verified against useApi.ts + Go backend): GET /auth/me,
// PUT /users/{id} (also used for password change — there's no dedicated
// change-password endpoint), GET/POST /auth/mfa/{setup,enable,disable},
// GET /auth/mfa/status, GET recovery-code status + POST regenerate, GET
// /auth/tokens + POST /auth/keys + DELETE /auth/tokens/:id. There's
// deliberately no "Active Sessions" card here — the old Nuxt frontend's
// version of that is a 100% fake hardcoded single session with no backend
// behind it at all, and replicating a placeholder of a placeholder isn't
// worth it.

function fmtDate(iso: string | null): string {
  if (!iso) return 'Never'
  try {
    return new Date(iso).toLocaleDateString()
  } catch {
    return iso
  }
}

const EXPIRY_DAYS: Record<string, number | null> = { never: null, '30d': 30, '90d': 90, '1y': 365 }

function Account() {
  const toast = useToast()
  const location = useLocation()
  // Set by Login when Settings → Auth's "require MFA for every user" is on
  // and this account hasn't enabled it yet — opens straight into MFA setup
  // instead of leaving the requirement easy to miss.
  const forceMfaSetup = (location.state as { forceMfaSetup?: boolean } | null)?.forceMfaSetup === true

  // ── Profile ──────────────────────────────────────────────────────
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [profileLoading, setProfileLoading] = useState(true)
  const [profileError, setProfileError] = useState('')
  const [username, setUsername] = useState('')
  const [savingProfile, setSavingProfile] = useState(false)

  async function loadProfile() {
    setProfileLoading(true)
    setProfileError('')
    try {
      const me = await getMe()
      setProfile(me)
      setUsername(me.username)
    } catch (e) {
      setProfileError(e instanceof ApiError ? e.message : 'Failed to load profile')
    } finally {
      setProfileLoading(false)
    }
  }

  useEffect(() => {
    loadProfile()
  }, [])

  async function saveProfile(e: FormEvent) {
    e.preventDefault()
    if (!profile) return
    setSavingProfile(true)
    try {
      await updateUser(profile.id, { username })
      setProfile((p) => (p ? { ...p, username } : p))
      toast('Profile updated')
    } catch (err) {
      toast(err instanceof ApiError ? err.message : 'Save failed', 'danger')
    } finally {
      setSavingProfile(false)
    }
  }

  // ── Password ─────────────────────────────────────────────────────
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordError, setPasswordError] = useState('')
  const [savingPassword, setSavingPassword] = useState(false)

  async function savePassword(e: FormEvent) {
    e.preventDefault()
    if (!profile) return
    if (!newPassword || newPassword.length < 12) {
      setPasswordError('Password must be at least 12 characters.')
      return
    }
    if (newPassword !== confirmPassword) {
      setPasswordError("Passwords don't match.")
      return
    }
    setSavingPassword(true)
    setPasswordError('')
    try {
      await updateUser(profile.id, { password: newPassword })
      setNewPassword('')
      setConfirmPassword('')
      toast('Password updated')
    } catch (err) {
      setPasswordError(err instanceof ApiError ? err.message : 'Update failed')
    } finally {
      setSavingPassword(false)
    }
  }

  // ── MFA ──────────────────────────────────────────────────────────
  const [mfaEnabled, setMfaEnabled] = useState(false)
  const [mfaLoading, setMfaLoading] = useState(true)
  const [mfaSetupOpen, setMfaSetupOpen] = useState(false)
  const [mfaSecret, setMfaSecret] = useState('')
  const [mfaUri, setMfaUri] = useState('')
  const [mfaQr, setMfaQr] = useState('')
  const [mfaCode, setMfaCode] = useState('')
  const [mfaError, setMfaError] = useState('')
  const [mfaBusy, setMfaBusy] = useState(false)
  const [mfaDisableOpen, setMfaDisableOpen] = useState(false)
  const [mfaDisableCode, setMfaDisableCode] = useState('')

  useEffect(() => {
    getMfaStatus()
      .then((s) => setMfaEnabled(s.enabled))
      .catch(() => {})
      .finally(() => setMfaLoading(false))
  }, [])

  useEffect(() => {
    if (forceMfaSetup && !mfaLoading && !mfaEnabled && !mfaSetupOpen) {
      beginMfaSetup()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [forceMfaSetup, mfaLoading, mfaEnabled])

  async function beginMfaSetup() {
    setMfaBusy(true)
    setMfaError('')
    try {
      const res = await setupMfa()
      setMfaSecret(res.secret)
      setMfaUri(res.uri)
      setMfaQr(res.qr_code)
      setMfaCode('')
      setMfaSetupOpen(true)
    } catch (err) {
      toast(err instanceof ApiError ? err.message : 'Failed to start MFA setup', 'danger')
    } finally {
      setMfaBusy(false)
    }
  }

  function cancelMfaSetup() {
    setMfaSetupOpen(false)
    setMfaCode('')
    setMfaSecret('')
    setMfaUri('')
    setMfaQr('')
    setMfaError('')
  }

  async function confirmEnableMfa(e: FormEvent) {
    e.preventDefault()
    if (mfaCode.trim().length !== 6) return
    setMfaBusy(true)
    setMfaError('')
    try {
      await enableMfa(mfaCode.trim())
      setMfaEnabled(true)
      cancelMfaSetup()
      toast('Two-factor authentication enabled')
    } catch (err) {
      setMfaError(err instanceof ApiError ? err.message : 'Invalid code — try again')
    } finally {
      setMfaBusy(false)
    }
  }

  async function confirmDisableMfa(e: FormEvent) {
    e.preventDefault()
    if (mfaDisableCode.trim().length !== 6) return
    setMfaBusy(true)
    setMfaError('')
    try {
      await disableMfa(mfaDisableCode.trim())
      setMfaEnabled(false)
      setMfaDisableOpen(false)
      setMfaDisableCode('')
      toast('Two-factor authentication disabled', 'danger')
    } catch (err) {
      setMfaError(err instanceof ApiError ? err.message : 'Invalid code — try again')
    } finally {
      setMfaBusy(false)
    }
  }

  // ── Recovery codes ───────────────────────────────────────────────
  const [rcRemaining, setRcRemaining] = useState<number | null>(null)
  const [rcLoading, setRcLoading] = useState(false)
  const [revealedCodes, setRevealedCodes] = useState<string[] | null>(null)

  useEffect(() => {
    getRecoveryCodeStatus()
      .then((s) => setRcRemaining(s.remaining))
      .catch(() => {})
  }, [])

  async function regenerateCodes() {
    setRcLoading(true)
    try {
      const res = await regenerateRecoveryCodes()
      setRevealedCodes(res.recovery_codes)
      setRcRemaining(res.recovery_codes.length)
      toast('Recovery codes regenerated')
    } catch (err) {
      toast(err instanceof ApiError ? err.message : 'Failed to regenerate codes', 'danger')
    } finally {
      setRcLoading(false)
    }
  }

  // ── API tokens ────────────────────────────────────────────────────
  const [tokens, setTokens] = useState<APIKeyRecord[]>([])
  const [tokensLoading, setTokensLoading] = useState(true)
  const [tokensError, setTokensError] = useState('')
  const [showCreateToken, setShowCreateToken] = useState(false)
  const [tokenName, setTokenName] = useState('')
  const [tokenExpiry, setTokenExpiry] = useState('never')
  const [revealedToken, setRevealedToken] = useState<string | null>(null)
  const [creatingToken, setCreatingToken] = useState(false)

  async function loadTokens() {
    setTokensLoading(true)
    setTokensError('')
    try {
      setTokens(await listTokens())
    } catch (e) {
      setTokensError(e instanceof ApiError ? e.message : 'Failed to load tokens')
    } finally {
      setTokensLoading(false)
    }
  }

  useEffect(() => {
    loadTokens()
  }, [])

  async function submitCreateToken(e: FormEvent) {
    e.preventDefault()
    if (!tokenName) return
    setCreatingToken(true)
    try {
      const days = EXPIRY_DAYS[tokenExpiry]
      const expiresAt = days ? new Date(Date.now() + days * 86400_000).toISOString() : undefined
      const res = await createToken(tokenName, expiresAt)
      setRevealedToken(res.key)
      setTokenName('')
      setTokenExpiry('never')
      setShowCreateToken(false)
      await loadTokens()
    } catch (err) {
      toast(err instanceof ApiError ? err.message : 'Failed to create token', 'danger')
    } finally {
      setCreatingToken(false)
    }
  }

  async function revokeToken(t: APIKeyRecord) {
    try {
      await deleteToken(t.id)
      await loadTokens()
      toast(`${t.name} revoked`, 'danger')
    } catch (err) {
      toast(err instanceof ApiError ? err.message : 'Revoke failed', 'danger')
    }
  }

  return (
    <AppShell title="Account">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Your Account</p>
            <h2 className={styles.reportTitle}>Profile &amp; security</h2>
          </div>
          {!mfaLoading && <span className={mfaEnabled ? 'stamp' : 'stamp stamp--alt'}>{mfaEnabled ? 'MFA enabled' : 'MFA disabled'}</span>}
        </div>

        {forceMfaSetup && !mfaLoading && !mfaEnabled && (
          <Callout variant="danger">
            <strong>Multi-factor authentication is required.</strong> Cluster policy requires every account to have MFA
            enabled — finish the setup below to continue.
          </Callout>
        )}

        <Section title="Profile">
          {profileError ? (
            <Callout variant="danger">{profileError}</Callout>
          ) : profileLoading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : (
            <form className={styles.profileForm} onSubmit={saveProfile}>
              <TextField label="Username" id="username" value={username} onChange={(e) => setUsername(e.target.value)} />
              <div className={styles.profileMeta}>
                <div>
                  <span className={styles.metaLabel}>Roles</span>
                  <div className={styles.roleBadges}>
                    {(profile?.roles ?? []).length === 0 ? (
                      <span className={styles.metaValue}>None</span>
                    ) : (
                      profile!.roles.map((r) => (
                        <Badge variant="success" key={r}>
                          {r}
                        </Badge>
                      ))
                    )}
                  </div>
                </div>
                <div>
                  <span className={styles.metaLabel}>Member since</span>
                  <span className={styles.metaValue}>{fmtDate(profile?.created_at ?? null)}</span>
                </div>
              </div>
              <div className={styles.formActions}>
                <Button type="submit" disabled={savingProfile}>
                  {savingProfile ? 'Saving…' : 'Save profile'}
                </Button>
              </div>
            </form>
          )}
        </Section>

        <Section title="Change password">
          <form className={styles.profileForm} onSubmit={savePassword}>
            <div className={styles.passwordFields}>
              <PasswordField label="New password" id="newPassword" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
              <PasswordField label="Confirm password" id="confirmPassword" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} />
            </div>
            {passwordError && <p className={styles.formError}>{passwordError}</p>}
            <div className={styles.formActions}>
              <Button type="submit" disabled={savingPassword}>
                {savingPassword ? 'Updating…' : 'Update password'}
              </Button>
            </div>
          </form>
        </Section>

        <Section title="Two-factor authentication">
          {mfaLoading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : (
            <>
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
                  <Button variant="ghost" onClick={mfaSetupOpen ? cancelMfaSetup : beginMfaSetup} disabled={mfaBusy}>
                    {mfaBusy ? 'Generating…' : mfaSetupOpen ? 'Cancel' : 'Enable'}
                  </Button>
                )}
              </div>

              {mfaSetupOpen && !mfaEnabled && (
                <form className={styles.mfaSetup} onSubmit={confirmEnableMfa}>
                  <p className={styles.mfaHint}>Scan this QR code with your authenticator app, or enter the secret manually.</p>
                  {mfaQr && <img className={styles.mfaQr} src={`data:image/png;base64,${mfaQr}`} alt="MFA QR Code" />}
                  <div className={styles.mfaSecretBlock}>
                    <div>
                      <span className={styles.metaLabel}>Secret</span>
                      <span className={styles.mono}>{mfaSecret}</span>
                    </div>
                    <div>
                      <span className={styles.metaLabel}>otpauth URI</span>
                      <span className={styles.mono}>{mfaUri}</span>
                    </div>
                  </div>
                  <div className={styles.mfaCodeRow}>
                    <TextField label="6-digit code" id="mfaCode" value={mfaCode} onChange={(e) => setMfaCode(e.target.value)} placeholder="000000" maxLength={6} />
                    <Button type="submit" disabled={mfaCode.trim().length !== 6 || mfaBusy}>
                      {mfaBusy ? 'Verifying…' : 'Confirm'}
                    </Button>
                  </div>
                  {mfaError && <p className={styles.formError}>{mfaError}</p>}
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
                    <Button type="submit" disabled={mfaDisableCode.trim().length !== 6 || mfaBusy}>
                      {mfaBusy ? 'Disabling…' : 'Disable MFA'}
                    </Button>
                  </div>
                  {mfaError && <p className={styles.formError}>{mfaError}</p>}
                </form>
              )}
            </>
          )}
        </Section>

        <Section title="Recovery codes" action={<Button variant="ghost" onClick={regenerateCodes} disabled={rcLoading}>{rcLoading ? 'Generating…' : 'Regenerate'}</Button>}>
          <p className={styles.recoveryStatus}>{rcRemaining === null ? 'Loading…' : `${rcRemaining} codes remaining`}</p>
          {revealedCodes && (
            <div className={styles.calloutWrap}>
              <Callout variant="warning">These codes are shown once and won't be shown again — store them somewhere safe.</Callout>
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
            <form className={styles.tokenForm} onSubmit={submitCreateToken}>
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
                <Button type="submit" disabled={!tokenName || creatingToken}>
                  {creatingToken ? 'Creating…' : 'Create'}
                </Button>
              </div>
            </form>
          )}

          {revealedToken && (
            <div className={styles.calloutWrap}>
              <Callout variant="warning">Copy this token now — it won't be shown again.</Callout>
              <div className={styles.tokenReveal}>
                <span className={styles.mono}>{revealedToken}</span>
                <button
                  type="button"
                  className={styles.copyBtn}
                  onClick={async () => {
                    try {
                      await navigator.clipboard.writeText(revealedToken)
                      toast('Copied to clipboard')
                    } catch {
                      /* clipboard permission denied */
                    }
                    setRevealedToken(null)
                  }}
                  aria-label="Copy token"
                >
                  <Copy size={14} />
                </button>
              </div>
            </div>
          )}

          {tokensError ? (
            <Callout variant="danger">{tokensError}</Callout>
          ) : tokensLoading ? (
            <div className={styles.loadingRow}>Loading…</div>
          ) : tokens.length === 0 ? (
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
                    <td className="cell-muted">{fmtDate(t.created_at)}</td>
                    <td className="cell-muted">{fmtDate(t.expires_at)}</td>
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
