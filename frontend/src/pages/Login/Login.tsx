import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ShieldCheck } from 'lucide-react'
import AuthShell from '../AuthShell/AuthShell'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import PasswordField from '../../components/ui/PasswordField/PasswordField'
import Checkbox from '../../components/ui/Checkbox/Checkbox'
import Callout from '../../components/ui/Callout/Callout'
import { ApiError, getSSOProviders, login, setToken, type SSOProvider } from '../../lib/api'
import styles from './Login.module.css'

function Login() {
  const navigate = useNavigate()
  const [identity, setIdentity] = useState('')
  const [password, setPassword] = useState('')
  const [remember, setRemember] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [ssoProviders, setSsoProviders] = useState<SSOProvider[]>([])

  const [totpRequired, setTotpRequired] = useState(false)
  const [totpCode, setTotpCode] = useState('')

  useEffect(() => {
    const hash = window.location.hash
    if (hash.startsWith('#sso-token=')) {
      const token = hash.slice('#sso-token='.length)
      history.replaceState(null, '', window.location.pathname + window.location.search)
      setToken(token)
      navigate('/dashboard', { replace: true })
      return
    }
    getSSOProviders()
      .then(setSsoProviders)
      .catch(() => setSsoProviders([]))
  }, [navigate])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!identity || !password) {
      setError('Both fields are required.')
      return
    }
    setError('')
    setLoading(true)
    try {
      const res = await login(identity, password)
      if (res.totp_required) {
        setTotpRequired(true)
        return
      }
      setToken(res.token!)
      if (res.mfa_setup_required) {
        navigate('/account', { state: { forceMfaSetup: true } })
      } else {
        navigate('/dashboard')
      }
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Invalid credentials.')
    } finally {
      setLoading(false)
    }
  }

  async function handleTotpSubmit(e: FormEvent) {
    e.preventDefault()
    if (totpCode.length !== 6) return
    setError('')
    setLoading(true)
    try {
      const res = await login(identity, password, totpCode)
      setToken(res.token!)
      navigate('/dashboard')
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Invalid code.')
      setTotpCode('')
    } finally {
      setLoading(false)
    }
  }

  return (
    <AuthShell
      eyebrow="Access Control"
      title="Log in."
      subtitle="Authenticate against your own instance — nothing about this request leaves your mesh."
    >
      {error && <Callout variant="danger">{error}</Callout>}

      {!totpRequired ? (
        <form onSubmit={handleSubmit}>
          <TextField
            label="Email or username"
            id="identity"
            autoComplete="username"
            placeholder="admin@scutum.local"
            value={identity}
            disabled={loading}
            onChange={(e) => setIdentity(e.target.value)}
          />
          <PasswordField
            label={
              <span className={styles.passwordLabel}>
                Password
                <Link to="/forgot" className={styles.forgotLink}>
                  Forgot?
                </Link>
              </span>
            }
            id="password"
            autoComplete="current-password"
            value={password}
            disabled={loading}
            onChange={(e) => setPassword(e.target.value)}
          />

          <Checkbox checked={remember} onChange={(e) => setRemember(e.target.checked)}>
            Remember this device for 30 days
          </Checkbox>

          <Button type="submit" block disabled={loading}>
            {loading ? 'Signing in…' : 'Sign In'}
          </Button>
        </form>
      ) : (
        <form onSubmit={handleTotpSubmit}>
          <div className={styles.mfaCard}>
            <div className={styles.mfaIcon}>
              <ShieldCheck size={20} />
            </div>
            <p className={styles.mfaLabel}>Enter the 6-digit code from your authenticator app</p>
            <input
              className={styles.mfaInput}
              inputMode="numeric"
              maxLength={6}
              placeholder="000000"
              autoFocus
              disabled={loading}
              value={totpCode}
              onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, ''))}
            />
          </div>
          <Button type="submit" block disabled={loading || totpCode.length !== 6}>
            {loading ? 'Verifying…' : 'Verify'}
          </Button>
          <Button
            type="button"
            variant="ghost"
            block
            className={styles.backBtn}
            disabled={loading}
            onClick={() => {
              setTotpRequired(false)
              setTotpCode('')
            }}
          >
            Back
          </Button>
        </form>
      )}

      {!totpRequired && ssoProviders.length > 0 && (
        <>
          <div className={styles.ssoDivider}>
            <span />
            <span>or continue with</span>
            <span />
          </div>
          <div className={styles.ssoButtons}>
            {ssoProviders.map((p) => (
              <a key={p.id} className={styles.ssoBtn} href={`/api/auth/sso/${p.id}`}>
                {p.name}
              </a>
            ))}
          </div>
        </>
      )}
    </AuthShell>
  )
}

export default Login
