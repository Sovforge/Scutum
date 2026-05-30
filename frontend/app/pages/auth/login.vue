<template>
  <div class="login-page">

    <div class="login-card">

      <!-- Logo -->
      <div class="login-card__brand">
        <img src="/logo.svg" alt="Scutum" class="login-card__logo" />
        <div class="login-card__brand-text">
          <h1 class="login-card__title">Scutum</h1>
          <p class="login-card__sub">Distributed infrastructure control plane</p>
        </div>
      </div>

      <!-- Error banner -->
      <div v-if="error" class="login-card__error">
        <Icon name="lucide:alert-circle" size="14" />
        {{ error }}
      </div>

      <!-- Credentials form -->
      <form v-if="!totpRequired" class="login-form" @submit.prevent="submit">

        <div class="form-field">
          <label class="form-label">Email or username</label>
          <input
            v-model="form.identity"
            class="form-input"
            type="text"
            autocomplete="username"
            placeholder="admin@scutum.local"
            :disabled="loading"
            required
          />
        </div>

        <div class="form-field">
          <label class="form-label">
            Password
            <NuxtLink to="/auth/forgot" class="form-label__link">Forgot?</NuxtLink>
          </label>
          <div class="input-wrap">
            <input
              v-model="form.password"
              class="form-input"
              :type="showPw ? 'text' : 'password'"
              autocomplete="current-password"
              placeholder="••••••••"
              :disabled="loading"
              required
            />
            <button type="button" class="input-wrap__eye" @click="showPw = !showPw" tabindex="-1">
              <Icon :name="showPw ? 'lucide:eye-off' : 'lucide:eye'" size="14" />
            </button>
          </div>
        </div>

        <label class="remember-row">
          <input v-model="form.remember" type="checkbox" class="remember-checkbox" />
          <span>Remember this device for 30 days</span>
        </label>

        <button type="submit" class="login-btn" :disabled="loading">
          <span v-if="loading" class="login-btn__spinner" />
          <span v-else>Sign in</span>
        </button>

      </form>

      <!-- TOTP step -->
      <form v-else class="login-form" @submit.prevent="submitTotp">
        <div class="mfa-card">
          <div class="mfa-card__icon"><Icon name="lucide:shield-check" size="20" /></div>
          <p class="mfa-card__label">Enter the 6-digit code from your authenticator app</p>
          <input
            v-model="totpCode"
            class="form-input mfa-input"
            inputmode="numeric"
            maxlength="6"
            placeholder="000000"
            autofocus
            :disabled="loading"
          />
        </div>
        <button type="submit" class="login-btn" :disabled="loading || totpCode.length !== 6">
          <span v-if="loading" class="login-btn__spinner" />
          <span v-else>Verify</span>
        </button>
        <button type="button" class="login-btn login-btn--ghost" @click="totpRequired = false; totpCode = ''">
          Back
        </button>
      </form>

      <!-- SSO providers -->
      <template v-if="ssoProviders.length > 0 && !totpRequired">
        <div class="sso-divider">
          <span class="sso-divider__line" />
          <span class="sso-divider__text">or continue with</span>
          <span class="sso-divider__line" />
        </div>
        <div class="sso-buttons">
          <button
            v-for="p in ssoProviders"
            :key="p.id"
            type="button"
            class="sso-btn"
            @click="loginSSO(p.id)"
          >
            <span class="sso-btn__icon" v-html="ssoIcon(p.icon)" />
            <span>{{ p.name }}</span>
          </button>
        </div>
      </template>

    </div>

  </div>
</template>

<script setup lang="ts">
import type { SSOProvider } from '~/composables/useApi'

definePageMeta({ layout: 'auth' })

const api  = useApi()
const auth = useAuth()

const form = reactive({ identity: '', password: '', remember: false })
const loading      = ref(false)
const error        = ref('')
const showPw       = ref(false)
const totpRequired = ref(false)
const totpCode     = ref('')
const ssoProviders = ref<SSOProvider[]>([])

onMounted(async () => {
  const hash = window.location.hash
  if (hash.startsWith('#sso-token=')) {
    const token = hash.slice('#sso-token='.length)
    history.replaceState(null, '', window.location.pathname + window.location.search)
    auth.setToken(token)
    await navigateTo('/')
    return
  }
  ssoProviders.value = await api.getSSOProviders()
})

function loginSSO(providerId: string) {
  window.location.href = '/api/auth/sso/' + providerId
}

const ssoIconMap: Record<string, string> = {
  microsoft: `<svg viewBox="0 0 21 21" width="16" height="16" fill="none" xmlns="http://www.w3.org/2000/svg"><rect x="1" y="1" width="9" height="9" fill="#f25022"/><rect x="11" y="1" width="9" height="9" fill="#7fba00"/><rect x="1" y="11" width="9" height="9" fill="#00a4ef"/><rect x="11" y="11" width="9" height="9" fill="#ffb900"/></svg>`,
  github: `<svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>`,
}

function ssoIcon(icon: string): string {
  if (ssoIconMap[icon]) return ssoIconMap[icon]
  const lucideMap: Record<string, string> = {
    authentik: 'shield',
    keycloak: 'key',
    oidc: 'log-in',
  }
  const lucideName = lucideMap[icon] ?? 'log-in'
  return `<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" xmlns="http://www.w3.org/2000/svg">${lucidePaths[lucideName] ?? ''}</svg>`
}

const lucidePaths: Record<string, string> = {
  shield: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>',
  key: '<circle cx="7.5" cy="15.5" r="5.5"/><path d="M21 2l-9.6 9.6"/><path d="M15.5 7.5l3 3L22 7l-3-3"/>',
  'log-in': '<path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"/><polyline points="10 17 15 12 10 7"/><line x1="15" y1="12" x2="3" y2="12"/>',
}

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const res = await api.login(form.identity, form.password)
    if (res.totp_required) {
      totpRequired.value = true
      return
    }
    auth.setToken(res.token!)
    await navigateTo('/')
  } catch (e: any) {
    error.value = e?.data ?? e?.message ?? 'Invalid credentials.'
  } finally {
    loading.value = false
  }
}

async function submitTotp() {
  if (totpCode.value.length !== 6) return
  error.value = ''
  loading.value = true
  try {
    const res = await api.login(form.identity, form.password, totpCode.value)
    auth.setToken(res.token!)
    await navigateTo('/')
  } catch (e: any) {
    error.value = e?.data ?? e?.message ?? 'Invalid code.'
    totpCode.value = ''
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-base);
  background-image:
    radial-gradient(ellipse at 20% 50%, var(--accent-dim) 0%, transparent 60%),
    radial-gradient(ellipse at 80% 20%, var(--accent-dim) 0%, transparent 50%);
  padding: 1.5rem;
}

/* ── Card ─────────────────────────────────────────────────────────────── */
.login-card {
  width: 100%;
  max-width: 400px;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  padding: 2rem;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  box-shadow: 0 24px 64px rgba(0,0,0,0.6);
}

/* ── Brand ─────────────────────────────────────────────────────────────── */
.login-card__brand {
  display: flex;
  align-items: center;
  gap: 0.875rem;
}
.login-card__logo {
  width: 48px;
  height: 48px;
  object-fit: contain;
  flex-shrink: 0;
}
.login-card__brand-text {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}
.login-card__title {
  margin: 0;
  font-size: 1.4rem;
  font-weight: 800;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}
.login-card__sub {
  margin: 0;
  font-size: 0.72rem;
  color: var(--text-dim);
}

/* ── Error ─────────────────────────────────────────────────────────────── */
.login-card__error {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background: var(--danger-bg);
  border: 1px solid #7f1d1d55;
  border-radius: 0.375rem;
  padding: 0.6rem 0.875rem;
  font-size: 0.8rem;
  color: var(--danger-lighter);
}

/* ── Form ─────────────────────────────────────────────────────────────── */
.login-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.form-field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}
.form-label {
  font-size: 0.78rem;
  color: var(--text-tertiary);
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.form-label__link {
  font-size: 0.72rem;
  color: var(--accent);
  text-decoration: none;
}
.form-label__link:hover { color: var(--accent-light); }
.form-input {
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.6rem 0.875rem;
  font-size: 0.875rem;
  color: var(--text-primary);
  font-family: inherit;
  outline: none;
  transition: border-color 0.15s;
  width: 100%;
  box-sizing: border-box;
}
.form-input:focus { border-color: var(--accent); }
.form-input:disabled { opacity: 0.5; cursor: not-allowed; }
.form-input::placeholder { color: var(--text-subtle); }

.input-wrap {
  position: relative;
}
.input-wrap .form-input { padding-right: 2.5rem; }
.input-wrap__eye {
  position: absolute;
  right: 0.625rem;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  padding: 0.25rem;
  display: flex;
  align-items: center;
}
.input-wrap__eye:hover { color: var(--text-tertiary); }

.remember-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.78rem;
  color: var(--text-muted);
  cursor: pointer;
  user-select: none;
}
.remember-checkbox {
  accent-color: var(--accent);
  width: 14px;
  height: 14px;
  flex-shrink: 0;
}

/* ── Submit button ─────────────────────────────────────────────────────── */
.login-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: 0.375rem;
  padding: 0.65rem 1rem;
  font-size: 0.875rem;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
  width: 100%;
}
.login-btn:hover:not(:disabled) { background: var(--accent-hover); }
.login-btn:disabled { opacity: 0.55; cursor: not-allowed; }
.login-btn--ghost {
  background: none;
  border: 1px solid var(--border-strong);
  color: var(--text-muted);
  margin-top: -0.25rem;
}
.login-btn--ghost:hover:not(:disabled) { background: var(--hover-bg); color: var(--text-primary); }

.login-btn__spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* ── MFA ───────────────────────────────────────────────────────────────── */
.mfa-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  padding: 1.25rem;
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.5rem;
}
.mfa-card__icon {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--accent-subtle);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent-light);
}
.mfa-card__label {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-tertiary);
}
.mfa-input {
  text-align: center;
  font-size: 1.4rem;
  letter-spacing: 0.3em;
  font-family: monospace;
  max-width: 180px;
}

/* ── SSO ───────────────────────────────────────────────────────────────── */
.sso-divider {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}
.sso-divider__line {
  flex: 1;
  height: 1px;
  background: var(--border);
}
.sso-divider__text {
  font-size: 0.72rem;
  color: var(--text-dim);
  white-space: nowrap;
}
.sso-buttons {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.sso-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.625rem;
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.6rem 1rem;
  font-size: 0.875rem;
  font-weight: 500;
  font-family: inherit;
  color: var(--text-primary);
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s;
  width: 100%;
}
.sso-btn:hover { background: var(--hover-bg); border-color: var(--accent); }
.sso-btn__icon {
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

/* ── Footer ────────────────────────────────────────────────────────────── */
.login-card__footer {
  margin: 0;
  font-size: 0.7rem;
  color: var(--border-strong);
  text-align: center;
}
.login-card__footer-mesh { color: var(--success-glow); }
</style>
