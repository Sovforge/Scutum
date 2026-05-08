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


    </div>

  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'auth' })

const api  = useApi()
const auth = useAuth()

const form = reactive({ identity: '', password: '', remember: false })
const loading     = ref(false)
const error       = ref('')
const showPw      = ref(false)
const totpRequired = ref(false)
const totpCode    = ref('')

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

/* ── Footer ────────────────────────────────────────────────────────────── */
.login-card__footer {
  margin: 0;
  font-size: 0.7rem;
  color: var(--border-strong);
  text-align: center;
}
.login-card__footer-mesh { color: var(--success-glow); }
</style>
