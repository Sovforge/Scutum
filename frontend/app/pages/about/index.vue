<template>
  <div class="about">

    <!-- Hero -->
    <div class="hero">
      <div class="hero__brand">
        <img src="/logo.svg" alt="Scutum" class="hero__logo" />
        <div>
          <h1 class="hero__name">Scutum</h1>
          <p class="hero__tagline">Sovereign orchestration for people who build their own infrastructure — not rent it.</p>
        </div>
      </div>
      <div class="hero__badges">
        <span class="badge badge--version">{{ versionStr }}</span>
        <span class="badge badge--build">build {{ buildStr }}</span>
        <span class="badge badge--commit">{{ commitStr }}</span>
      </div>
    </div>

    <!-- Body grid -->
    <div class="about__grid">

      <!-- Principles -->
      <UiCard title="Core Principles">
        <div class="principles">
          <div v-for="p in principles" :key="p.title" class="principle">
            <div class="principle__icon">
              <Icon :name="p.icon" size="16" />
            </div>
            <div>
              <h3 class="principle__title">{{ p.title }}</h3>
              <p class="principle__desc">{{ p.desc }}</p>
            </div>
          </div>
        </div>
      </UiCard>

      <!-- Tech stack -->
      <UiCard title="Tech Stack">
        <dl class="stack-list">
          <div v-for="s in stack" :key="s.label" class="stack-row">
            <dt class="stack-label">{{ s.label }}</dt>
            <dd class="stack-val">{{ s.value }}</dd>
          </div>
        </dl>
      </UiCard>

      <!-- API reference -->
      <UiCard title="API Quick Reference" class="about__api">
        <table class="api-table">
          <thead>
            <tr>
              <th>Method</th>
              <th>Endpoint</th>
              <th>Description</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="ep in endpoints" :key="ep.path">
              <td><span class="method-badge" :class="`method-badge--${ep.method.toLowerCase()}`">{{ ep.method }}</span></td>
              <td class="ep-path">{{ ep.path }}</td>
              <td class="ep-desc">{{ ep.desc }}</td>
            </tr>
          </tbody>
        </table>
      </UiCard>

      <!-- Deployment -->
      <UiCard title="Deployment">
        <div class="deploy-tabs">
          <button
            v-for="t in deployTabs"
            :key="t.id"
            class="deploy-tab"
            :class="{ 'deploy-tab--active': activeTab === t.id }"
            @click="activeTab = t.id"
          >{{ t.label }}</button>
        </div>

        <template v-if="activeTab === 'docker'">
          <p class="deploy-note">Pull and run the unified image — everything is included.</p>
          <div class="code-block">
            <button class="code-block__copy" @click="copy(dockerCmd)" :title="copied === 'docker' ? 'Copied!' : 'Copy'">
              <Icon :name="copied === 'docker' ? 'lucide:check' : 'lucide:copy'" size="13" />
            </button>
            <pre class="code-block__pre">{{ dockerCmd }}</pre>
          </div>
        </template>

        <template v-if="activeTab === 'sbom'">
          <p class="deploy-note">Every image ships with a full Software Bill of Materials for auditability.</p>
          <div class="code-block">
            <button class="code-block__copy" @click="copy(sbomCmd)" :title="copied === 'sbom' ? 'Copied!' : 'Copy'">
              <Icon :name="copied === 'sbom' ? 'lucide:check' : 'lucide:copy'" size="13" />
            </button>
            <pre class="code-block__pre">{{ sbomCmd }}</pre>
          </div>
        </template>
      </UiCard>

    </div>

    <!-- Footer strip -->
    <div class="about__footer">
      <span class="about__footer-text">Secure · Peer-to-peer · Zero cloud dependencies · Open-core</span>
      <span class="about__footer-text">Free for personal &amp; non-commercial use · No artificial node limits</span>
    </div>

  </div>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()
const versionStr = ref('…')
const buildStr   = ref('…')
const commitStr  = ref('…')

onMounted(async () => {
  const v = await api.getVersion()
  versionStr.value = v.version
  buildStr.value   = v.build
  commitStr.value  = v.commit
})

const principles = [
  {
    icon: 'lucide:shield',
    title: 'Absolute Sovereignty',
    desc: 'No cloud control plane. No relays. No vendor lock-in. Your nodes talk directly to each other — always.',
  },
  {
    icon: 'lucide:box',
    title: 'Container-First',
    desc: 'Designed for immutable OSes (Alpine, Talos-style, ZimaOS). Everything lives inside one image.',
  },
  {
    icon: 'lucide:network',
    title: 'Universal Networking',
    desc: 'Kernel WireGuard when available. Automatic userspace fallback when containers are restricted.',
  },
  {
    icon: 'lucide:unplug',
    title: 'Zero External Dependencies',
    desc: 'No external services required. No databases needed unless you want them.',
  },
]

const stack = [
  { label: 'Runtime',       value: 'Go 1.24+ (static binary)' },
  { label: 'Networking',    value: 'WireGuard — kernel + userspace fallback' },
  { label: 'State storage', value: 'Encrypted SQLite (default) · PostgreSQL · MySQL' },
  { label: 'Observability', value: 'OpenTelemetry — logs, metrics, distributed traces' },
  { label: 'Runtimes',      value: 'Docker · Kubernetes' },
  { label: 'GitOps',        value: 'Git-native manifest reconciliation' },
  { label: 'Storage',       value: 'S3-compatible (MinIO, R2, AWS S3, Backblaze B2, Ceph)' },
  { label: 'Distribution',  value: 'GHCR — includes SBOM on every image' },
]

const endpoints = [
  { method: 'GET',    path: '/health',                       desc: 'System heartbeat and uptime' },
  { method: 'GET',    path: '/version',                      desc: 'Version, build hash, and commit ref' },
  { method: 'POST',   path: '/auth/login',                   desc: 'Authenticate and receive a JWT' },
  { method: 'GET',    path: '/docker/containers',            desc: 'List all containers with state' },
  { method: 'POST',   path: '/docker/deploy-compose',        desc: 'Deploy a Docker Compose stack' },
  { method: 'GET',    path: '/docker/containers/{id}/stats', desc: 'Live CPU / memory stats for a container' },
  { method: 'GET',    path: '/kubernetes/summary',           desc: 'Cluster pod, deployment, and node counts' },
  { method: 'POST',   path: '/kubernetes/apply',             desc: 'Apply a Kubernetes YAML manifest' },
  { method: 'POST',   path: '/network/peer',                 desc: 'Enroll a node into the WireGuard mesh' },
  { method: 'GET',    path: '/network/mesh-summary',         desc: 'Healthy vs. total peer count' },
  { method: 'GET',    path: '/nodes',                        desc: 'List registered mesh nodes' },
  { method: 'GET',    path: '/audit/logs',                   desc: 'Paginated audit event log' },
  { method: 'GET',    path: '/admin/export',                 desc: 'Full database export as JSON' },
  { method: 'GET',    path: '/storage/backends',             desc: 'List S3-compatible storage backends' },
  { method: 'DELETE', path: '/plugins/{name}',               desc: 'Unload a running plugin' },
]

const deployTabs = [
  { id: 'docker', label: 'Docker' },
  { id: 'sbom',   label: 'SBOM / Audit' },
]
const activeTab = ref('docker')
const copied    = ref('')

const dockerCmd = `docker pull ghcr.io/your-username/scutum:latest

docker run -d \\
  --name scutum \\
  --restart unless-stopped \\
  -v /app/data:/app/data \\
  -v /app/secrets:/app/secrets \\
  -p 8080:8080 \\
  ghcr.io/andreas14101/scutum:latest`


const sbomCmd = `# Inspect the SBOM with Syft
syft ghcr.io/andreas14101/scutum:latest

# Export to CycloneDX JSON
syft ghcr.io/andreas14101/scutum:latest \\
  -o cyclonedx-json > scutum-sbom.json`

async function copy(text: string) {
  const key = text === dockerCmd ? 'docker' : 'sbom'
  await navigator.clipboard.writeText(text)
  copied.value = key
  setTimeout(() => { copied.value = '' }, 2000)
}
</script>

<style scoped>
.about {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

/* ── Hero ──────────────────────────────────────────────────────────────── */
.hero {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  padding: 1.75rem 2rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1.5rem;
  flex-wrap: wrap;
  background-image:
    radial-gradient(ellipse at 90% 50%, var(--accent-dim) 0%, transparent 55%);
}
.hero__brand {
  display: flex;
  align-items: center;
  gap: 1rem;
}
.hero__logo {
  width: 52px;
  height: 52px;
  object-fit: contain;
  filter: drop-shadow(0 0 16px var(--accent-soft));
}
.hero__name {
  margin: 0 0 0.2rem;
  font-size: 1.5rem;
  font-weight: 800;
  color: var(--text-primary);
  letter-spacing: -0.03em;
}
.hero__tagline {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-muted);
  max-width: 420px;
  line-height: 1.5;
}
.hero__badges {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.badge {
  display: inline-flex;
  padding: 0.2rem 0.6rem;
  border-radius: 9999px;
  font-size: 0.7rem;
  font-weight: 600;
  font-family: monospace;
  border: 1px solid;
}
.badge--version { background: var(--accent-subtle); border-color: var(--accent-soft); color: var(--accent-light); }
.badge--build   { background: var(--bg-elevated);   border-color: var(--border-strong); color: var(--text-tertiary); }
.badge--commit  { background: var(--bg-elevated);   border-color: var(--border-strong); color: var(--text-dim); }

/* ── Grid ──────────────────────────────────────────────────────────────── */
.about__grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.25rem;
}
.about__api { grid-column: 1 / -1; }

/* ── Principles ────────────────────────────────────────────────────────── */
.principles {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.principle {
  display: flex;
  gap: 0.875rem;
  align-items: flex-start;
}
.principle__icon {
  width: 32px;
  height: 32px;
  border-radius: 0.5rem;
  background: var(--accent-subtle);
  border: 1px solid var(--accent-soft);
  color: var(--accent-light);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.principle__title {
  margin: 0 0 0.2rem;
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-primary);
}
.principle__desc {
  margin: 0;
  font-size: 0.78rem;
  color: var(--text-muted);
  line-height: 1.55;
}

/* ── Tech stack ────────────────────────────────────────────────────────── */
.stack-list {
  margin: 0;
  display: flex;
  flex-direction: column;
}
.stack-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  padding: 0.45rem 0;
  border-bottom: 1px solid var(--border-faint);
  gap: 1rem;
}
.stack-row:last-child { border-bottom: none; }
.stack-label {
  font-size: 0.72rem;
  color: var(--text-dim);
  white-space: nowrap;
  flex-shrink: 0;
}
.stack-val {
  margin: 0;
  font-size: 0.78rem;
  color: var(--text-secondary);
  text-align: right;
}

/* ── API table ─────────────────────────────────────────────────────────── */
.api-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8rem;
}
.api-table th {
  text-align: left;
  padding: 0 0.75rem 0.625rem;
  color: var(--text-dim);
  font-weight: 500;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid var(--border);
}
.api-table td { padding: 0.55rem 0.75rem; border-bottom: 1px solid var(--border-faint); vertical-align: middle; }
.api-table tbody tr:last-child td { border-bottom: none; }
.api-table tbody tr:hover { background: var(--hover-subtle); }

.method-badge {
  display: inline-flex;
  padding: 0.1rem 0.4rem;
  border-radius: 0.2rem;
  font-size: 0.68rem;
  font-weight: 700;
  font-family: monospace;
}
.method-badge--get  { background: rgba(34, 197, 94, 0.12);  color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.25); }
.method-badge--post { background: rgba(124, 58, 237, 0.12); color: var(--accent-light); border: 1px solid rgba(124, 58, 237, 0.25); }

.ep-path { font-family: monospace; font-size: 0.75rem; color: var(--text-tertiary); }
.ep-desc { color: var(--text-muted); }

/* ── Deployment ────────────────────────────────────────────────────────── */
.deploy-tabs {
  display: flex;
  gap: 0.25rem;
  margin-bottom: 1rem;
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.5rem;
  padding: 0.25rem;
  width: fit-content;
}
.deploy-tab {
  padding: 0.3rem 0.75rem;
  border-radius: 0.35rem;
  border: none;
  background: none;
  font-size: 0.78rem;
  font-family: inherit;
  color: var(--text-muted);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.deploy-tab--active { background: var(--bg-surface); color: var(--text-primary); box-shadow: 0 1px 3px var(--shadow); }
.deploy-tab:hover:not(.deploy-tab--active) { color: var(--text-primary); }
.deploy-note { font-size: 0.78rem; color: var(--text-muted); margin: 0 0 0.75rem; }

.code-block {
  position: relative;
  background: var(--bg-base);
  border: 1px solid var(--border-strong);
  border-radius: 0.5rem;
  overflow: hidden;
}
.code-block__pre {
  margin: 0;
  padding: 1rem 1.25rem;
  font-size: 0.75rem;
  font-family: monospace;
  color: var(--text-tertiary);
  white-space: pre;
  overflow-x: auto;
  line-height: 1.65;
}
.code-block__copy {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  background: var(--bg-surface);
  border: 1px solid var(--border-strong);
  border-radius: 0.3rem;
  color: var(--text-dim);
  cursor: pointer;
  padding: 0.25rem;
  display: flex;
  transition: color 0.15s, background 0.15s;
}
.code-block__copy:hover { color: var(--text-primary); background: var(--hover-bg); }

/* ── Footer ────────────────────────────────────────────────────────────── */
.about__footer {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.5rem;
  padding: 0.75rem 0;
  border-top: 1px solid var(--border-faint);
}
.about__footer-text {
  font-size: 0.7rem;
  color: var(--text-subtle);
}
</style>
