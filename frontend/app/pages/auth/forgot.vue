<template>
  <div class="login-page">
    <div class="login-card">

      <div class="login-card__brand">
        <img src="/logo.svg" alt="Scutum" class="login-card__logo" />
        <div class="login-card__brand-text">
          <h1 class="login-card__title">Reset Password</h1>
          <p class="login-card__sub">Use a recovery code or 2FA to regain access</p>
        </div>
      </div>

      <div v-if="success" class="success-banner">
        <Icon name="lucide:check-circle" size="14" />
        Password updated. <NuxtLink to="/auth/login" class="link">Sign in</NuxtLink>
      </div>

      <template v-else>
        <div v-if="error" class="login-card__error">
          <Icon name="lucide:alert-circle" size="14" /> {{ error }}
        </div>

        <!-- Step 1: enter username + method -->
        <form v-if="step === 1" class="login-form" @submit.prevent="nextStep">
          <div class="form-field">
            <label class="form-label">Username</label>
            <input v-model="form.username" class="form-input" type="text" autocomplete="username"
              placeholder="admin" :disabled="loading" required />
          </div>

          <div class="form-field">
            <label class="form-label">Reset method</label>
            <div class="method-group">
              <label class="method-opt" :class="{ 'method-opt--active': method === 'recovery' }">
                <input v-model="method" type="radio" value="recovery" class="sr-only" />
                <Icon name="lucide:key" size="14" />
                Recovery code
              </label>
              <label class="method-opt" :class="{ 'method-opt--active': method === 'totp' }">
                <input v-model="method" type="radio" value="totp" class="sr-only" />
                <Icon name="lucide:shield-check" size="14" />
                Authenticator app
              </label>
            </div>
          </div>

          <button type="submit" class="login-btn">Continue</button>
          <NuxtLink to="/auth/login" class="back-link">Back to sign in</NuxtLink>
        </form>

        <!-- Step 2: enter code + new password -->
        <form v-else class="login-form" @submit.prevent="submit">

          <div v-if="method === 'recovery'" class="form-field">
            <label class="form-label">Recovery code</label>
            <input v-model="form.recoveryCode" class="form-input" type="text"
              placeholder="xxxx-xxxx-xxxx-xxxx" autocomplete="off" spellcheck="false"
              :disabled="loading" required />
            <p class="form-hint">Enter one of the codes you saved when you created your account.</p>
          </div>

          <div v-else class="form-field">
            <label class="form-label">Authenticator code</label>
            <input v-model="form.totpCode" class="form-input" type="text"
              inputmode="numeric" maxlength="6" placeholder="000000"
              autocomplete="one-time-code" :disabled="loading" required />
          </div>

          <div class="form-field">
            <label class="form-label">New password</label>
            <div class="input-wrap">
              <input v-model="form.newPassword" class="form-input"
                :type="showPw ? 'text' : 'password'" placeholder="••••••••"
                autocomplete="new-password" :disabled="loading" required />
              <button type="button" class="input-wrap__eye" @click="showPw = !showPw" tabindex="-1">
                <Icon :name="showPw ? 'lucide:eye-off' : 'lucide:eye'" size="14" />
              </button>
            </div>
            <div class="pw-strength">
              <div class="pw-strength__bar" :style="{ width: pwStrength.pct + '%', background: pwStrength.color }" />
            </div>
          </div>

          <div class="form-field">
            <label class="form-label">Confirm new password</label>
            <input v-model="form.confirm" class="form-input" type="password"
              placeholder="••••••••" autocomplete="new-password" :disabled="loading" required />
          </div>

          <button type="submit" class="login-btn" :disabled="loading || !canSubmit">
            <span v-if="loading" class="login-btn__spinner" />
            <span v-else>Reset password</span>
          </button>
          <button type="button" class="login-btn login-btn--ghost" @click="step = 1">Back</button>
        </form>
      </template>

    </div>
  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'auth' })

const api = useApi()

const step   = ref(1)
const method = ref<'recovery' | 'totp'>('recovery')
const showPw = ref(false)
const loading = ref(false)
const error   = ref('')
const success = ref(false)

const form = reactive({
  username:     '',
  recoveryCode: '',
  totpCode:     '',
  newPassword:  '',
  confirm:      '',
})

const pwStrength = computed(() => {
  const p = form.newPassword
  let score = 0
  if (p.length >= 12) score++
  if (/[A-Z]/.test(p)) score++
  if (/[0-9]/.test(p)) score++
  if (/[^A-Za-z0-9]/.test(p)) score++
  const colors = ['#ef4444', '#f97316', '#eab308', '#22c55e']
  return { pct: (score / 4) * 100, color: colors[score - 1] ?? '#3b82f6' }
})

const canSubmit = computed(() => {
  if (form.newPassword.length < 12) return false
  if (form.newPassword !== form.confirm) return false
  if (method.value === 'recovery' && !form.recoveryCode.trim()) return false
  if (method.value === 'totp' && form.totpCode.length !== 6) return false
  return true
})

function nextStep() {
  if (!form.username.trim()) { error.value = 'Username is required.'; return }
  error.value = ''
  step.value = 2
}

async function submit() {
  if (!canSubmit.value) return
  if (form.newPassword !== form.confirm) { error.value = 'Passwords do not match.'; return }
  error.value = ''
  loading.value = true
  try {
    await api.forgotPassword({
      username:      form.username,
      new_password:  form.newPassword,
      recovery_code: method.value === 'recovery' ? form.recoveryCode.trim() : undefined,
      totp_code:     method.value === 'totp'     ? form.totpCode : undefined,
    })
    success.value = true
  } catch (e: any) {
    error.value = e?.data ?? e?.message ?? 'Reset failed. Check your code and try again.'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* Reuse login-page styles — shared via auth layout */
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
.login-card__brand { display: flex; align-items: center; gap: 0.875rem; }
.login-card__logo  { width: 48px; height: 48px; object-fit: contain; }
.login-card__brand-text { display: flex; flex-direction: column; gap: 0.15rem; }
.login-card__title { margin: 0; font-size: 1.3rem; font-weight: 800; color: var(--text-primary); }
.login-card__sub   { margin: 0; font-size: 0.72rem; color: var(--text-dim); }
.login-card__error {
  display: flex; align-items: center; gap: 0.5rem;
  background: var(--danger-bg); border: 1px solid #7f1d1d55;
  border-radius: 0.375rem; padding: 0.6rem 0.875rem;
  font-size: 0.8rem; color: var(--danger-lighter);
}
.success-banner {
  display: flex; align-items: center; gap: 0.5rem;
  background: rgba(34,197,94,0.08); border: 1px solid rgba(34,197,94,0.25);
  border-radius: 0.375rem; padding: 0.6rem 0.875rem;
  font-size: 0.85rem; color: #86efac;
}
.link { color: var(--accent-light); text-decoration: none; }
.link:hover { text-decoration: underline; }

.login-form { display: flex; flex-direction: column; gap: 1rem; }
.form-field { display: flex; flex-direction: column; gap: 0.375rem; }
.form-label { font-size: 0.78rem; color: var(--text-tertiary); }
.form-hint  { font-size: 0.72rem; color: var(--text-dim); margin: 0; }
.form-input {
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 0.375rem; padding: 0.6rem 0.875rem;
  font-size: 0.875rem; color: var(--text-primary); font-family: inherit;
  outline: none; transition: border-color 0.15s; width: 100%; box-sizing: border-box;
}
.form-input:focus { border-color: var(--accent); }
.form-input:disabled { opacity: 0.5; cursor: not-allowed; }
.form-input::placeholder { color: var(--text-subtle); }
.input-wrap { position: relative; }
.input-wrap .form-input { padding-right: 2.5rem; }
.input-wrap__eye {
  position: absolute; right: 0.625rem; top: 50%; transform: translateY(-50%);
  background: none; border: none; color: var(--text-dim); cursor: pointer;
  padding: 0.25rem; display: flex; align-items: center;
}

.method-group { display: flex; gap: 0.5rem; }
.method-opt {
  flex: 1; display: flex; align-items: center; justify-content: center; gap: 0.4rem;
  padding: 0.55rem 0.75rem; border: 1px solid var(--border-strong);
  border-radius: 0.375rem; font-size: 0.8rem; color: var(--text-muted);
  cursor: pointer; transition: all 0.15s;
}
.method-opt:hover { color: var(--text-primary); border-color: var(--accent-soft); }
.method-opt--active { color: var(--accent-light); border-color: var(--accent); background: var(--accent-dim); }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0,0,0,0); }

.pw-strength { height: 3px; background: var(--border); border-radius: 2px; margin-top: 0.375rem; overflow: hidden; }
.pw-strength__bar { height: 100%; border-radius: 2px; transition: width 0.25s, background 0.25s; }

.login-btn {
  display: flex; align-items: center; justify-content: center; gap: 0.5rem;
  background: var(--accent); color: #fff; border: none;
  border-radius: 0.375rem; padding: 0.65rem 1rem;
  font-size: 0.875rem; font-weight: 600; font-family: inherit;
  cursor: pointer; transition: background 0.15s, opacity 0.15s; width: 100%;
}
.login-btn:hover:not(:disabled) { background: var(--accent-hover); }
.login-btn:disabled { opacity: 0.55; cursor: not-allowed; }
.login-btn--ghost {
  background: none; border: 1px solid var(--border-strong);
  color: var(--text-muted); margin-top: -0.25rem;
}
.login-btn--ghost:hover:not(:disabled) { background: var(--hover-bg); color: var(--text-primary); }
.login-btn__spinner {
  width: 14px; height: 14px; border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff; border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
.back-link {
  text-align: center; font-size: 0.78rem; color: var(--text-dim);
  text-decoration: none; margin-top: -0.25rem;
}
.back-link:hover { color: var(--text-muted); }
</style>
