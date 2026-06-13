<template>
  <SettingsShell>
    <div class="page-header">
      <div>
        <h1 class="page-title">Emergency Recovery Keys</h1>
        <p class="page-sub">Manage Shamir secret shares for offline master-key recovery</p>
      </div>
    </div>

    <div class="info-note">
      <Icon name="lucide:info" size="13" class="info-note__icon" />
      <span>
        ERK shares use Shamir's Secret Sharing over GF(256). You need
        <strong>threshold</strong> shares to reconstruct the master key — losing fewer than
        <strong>n − threshold</strong> shares is safe. Only the <strong>local</strong> KMS
        provider supports share generation; cloud KMS providers manage the key externally.
      </span>
    </div>

    <!-- Generate new shares -->
    <UiCard title="Generate Shares">
      <p class="section-hint">
        Export the current master key as a fresh set of offline shares. Use this when
        setting up share holders for the first time. Existing shares are not invalidated.
      </p>
      <div class="form-grid">
        <div class="form-row">
          <label class="form-label">Total shares (n)</label>
          <input v-model.number="gen.n" type="number" min="2" max="20" class="form-input form-input--sm" />
        </div>
        <div class="form-row">
          <label class="form-label">Threshold (t)</label>
          <input v-model.number="gen.t" type="number" min="2" :max="gen.n" class="form-input form-input--sm" />
          <span class="form-hint">{{ gen.t }} of {{ gen.n }} shares needed to recover</span>
        </div>
      </div>
      <div class="card-footer">
        <p v-if="genError" class="field-error">{{ genError }}</p>
        <button class="btn-primary" :disabled="genLoading" @click="generateShares">
          <Icon name="lucide:key" size="14" />
          {{ genLoading ? 'Generating…' : 'Generate Shares' }}
        </button>
      </div>
      <ShareList v-if="genShares.length" :shares="genShares" label="New shares — save each one now" />
    </UiCard>

    <!-- Reissue shares -->
    <UiCard title="Reissue Shares">
      <p class="section-hint">
        Provide your existing shares to re-split the master key into a new set. Use this
        when a share holder leaves or a share is suspected compromised. No DEKs are
        re-encrypted; only the share distribution changes.
      </p>

      <div class="shares-input">
        <div v-for="(_, i) in reissue.inputShares" :key="i" class="share-input-row">
          <span class="share-idx">{{ i + 1 }}</span>
          <input
            v-model="reissue.inputShares[i]"
            class="form-input font-mono"
            placeholder="scutum-erk-v1-…"
          />
          <button class="usb-btn" title="Load from USB / IronKey" @click="loadShareFromUsb(i, reissue.inputShares)">
            <Icon name="lucide:usb" size="13" />
          </button>
          <button class="icon-btn icon-btn--danger" @click="reissue.inputShares.splice(i, 1)">
            <Icon name="lucide:x" size="12" />
          </button>
        </div>
        <button class="btn-ghost btn-ghost--sm" @click="reissue.inputShares.push('')">
          <Icon name="lucide:plus" size="13" /> Add share
        </button>
      </div>

      <div class="form-grid mt">
        <div class="form-row">
          <label class="form-label">New total (n)</label>
          <input v-model.number="reissue.n" type="number" min="2" max="20" class="form-input form-input--sm" />
        </div>
        <div class="form-row">
          <label class="form-label">New threshold (t)</label>
          <input v-model.number="reissue.t" type="number" min="2" :max="reissue.n" class="form-input form-input--sm" />
        </div>
      </div>
      <div class="card-footer">
        <p v-if="reissueError" class="field-error">{{ reissueError }}</p>
        <button class="btn-primary" :disabled="reissueLoading || reissue.inputShares.filter(Boolean).length < 2" @click="reissueShares">
          <Icon name="lucide:refresh-cw" size="14" />
          {{ reissueLoading ? 'Reissuing…' : 'Reissue Shares' }}
        </button>
      </div>
      <ShareList v-if="reissueShares_.length" :shares="reissueShares_" label="New shares — old shares are now invalid" />
    </UiCard>

  </SettingsShell>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()

// ── Share display sub-component ───────────────────────────────────────────
const ShareList = defineComponent({
  props: { shares: Array as () => string[], label: String },
  setup(props) {
    const copied = ref<number | null>(null)
    async function copy(s: string, i: number) {
      await navigator.clipboard.writeText(s).catch(() => {})
      copied.value = i
      setTimeout(() => { copied.value = null }, 1800)
    }
    async function exportUsb(s: string, i: number) {
      const filename = `scutum-erk-share-${i + 1}-of-${props.shares!.length}.erk`
      if (typeof window !== 'undefined' && 'showSaveFilePicker' in window) {
        try {
          const handle = await (window as any).showSaveFilePicker({
            suggestedName: filename,
            types: [{ description: 'Scutum ERK Share', accept: { 'text/plain': ['.erk'] } }],
          })
          const w = await handle.createWritable()
          await w.write(s); await w.close()
        } catch (e: any) { if (e.name !== 'AbortError') throw e }
      } else {
        const a = document.createElement('a')
        a.href = 'data:text/plain;charset=utf-8,' + encodeURIComponent(s)
        a.download = filename
        document.body.appendChild(a); a.click(); document.body.removeChild(a)
      }
    }
    return () => h('div', { class: 'share-list' }, [
      h('p', { class: 'share-list__label' }, props.label),
      ...(props.shares ?? []).map((s, i) =>
        h('div', { class: 'share-item', key: i }, [
          h('span', { class: 'share-item__idx' }, `${i + 1}`),
          h('code', { class: 'share-item__val' }, s),
          h('button', { class: 'share-item__btn', title: 'Copy', onClick: () => copy(s, i) },
            h(resolveComponent('Icon'), { name: copied.value === i ? 'lucide:check' : 'lucide:copy', size: '12' })),
          h('button', { class: 'share-item__btn share-item__btn--usb', title: 'Save to USB / IronKey', onClick: () => exportUsb(s, i) },
            h(resolveComponent('Icon'), { name: 'lucide:usb', size: '12' })),
        ])
      ),
    ])
  },
})

// ── Generate ───────────────────────────────────────────────────────────────
const gen = reactive({ n: 5, t: 3 })
const genShares  = ref<string[]>([])
const genLoading = ref(false)
const genError   = ref('')

async function generateShares() {
  genError.value = ''
  genLoading.value = true
  try {
    genShares.value = await api.erkGenerateShares(gen.n, gen.t)
  } catch (e: any) {
    genError.value = e?.data?.error ?? e?.message ?? 'Failed to generate shares'
  } finally {
    genLoading.value = false
  }
}

// ── Reissue ────────────────────────────────────────────────────────────────
const reissue = reactive({ inputShares: ['', ''], n: 5, t: 3 })
const reissueShares_  = ref<string[]>([])
const reissueLoading  = ref(false)
const reissueError    = ref('')

async function reissueShares() {
  reissueError.value = ''
  reissueLoading.value = true
  try {
    const filled = reissue.inputShares.map(s => s.trim()).filter(Boolean)
    reissueShares_.value = await api.erkReissueShares(filled, reissue.n, reissue.t)
  } catch (e: any) {
    reissueError.value = e?.data?.error ?? e?.message ?? 'Failed to reissue shares'
  } finally {
    reissueLoading.value = false
  }
}

// ── USB import helper ──────────────────────────────────────────────────────
async function loadShareFromUsb(index: number, target: string[]) {
  if (typeof window !== 'undefined' && 'showOpenFilePicker' in window) {
    try {
      const [handle] = await (window as any).showOpenFilePicker({
        types: [{ description: 'Scutum ERK Share', accept: { 'text/plain': ['.erk', '.txt'] } }],
        multiple: false,
      })
      target[index] = ((await (await handle.getFile()).text()) as string).trim()
    } catch (e: any) { if (e.name !== 'AbortError') throw e }
  } else {
    const input = document.createElement('input')
    input.type = 'file'; input.accept = '.erk,.txt'
    input.onchange = async () => {
      const file = input.files?.[0]
      if (file) target[index] = (await file.text()).trim()
    }
    input.click()
  }
}
</script>

<style scoped>
.settings-page { display: flex; flex-direction: column; gap: 1.25rem; }

.page-header {
  display: flex; align-items: flex-start; justify-content: space-between;
  gap: 1rem;
}
.page-title { margin: 0; font-size: 1.15rem; font-weight: 700; color: var(--text-primary); }
.page-sub   { margin: 0.2rem 0 0; font-size: 0.8rem; color: var(--text-muted); }

.info-note {
  display: flex; align-items: flex-start; gap: 0.625rem;
  background: rgba(59,130,246,0.07); border: 1px solid rgba(59,130,246,0.2);
  border-radius: 0.5rem; padding: 0.75rem 1rem;
  font-size: 0.8rem; color: #93c5fd; line-height: 1.5;
}
.info-note__icon { color: #60a5fa; flex-shrink: 0; margin-top: 0.1rem; }

.section-hint { font-size: 0.8rem; color: var(--text-muted); margin: 0 0 1rem; line-height: 1.5; }

.form-grid { display: flex; flex-direction: column; gap: 0.75rem; }
.form-row  { display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap; }
.form-label { font-size: 0.8rem; color: var(--text-tertiary); min-width: 130px; }
.form-hint  { font-size: 0.75rem; color: var(--text-dim); }
.form-input {
  background: var(--bg-overlay); border: 1px solid var(--border-strong);
  border-radius: 0.375rem; padding: 0.38rem 0.625rem;
  font-size: 0.8rem; color: var(--text-primary); font-family: inherit;
  outline: none;
}
.form-input:focus { border-color: var(--accent); }
.form-input--sm { width: 80px; }
.font-mono { font-family: monospace; font-size: 0.75rem; }

.card-footer { display: flex; align-items: center; gap: 1rem; margin-top: 1rem; flex-wrap: wrap; }
.field-error { font-size: 0.78rem; color: var(--danger-light); margin: 0; }

.btn-primary {
  display: inline-flex; align-items: center; gap: 0.4rem;
  background: var(--accent); border: none; border-radius: 0.375rem;
  padding: 0.45rem 1.1rem; font-size: 0.82rem; color: #fff;
  cursor: pointer; transition: background 0.15s; font-family: inherit;
}
.btn-primary:hover:not(:disabled) { background: var(--accent-hover); }
.btn-primary:disabled { opacity: 0.45; cursor: not-allowed; }

.btn-ghost {
  background: none; border: 1px solid var(--border-strong); border-radius: 0.375rem;
  padding: 0.45rem 1rem; font-size: 0.82rem; color: var(--text-muted);
  cursor: pointer; font-family: inherit; display: inline-flex; align-items: center; gap: 0.35rem;
}
.btn-ghost:hover { color: var(--text-primary); border-color: var(--border-hover); }
.btn-ghost--sm { padding: 0.3rem 0.75rem; font-size: 0.78rem; margin-top: 0.5rem; }

/* Share input list */
.shares-input { display: flex; flex-direction: column; gap: 0.5rem; }
.share-input-row { display: flex; align-items: center; gap: 0.5rem; }
.share-idx {
  font-size: 0.72rem; color: var(--text-dim); min-width: 1.2rem;
  text-align: right; flex-shrink: 0;
}
.share-input-row .form-input { flex: 1; }
.usb-btn {
  background: none; border: 1px solid var(--border-strong); border-radius: 0.25rem;
  color: var(--text-dim); padding: 0.28rem; cursor: pointer; display: flex; align-items: center;
  transition: all 0.15s; flex-shrink: 0;
}
.usb-btn:hover { color: var(--accent-light); border-color: var(--accent-soft); }

.icon-btn {
  background: none; border: 1px solid var(--border-strong); border-radius: 0.25rem;
  color: var(--text-muted); padding: 0.25rem; cursor: pointer;
  display: flex; align-items: center; flex-shrink: 0; transition: all 0.15s;
}
.icon-btn--danger:hover { color: var(--danger-light); border-color: #7f1d1d; }

.mt { margin-top: 1rem; }

/* Share output list */
.share-list { margin-top: 1.25rem; display: flex; flex-direction: column; gap: 0.4rem; }
.share-list__label {
  font-size: 0.72rem; font-weight: 600; color: var(--danger-light);
  text-transform: uppercase; letter-spacing: 0.04em; margin: 0 0 0.5rem;
}
.share-item {
  display: flex; align-items: center; gap: 0.5rem;
  background: var(--bg-elevated); border: 1px solid var(--border);
  border-radius: 0.375rem; padding: 0.4rem 0.625rem;
}
.share-item__idx { font-size: 0.7rem; color: var(--text-dim); min-width: 1rem; }
.share-item__val {
  font-family: monospace; font-size: 0.7rem; color: var(--text-secondary);
  word-break: break-all; flex: 1;
}
.share-item__btn {
  background: none; border: none; cursor: pointer; color: var(--text-dim);
  display: flex; padding: 0.15rem; border-radius: 0.2rem; flex-shrink: 0;
  transition: color 0.15s;
}
.share-item__btn:hover { color: var(--text-primary); }
.share-item__btn--usb:hover { color: var(--accent-light); }
</style>
