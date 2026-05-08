<template>
  <div class="account-page">

    <!-- Header -->
    <div class="page-header">
      <div class="page-header__title">
        <div class="account-avatar">{{ initials }}</div>
        <div>
          <h2 class="page-header__name">{{ profile.username || '…' }}</h2>
          <span class="page-header__role">{{ profile.roles.join(', ') || 'No roles assigned' }}</span>
        </div>
      </div>
    </div>

    <div class="account-grid">

      <!-- ── Left column ──────────────────────────────────────────────── -->
      <div class="account-col">

        <!-- Profile -->
        <UiCard title="Profile">
          <div class="form-grid">
            <div class="form-row">
              <label class="form-label">Username</label>
              <input v-model="profile.username" class="form-input" />
            </div>
            <div class="form-row">
              <label class="form-label">Roles</label>
              <input :value="profile.roles.join(', ') || 'None'" class="form-input" disabled />
            </div>
          </div>
          <p v-if="profileError" class="field-error">{{ profileError }}</p>
          <div class="card-actions">
            <div v-if="profileSaved" class="save-ok">
              <Icon name="lucide:check" size="12" /> Saved
            </div>
            <button class="action-btn action-btn--primary" :disabled="profileSaving" @click="saveProfile">
              {{ profileSaving ? 'Saving…' : 'Save changes' }}
            </button>
          </div>
        </UiCard>

        <!-- Change password -->
        <UiCard title="Change Password">
          <div class="form-grid">
            <div class="form-row">
              <label class="form-label">New password</label>
              <div class="input-wrap">
                <input v-model="pw.next" class="form-input" :type="showPw[0] ? 'text' : 'password'" placeholder="••••••••" />
                <button type="button" class="input-eye" @click="showPw[0] = !showPw[0]" tabindex="-1">
                  <Icon :name="showPw[0] ? 'lucide:eye-off' : 'lucide:eye'" size="13" />
                </button>
              </div>
              <div class="pw-strength">
                <div class="pw-strength__bar" :style="{ width: pwStrength.pct + '%', background: pwStrength.color }" />
              </div>
            </div>
            <div class="form-row">
              <label class="form-label">Confirm new password</label>
              <input v-model="pw.confirm" class="form-input" type="password" placeholder="••••••••" />
            </div>
          </div>
          <p v-if="pwError"   class="field-error">{{ pwError }}</p>
          <p v-if="pwSuccess" class="field-ok">Password updated.</p>
          <div class="card-actions">
            <button class="action-btn action-btn--primary" :disabled="pwSaving" @click="changePassword">
              {{ pwSaving ? 'Updating…' : 'Update password' }}
            </button>
          </div>
        </UiCard>

      </div>

      <!-- ── Right column ─────────────────────────────────────────────── -->
      <div class="account-col">

        <!-- MFA -->
        <UiCard title="Multi-Factor Authentication">
          <div class="mfa-status" :class="mfa.enabled ? 'mfa-status--on' : 'mfa-status--off'">
            <Icon :name="mfa.enabled ? 'lucide:shield-check' : 'lucide:shield-off'" size="16" />
            <span>{{ mfa.enabled ? 'MFA enabled (TOTP)' : 'MFA not configured' }}</span>
          </div>

          <!-- Not set up yet -->
          <template v-if="!mfa.enabled && !mfa.setup">
            <p class="section-note">Use an authenticator app (Google Authenticator, Authy, 1Password) to add the secret and generate time-based codes.</p>
            <button class="action-btn action-btn--ghost" :disabled="mfa.loading" @click="beginSetup">
              <Icon name="lucide:shield-plus" size="13" />
              {{ mfa.loading ? 'Generating…' : 'Set up TOTP' }}
            </button>
          </template>

          <!-- Setup flow: show secret + confirm code -->
          <template v-else-if="mfa.setup && !mfa.enabled">
            <p class="section-note">Add this secret to your authenticator app, then enter the 6-digit code it generates to confirm.</p>

            <p class="section-note">Scan this QR code with your authenticator app.</p>

            <div class="qr-container">
              <img 
                v-if="mfa.qr_code" 
                :src="`data:image/png;base64,${mfa.qr_code}`" 
                alt="MFA QR Code" 
                class="mfa-qr-img" 
              />
            </div>

            <div class="totp-secret-block">
              <div class="totp-secret-row">
                <span class="totp-secret-label">Secret key</span>
                <code class="totp-secret-val">{{ mfa.secret }}</code>
                <button class="icon-btn" title="Copy secret" @click="copySecret">
                  <Icon :name="secretCopied ? 'lucide:check' : 'lucide:copy'" size="13" />
                </button>
              </div>
              <div class="totp-secret-row totp-uri-row">
                <span class="totp-secret-label">URI</span>
                <code class="totp-secret-val totp-uri-val">{{ mfa.uri }}</code>
                <button class="icon-btn" title="Copy URI" @click="copyUri">
                  <Icon :name="uriCopied ? 'lucide:check' : 'lucide:copy'" size="13" />
                </button>
              </div>
            </div>

            <div class="form-row" style="margin-top:0.875rem">
              <label class="form-label">Enter 6-digit code to confirm setup</label>
              <input v-model="mfa.code" class="form-input mfa-code-input" maxlength="6" inputmode="numeric" placeholder="000000" @keyup.enter="confirmEnable" />
            </div>
            <p v-if="mfa.error" class="field-error">{{ mfa.error }}</p>
            <div class="card-actions">
              <button class="action-btn action-btn--ghost" @click="cancelSetup">Cancel</button>
              <button class="action-btn action-btn--primary" :disabled="mfa.loading || mfa.code.length !== 6" @click="confirmEnable">
                {{ mfa.loading ? 'Verifying…' : 'Enable MFA' }}
              </button>
            </div>
          </template>

          <!-- Enabled: show disable flow -->
          <template v-else-if="mfa.enabled">
            <p class="section-note">MFA is active. All logins require a TOTP code from your authenticator app.</p>
            <template v-if="!mfa.disabling">
              <button class="action-btn action-btn--danger" @click="mfa.disabling = true">
                <Icon name="lucide:shield-off" size="13" />
                Disable MFA
              </button>
            </template>
            <template v-else>
              <div class="form-row" style="margin-top:0.5rem">
                <label class="form-label">Enter current TOTP code to confirm</label>
                <input v-model="mfa.code" class="form-input mfa-code-input" maxlength="6" inputmode="numeric" placeholder="000000" @keyup.enter="confirmDisable" />
              </div>
              <p v-if="mfa.error" class="field-error">{{ mfa.error }}</p>
              <div class="card-actions">
                <button class="action-btn action-btn--ghost" @click="mfa.disabling = false; mfa.code = ''; mfa.error = ''">Cancel</button>
                <button class="action-btn action-btn--danger" :disabled="mfa.loading || mfa.code.length !== 6" @click="confirmDisable">
                  {{ mfa.loading ? 'Disabling…' : 'Confirm disable' }}
                </button>
              </div>
            </template>
          </template>
        </UiCard>

        <!-- Recovery codes -->
        <UiCard title="Recovery Codes">
          <p class="section-note">
            Recovery codes let you reset your password if you lose access to your account.
            Each code can only be used once.
          </p>

          <div class="recovery-status">
            <Icon name="lucide:key" size="14" class="recovery-status__icon" />
            <span v-if="rcLoading">Loading…</span>
            <span v-else>
              <strong>{{ rcRemaining }}</strong> of {{ rcTotal }} codes remaining
            </span>
          </div>

          <!-- Show newly generated codes -->
          <div v-if="rcNewCodes.length" class="rc-new-codes">
            <div class="rc-new-codes__header">
              <Icon name="lucide:alert-triangle" size="13" class="rc-warn-icon" />
              Save these codes now — they will not be shown again.
            </div>
            <div class="rc-grid">
              <code v-for="(code, i) in rcNewCodes" :key="i" class="rc-code">{{ code }}</code>
            </div>
            <div class="card-actions" style="margin-top:0.75rem">
              <button class="action-btn action-btn--ghost" @click="copyAllCodes">
                <Icon :name="rcCopied ? 'lucide:check' : 'lucide:copy'" size="13" />
                {{ rcCopied ? 'Copied' : 'Copy all' }}
              </button>
              <button class="action-btn action-btn--ghost" @click="rcNewCodes = []">
                <Icon name="lucide:x" size="13" /> Dismiss
              </button>
            </div>
          </div>

          <div class="card-actions" v-if="!rcNewCodes.length">
            <button class="action-btn action-btn--ghost" :disabled="rcLoading" @click="regen">
              <Icon name="lucide:refresh-cw" size="13" :class="{ spin: rcLoading }" />
              {{ rcLoading ? 'Generating…' : 'Regenerate codes' }}
            </button>
          </div>
        </UiCard>

        <!-- API tokens -->
        <UiCard title="API Tokens">
          <table class="mini-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Created</th>
                <th>Last used</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="tok in tokens" :key="tok.id">
                <td class="bold">{{ tok.name }}</td>
                <td class="muted">{{ formatExpiry(tok.created_at) }}</td>
                <td class="muted">{{ formatExpiry(tok.expires_at) }}</td>
                <td>
                  <button class="icon-btn icon-btn--danger" @click="revokeToken(tok.id)" title="Revoke">
                    <Icon name="lucide:trash-2" size="13" />
                  </button>
                </td>
              </tr>
              <tr v-if="tokens.length === 0">
                <td colspan="4" class="muted empty-row">No tokens</td>
              </tr>
            </tbody>
          </table>

          <div v-if="newToken" class="new-token-banner">
            <Icon name="lucide:key-round" size="13" />
            <span>Copy now — this is shown only once:</span>
            <code class="new-token-val">{{ newToken }}</code>
            <button class="icon-btn" @click="copyToken">
              <Icon :name="tokenCopied ? 'lucide:check' : 'lucide:copy'" size="13" />
            </button>
          </div>

          <div class="card-actions">
            <input v-model="newTokenName" class="form-input form-input--sm" placeholder="Token name" />
            <button class="action-btn action-btn--ghost" @click="issueToken">
              <Icon name="lucide:plus" size="13" />
              Create token
            </button>
          </div>
        </UiCard>

        <!-- Active sessions -->
        <UiCard title="Active Sessions">
          <table class="mini-table">
            <thead>
              <tr>
                <th>Device</th>
                <th>IP</th>
                <th>Last seen</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="sess in sessions" :key="sess.id">
                <td>
                  <div class="session-device">
                    <Icon :name="sess.icon" size="13" style="color: #64748b" />
                    <span :class="sess.current ? 'bold' : ''">
                      {{ sess.device }}
                      <UiBadge v-if="sess.current" variant="info" class="current-badge">current</UiBadge>
                    </span>
                  </div>
                </td>
                <td class="mono">{{ sess.ip }}</td>
                <td class="muted">{{ sess.lastSeen }}</td>
                <td>
                  <button
                    v-if="!sess.current"
                    class="icon-btn icon-btn--danger"
                    @click="revokeSession(sess.id)"
                    title="Revoke"
                  >
                    <Icon name="lucide:x" size="13" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <div class="card-actions">
            <button class="action-btn action-btn--danger" @click="revokeAllSessions">Revoke all other sessions</button>
          </div>
        </UiCard>

      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()

// ── Profile ────────────────────────────────────────────────────────────────
const userId   = ref('')
const profile  = reactive({ username: '', roles: [] as string[] })
const initials = computed(() => (profile.username[0] ?? '?').toUpperCase())

const profileSaved  = ref(false)
const profileError  = ref('')
const profileSaving = ref(false)

async function loadProfile() {
  try {
    const me = await api.getMe()
    userId.value    = me.id
    profile.username = me.username
    profile.roles    = me.roles ?? []
  } catch {
    // not logged in or backend down — leave defaults
  }
}

async function saveProfile() {
  profileError.value = ''
  profileSaving.value = true
  try {
    await api.updateUser(userId.value, { username: profile.username })
    profileSaved.value = true
    setTimeout(() => { profileSaved.value = false }, 2000)
  } catch (e: any) {
    profileError.value = e?.data?.error ?? 'Save failed'
  } finally {
    profileSaving.value = false
  }
}

// ── Password ───────────────────────────────────────────────────────────────
const pw      = reactive({ next: '', confirm: '' })
const showPw  = reactive([false, false])
const pwError   = ref('')
const pwSuccess = ref(false)
const pwSaving  = ref(false)

const pwStrength = computed(() => {
  const p = pw.next
  if (!p) return { pct: 0, color: '#2d2d44' }
  let score = 0
  if (p.length >= 8)  score++
  if (p.length >= 12) score++
  if (/[A-Z]/.test(p)) score++
  if (/[0-9]/.test(p)) score++
  if (/[^A-Za-z0-9]/.test(p)) score++
  const colors = ['#ef4444', '#f59e0b', '#f59e0b', '#22c55e', '#22c55e']
  return { pct: (score / 5) * 100, color: colors[score - 1] ?? '#2d2d44' }
})

async function changePassword() {
  pwError.value = ''
  pwSuccess.value = false
  if (pw.next.length < 12) { pwError.value = 'Password must be at least 12 characters.'; return }
  if (pw.next !== pw.confirm) { pwError.value = 'Passwords do not match.'; return }
  pwSaving.value = true
  try {
    await api.updateUser(userId.value, { password: pw.next })
    pw.next = ''; pw.confirm = ''
    pwSuccess.value = true
    setTimeout(() => { pwSuccess.value = false }, 3000)
  } catch (e: any) {
    pwError.value = e?.data?.error ?? 'Update failed'
  } finally {
    pwSaving.value = false
  }
}

// ── MFA ────────────────────────────────────────────────────────────────────
const mfa = reactive({
  enabled:  false,
  setup:    false,
  disabling: false,
  loading:  false,
  code:     '',
  secret:   '',
  uri:      '',
  error:    '',
  qr_code:    '',
})
const secretCopied = ref(false)
const uriCopied    = ref(false)

async function loadMfaStatus() {
  try {
    const s = await api.getMfaStatus()
    mfa.enabled = s.enabled
  } catch {}
}

async function beginSetup() {
  mfa.loading = true
  mfa.error   = ''
  try {
    const res   = await api.setupMfa()
    mfa.secret  = res.secret
    mfa.uri     = res.uri
    mfa.setup   = true
    mfa.code    = ''
    mfa.qr_code = res.qr_code
  } catch (e: any) {
    mfa.error = e?.data?.error ?? 'Failed to start MFA setup'
  } finally {
    mfa.loading = false
  }
}

function cancelSetup() {
  mfa.setup = false; mfa.code = ''; mfa.secret = ''; mfa.uri = ''; mfa.error = ''
}

async function confirmEnable() {
  if (mfa.code.length !== 6) return
  mfa.loading = true
  mfa.error   = ''
  try {
    await api.enableMfa(mfa.code)
    mfa.enabled = true; mfa.setup = false; mfa.code = ''; mfa.secret = ''; mfa.uri = ''
  } catch (e: any) {
    mfa.error = e?.data?.error ?? 'Invalid code — try again'
  } finally {
    mfa.loading = false
  }
}

async function confirmDisable() {
  if (mfa.code.length !== 6) return
  mfa.loading = true
  mfa.error   = ''
  try {
    await api.disableMfa(mfa.code)
    mfa.enabled = false; mfa.disabling = false; mfa.code = ''
  } catch (e: any) {
    mfa.error = e?.data?.error ?? 'Invalid code — try again'
  } finally {
    mfa.loading = false
  }
}

async function copySecret() {
  await navigator.clipboard.writeText(mfa.secret).catch(() => {})
  secretCopied.value = true
  setTimeout(() => { secretCopied.value = false }, 2000)
}

async function copyUri() {
  await navigator.clipboard.writeText(mfa.uri).catch(() => {})
  uriCopied.value = true
  setTimeout(() => { uriCopied.value = false }, 2000)
}

// ── Tokens ─────────────────────────────────────────────────────────────────
const tokens      = ref<APIKeyRecord[]>([])
const newTokenName = ref('')
const newToken     = ref('')
const tokenCopied  = ref(false)
const tokensError  = ref('')

async function loadTokens() {
  tokensError.value = ''
  try {
    tokens.value = await api.listTokens()
  } catch (e: any) {
    tokensError.value = e?.data?.error ?? 'Failed to load tokens'
  }
}

async function issueToken() {
  if (!newTokenName.value.trim()) return
  try {
    const res = await api.createToken(newTokenName.value.trim())
    newToken.value    = res.key
    newTokenName.value = ''
    await loadTokens()
  } catch (e: any) {
    tokensError.value = e?.data?.error ?? 'Failed to create token'
  }
}

async function revokeToken(id: string) {
  try {
    await api.deleteToken(id)
    await loadTokens()
  } catch (e: any) {
    tokensError.value = e?.data?.error ?? 'Revoke failed'
  }
}

async function copyToken() {
  await navigator.clipboard.writeText(newToken.value).catch(() => {})
  tokenCopied.value = true
  setTimeout(() => { tokenCopied.value = false; newToken.value = '' }, 2000)
}

function formatExpiry(iso: string | null): string {
  if (!iso) return 'never'
  try { return new Date(iso).toLocaleDateString() } catch { return iso }
}

// ── Sessions (no API yet — kept as UI placeholder) ────────────────────────
const sessions = ref([
  { id: 's1', device: 'Current session', icon: 'lucide:monitor', ip: '—', lastSeen: 'now', current: true },
])
function revokeSession(id: string) { sessions.value = sessions.value.filter(s => s.id !== id) }
function revokeAllSessions() { sessions.value = sessions.value.filter(s => s.current) }

// ── Recovery codes ─────────────────────────────────────────────────────────
const rcTotal     = 10
const rcRemaining = ref(0)
const rcLoading   = ref(false)
const rcNewCodes  = ref<string[]>([])
const rcCopied    = ref(false)

async function loadRecoveryCodes() {
  try {
    const res = await api.getRecoveryCodeStatus()
    rcRemaining.value = res.remaining
  } catch {}
}

async function regen() {
  rcLoading.value = true
  try {
    const res = await api.regenerateRecoveryCodes()
    rcNewCodes.value  = res.recovery_codes
    rcRemaining.value = res.recovery_codes.length
  } catch {} finally {
    rcLoading.value = false
  }
}

async function copyAllCodes() {
  await navigator.clipboard.writeText(rcNewCodes.value.join('\n'))
  rcCopied.value = true
  setTimeout(() => { rcCopied.value = false }, 2000)
}

onMounted(() => { loadProfile(); loadTokens(); loadMfaStatus(); loadRecoveryCodes() })
</script>

<style scoped>
.account-page { display: flex; flex-direction: column; gap: 1rem; }

/* Header */
.page-header {
  display: flex;
  align-items: center;
  gap: 1rem;
}
.page-header__title {
  display: flex;
  align-items: center;
  gap: 0.875rem;
}
.account-avatar {
  width: 44px;
  height: 44px;
  border-radius: 0.5rem;
  background: linear-gradient(135deg, var(--accent), var(--accent-secondary));
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.875rem;
  font-weight: 700;
  color: #fff;
  letter-spacing: 0.03em;
  flex-shrink: 0;
}
.page-header__name {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--text-primary);
}
.page-header__role { font-size: 0.75rem; color: var(--text-dim); }

/* Two-column layout */
.account-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
  align-items: start;
}
.account-col { display: flex; flex-direction: column; gap: 1rem; }

/* Forms */
.form-grid { display: flex; flex-direction: column; gap: 0.75rem; }
.form-row { display: flex; flex-direction: column; gap: 0.35rem; }
.form-label { font-size: 0.75rem; color: var(--text-tertiary); }
.form-input {
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.5rem 0.75rem;
  font-size: 0.8rem;
  color: var(--text-primary);
  font-family: inherit;
  outline: none;
  transition: border-color 0.15s;
  width: 100%;
  box-sizing: border-box;
}
.form-input:focus { border-color: var(--accent); }
.form-input:disabled { opacity: 0.4; cursor: not-allowed; }
.form-input::placeholder { color: var(--text-subtle); }
.form-input--sm { flex: 1; }

.input-wrap { position: relative; }
.input-wrap .form-input { padding-right: 2.25rem; }
.input-eye {
  position: absolute;
  right: 0.5rem;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  padding: 0.2rem;
  display: flex;
}
.input-eye:hover { color: var(--text-tertiary); }

/* Password strength */
.pw-strength {
  height: 2px;
  background: var(--border-strong);
  border-radius: 1px;
  overflow: hidden;
  margin-top: 0.25rem;
}
.pw-strength__bar {
  height: 100%;
  border-radius: 1px;
  transition: width 0.3s, background 0.3s;
}

/* Card actions */
.card-actions {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  margin-top: 1rem;
  justify-content: flex-end;
}
.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  border-radius: 0.375rem;
  padding: 0.4rem 0.875rem;
  font-size: 0.78rem;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  border: none;
  transition: background 0.15s, color 0.15s;
}
.action-btn--primary { background: var(--accent); color: #fff; }
.action-btn--primary:hover { background: var(--accent-hover); }
.action-btn--ghost {
  background: none;
  border: 1px solid var(--border-strong);
  color: var(--text-tertiary);
}
.action-btn--ghost:hover { background: var(--border); color: var(--text-primary); }
.action-btn--danger {
  background: none;
  border: 1px solid var(--danger-border);
  color: var(--danger-light);
}
.action-btn--danger:hover { background: var(--danger-bg); }

.save-ok {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.75rem;
  color: var(--success-light);
  margin-right: auto;
}

/* MFA */
/* TOTP secret display */
.totp-secret-block {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.5rem;
  padding: 0.75rem 1rem;
}
.totp-secret-row {
  display: flex;
  align-items: center;
  gap: 0.625rem;
}
.totp-secret-label {
  font-size: 0.72rem;
  color: var(--text-muted);
  width: 48px;
  flex-shrink: 0;
}
.totp-secret-val {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.82rem;
  color: var(--text-tertiary);
  flex: 1;
  letter-spacing: 0.08em;
  word-break: break-all;
}
.totp-uri-val {
  font-size: 0.68rem;
  letter-spacing: 0;
}

.mfa-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.82rem;
  font-weight: 600;
  padding: 0.625rem 0.875rem;
  border-radius: 0.375rem;
  margin-bottom: 0.75rem;
}
.mfa-status--on  { background: var(--success-dim); color: var(--success-light); border: 1px solid var(--success-dim); }
.mfa-status--off { background: var(--accent-dim); color: var(--text-muted); border: 1px solid var(--border-subtle); }

.section-note { margin: 0 0 0.75rem; font-size: 0.78rem; color: var(--text-muted); line-height: 1.6; }

.qr-mock {
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.5rem;
  padding: 1.25rem;
  display: flex;
  justify-content: center;
  margin-bottom: 0.75rem;
}
.qr-mock__inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}
.qr-mock__note { margin: 0; font-size: 0.72rem; color: var(--text-dim); }
.qr-mock__secret {
  font-family: monospace;
  font-size: 0.82rem;
  color: var(--text-tertiary);
  letter-spacing: 0.1em;
  background: var(--bg-surface);
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
}

.mfa-code-input {
  text-align: center;
  font-family: monospace;
  font-size: 1.2rem;
  letter-spacing: 0.3em;
}

/* Tokens / Sessions tables */
.mini-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.78rem;
}
.mini-table th {
  text-align: left;
  color: var(--text-dim);
  font-weight: 500;
  padding: 0 0.5rem 0.6rem;
  border-bottom: 1px solid var(--border);
}
.mini-table td {
  padding: 0.55rem 0.5rem;
  color: var(--text-secondary);
  border-bottom: 1px solid var(--border-faint);
}
.mini-table tbody tr:last-child td { border-bottom: none; }

.session-device {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.current-badge { margin-left: 0.25rem; }

.icon-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--text-dim);
  padding: 0.2rem;
  border-radius: 0.25rem;
  display: flex;
  transition: color 0.15s;
}
.icon-btn:hover { color: var(--text-tertiary); }
.icon-btn--danger:hover { color: var(--danger-light); }

.new-token-banner {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background: var(--success-dim);
  border: 1px solid var(--success-dim);
  border-radius: 0.375rem;
  padding: 0.5rem 0.75rem;
  font-size: 0.75rem;
  color: var(--success-light);
  margin: 0.75rem 0;
  flex-wrap: wrap;
}
.new-token-val {
  font-family: monospace;
  color: var(--text-tertiary);
  flex: 1;
  word-break: break-all;
}

.empty-row { text-align: center; padding: 1rem; }
.bold  { color: var(--text-primary); font-weight: 500; }
.muted { color: var(--text-dim); }
.mono  { font-family: monospace; font-size: 0.72rem; color: var(--text-muted); }
.field-error { font-size: 0.75rem; color: var(--danger-lighter); margin: 0.25rem 0 0; }
.field-ok    { font-size: 0.75rem; color: var(--success-light);  margin: 0.25rem 0 0; }
</style>
