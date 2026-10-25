import { useMemo, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { KeyRound, ShieldCheck } from 'lucide-react'
import AuthShell from '../AuthShell/AuthShell'
import Button from '../../components/ui/Button/Button'
import TextField from '../../components/ui/TextField/TextField'
import PasswordField from '../../components/ui/PasswordField/PasswordField'
import Callout from '../../components/ui/Callout/Callout'
import { ApiError, forgotPassword } from '../../lib/api'
import styles from './ForgotPassword.module.css'

type Method = 'recovery' | 'totp'

function passwordStrength(pw: string) {
  let score = 0
  if (pw.length >= 12) score++
  if (/[A-Z]/.test(pw)) score++
  if (/[0-9]/.test(pw)) score++
  if (/[^A-Za-z0-9]/.test(pw)) score++
  const colors = ['#ef4444', '#f97316', '#eab308', '#22c55e']
  return { pct: (score / 4) * 100, color: colors[score - 1] ?? 'var(--ink-line-strong)' }
}

function ForgotPassword() {
  const [step, setStep] = useState<1 | 2>(1)
  const [method, setMethod] = useState<Method>('recovery')
  const [username, setUsername] = useState('')
  const [recoveryCode, setRecoveryCode] = useState('')
  const [totpCode, setTotpCode] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)

  const strength = useMemo(() => passwordStrength(newPassword), [newPassword])
  const canSubmit =
    newPassword.length >= 12 &&
    newPassword === confirm &&
    (method === 'recovery' ? recoveryCode.trim().length > 0 : totpCode.length === 6)

  function nextStep(e: FormEvent) {
    e.preventDefault()
    if (!username.trim()) {
      setError('Username is required.')
      return
    }
    setError('')
    setStep(2)
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (newPassword !== confirm) {
      setError('Passwords do not match.')
      return
    }
    if (!canSubmit) return
    setError('')
    setLoading(true)
    try {
      await forgotPassword({
        username,
        new_password: newPassword,
        recovery_code: method === 'recovery' ? recoveryCode.trim() : undefined,
        totp_code: method === 'totp' ? totpCode : undefined,
      })
      setSuccess(true)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Reset failed. Check your code and try again.')
    } finally {
      setLoading(false)
    }
  }

  if (success) {
    return (
      <AuthShell eyebrow="Access Control" title="Reset password.">
        <Callout variant="success">
          Password updated.{' '}
          <Link to="/login" className={styles.link}>
            Sign in
          </Link>
        </Callout>
      </AuthShell>
    )
  }

  return (
    <AuthShell
      eyebrow="Access Control"
      title="Reset password."
      subtitle="Use a recovery code or your authenticator app to regain access."
    >
      {error && <Callout variant="danger">{error}</Callout>}

      {step === 1 ? (
        <form onSubmit={nextStep}>
          <TextField
            label="Username"
            id="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="admin"
            autoComplete="username"
          />

          <div className={styles.methodGroup}>
            <button
              type="button"
              className={method === 'recovery' ? `${styles.methodOpt} ${styles.methodOptActive}` : styles.methodOpt}
              onClick={() => setMethod('recovery')}
            >
              <KeyRound size={14} />
              Recovery code
            </button>
            <button
              type="button"
              className={method === 'totp' ? `${styles.methodOpt} ${styles.methodOptActive}` : styles.methodOpt}
              onClick={() => setMethod('totp')}
            >
              <ShieldCheck size={14} />
              Authenticator app
            </button>
          </div>

          <Button type="submit" block>
            Continue
          </Button>
          <Link to="/login" className={styles.backLink}>
            Back to sign in
          </Link>
        </form>
      ) : (
        <form onSubmit={submit}>
          {method === 'recovery' ? (
            <TextField
              label="Recovery code"
              id="recoveryCode"
              value={recoveryCode}
              onChange={(e) => setRecoveryCode(e.target.value)}
              placeholder="xxxx-xxxx-xxxx-xxxx"
              autoComplete="off"
              spellCheck={false}
              hint="Enter one of the codes you saved when you created your account."
              disabled={loading}
            />
          ) : (
            <TextField
              label="Authenticator code"
              id="totpCode"
              inputMode="numeric"
              maxLength={6}
              value={totpCode}
              onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, ''))}
              placeholder="000000"
              autoComplete="one-time-code"
              disabled={loading}
            />
          )}

          <PasswordField
            label="New password"
            id="newPassword"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            autoComplete="new-password"
            disabled={loading}
            hint={
              <div className={styles.strengthTrack}>
                <div className={styles.strengthBar} style={{ width: `${strength.pct}%`, background: strength.color }} />
              </div>
            }
          />

          <TextField
            label="Confirm new password"
            id="confirm"
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            autoComplete="new-password"
            disabled={loading}
          />

          <Button type="submit" block disabled={!canSubmit || loading}>
            {loading ? 'Resetting…' : 'Reset password'}
          </Button>
          <Button type="button" variant="ghost" block className={styles.backBtn} disabled={loading} onClick={() => setStep(1)}>
            Back
          </Button>
        </form>
      )}
    </AuthShell>
  )
}

export default ForgotPassword
