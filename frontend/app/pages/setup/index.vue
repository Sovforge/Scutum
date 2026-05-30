<template>
  <div class="setup-page">

    <div class="setup-shell">

      <!-- Brand header -->
      <div class="setup-brand">
        <img src="/logo.svg" alt="Scutum" class="setup-brand__logo" />
        <div>
          <h1 class="setup-brand__title">Scutum</h1>
          <p class="setup-brand__sub">Initial setup</p>
        </div>
      </div>

      <!-- Step track -->
      <div class="step-track">
        <template v-for="(s, i) in steps" :key="s.id">
          <div
            class="step-track__node"
            :class="{
              'step-track__node--done':   i < step,
              'step-track__node--active': i === step,
            }"
          >
            <span v-if="i < step" class="step-track__check">
              <Icon name="lucide:check" size="12" />
            </span>
            <span v-else>{{ i + 1 }}</span>
          </div>
          <div v-if="i < steps.length - 1" class="step-track__line" :class="{ 'step-track__line--done': i < step }" />
        </template>
      </div>

      <!-- Card body -->
      <div class="setup-card">

        <!-- ── Step 0: Welcome ─────────────────────────────────────── -->
        <template v-if="currentStepId === 'welcome'">
          <h2 class="setup-card__heading">Welcome to Scutum</h2>
          <p class="setup-card__desc">
            This wizard configures your Scutum control node. It runs once.
            Scutum forms a fully encrypted P2P mesh across your infrastructure — no SaaS control plane, no relays, no telemetry.
          </p>
          <div class="info-note">
            <Icon name="lucide:shield" size="13" />
            <span>Scutum uses <strong>manual peer enrollment only</strong> — nodes are never auto-discovered or auto-trusted.</span>
          </div>
          <div class="setup-card__actions">
            <button class="btn btn--primary" @click="step++">Begin setup →</button>
          </div>
        </template>

        <!-- ── Step 1: Mesh / WireGuard ───────────────────────────── -->
        <template v-else-if="currentStepId === 'mesh'">
          <h2 class="setup-card__heading">Mesh configuration</h2>
          <p class="setup-card__desc">Configure WireGuard for this node. The public key will be shown after setup completes.</p>

          <div class="form-grid">
            <div class="form-row">
              <label class="form-label">Node role</label>
              <select v-model="mesh.installType" class="form-select">
                <option value="hub">Hub — accepts inbound peers, must have a public IP/port</option>
                <option value="remote">Remote — connects outbound to a hub, no public IP needed</option>
                <option value="combined">Combined — acts as a hub and can connect upstream to another hub</option>
              </select>
            </div>

            <div class="form-row">
              <label class="form-label">Mesh IP address <span class="form-label-hint">(CIDR)</span></label>
              <input v-model="mesh.address" class="form-input font-mono"
                :placeholder="mesh.installType === 'remote' ? 'e.g. 10.100.5.2/24 — must be on the hub\'s subnet' : '10.100.5.1/24'" />
              <p v-if="mesh.installType === 'remote'" class="form-hint">
                Choose a unique IP on the same subnet as the hub (e.g. if the hub is <code>10.100.5.1/24</code>, use <code>10.100.5.2/24</code>).
              </p>
            </div>

            <!-- Listen port: required for hub and combined -->
            <template v-if="mesh.installType !== 'remote'">
              <div class="form-row">
                <label class="form-label">WireGuard listen port <span class="form-label-hint">(UDP)</span></label>
                <input v-model.number="mesh.listenPort" type="number" class="form-input font-mono" placeholder="51820" />
              </div>
            </template>

            <!-- Hub peer fields: required for remote, optional for combined -->
            <template v-if="mesh.installType !== 'hub'">
              <div v-if="mesh.installType === 'combined'" class="info-note" style="margin-top:0.25rem">
                <Icon name="lucide:info" size="13" />
                <span>For combined mode, upstream hub connection is <strong>optional</strong>. Leave blank to run as a standalone hub.</span>
              </div>
              <div class="form-row">
                <label class="form-label">
                  Hub endpoint
                  <span class="form-label-hint">(host:port{{ mesh.installType === 'combined' ? ' — optional' : '' }})</span>
                </label>
                <input v-model="mesh.hubEndpoint" class="form-input font-mono" placeholder="1.2.3.4:51820" />
              </div>
              <div class="form-row">
                <label class="form-label">
                  Hub public key
                  <span v-if="mesh.installType === 'combined'" class="form-label-hint">(optional)</span>
                </label>
                <input v-model="mesh.hubPublicKey" class="form-input font-mono" placeholder="Base64-encoded WireGuard public key" />
              </div>
              <div class="form-row">
                <label class="form-label">
                  Hub allowed IPs
                  <span v-if="mesh.installType === 'combined'" class="form-label-hint">(optional)</span>
                </label>
                <input v-model="mesh.hubAllowedIPs" class="form-input font-mono" placeholder="e.g. 10.100.5.0/24" />
                <p class="form-hint">Routes to send through the hub — auto-filled from your mesh IP. Use <code>0.0.0.0/0</code> to route all traffic.</p>
              </div>
              <div class="form-row">
                <label class="form-label">
                  Hub proxy key
                  <span class="form-label-hint">(from hub's Enroll Peer dialog)</span>
                </label>
                <input v-model="mesh.hubHMACKey" class="form-input font-mono" placeholder="hex key shown in the hub's enrollment dialog" />
                <p class="form-hint">Allows this node to accept API requests proxied from the hub.</p>
              </div>
            </template>

            <!-- Advanced: MTU -->
            <div class="form-row">
              <label class="form-label">MTU <span class="form-label-hint">(optional, leave 0 for default 1420)</span></label>
              <input v-model.number="mesh.mtu" type="number" class="form-input font-mono" placeholder="0" min="0" max="9000" />
            </div>
          </div>

          <p v-if="meshError" class="field-error">{{ meshError }}</p>

          <div class="setup-card__actions">
            <button class="btn btn--ghost" @click="step--">Back</button>
            <button class="btn btn--primary" :disabled="submitting" @click="nextMesh">
              <span v-if="submitting" class="btn-spinner" />
              <span v-else>{{ mesh.installType === 'remote' ? 'Connect to hub →' : 'Continue →' }}</span>
            </button>
          </div>
        </template>

        <!-- ── Step 2: Admin account ──────────────────────────────── -->
        <template v-else-if="currentStepId === 'account'">
          <h2 class="setup-card__heading">Create admin account</h2>
          <p class="setup-card__desc">This will be the primary superadmin. Additional accounts can be created after setup.</p>
          <div class="form-grid">
            <div class="form-row">
              <label class="form-label">Username</label>
              <input v-model="admin.username" class="form-input" placeholder="admin" autocomplete="username" />
            </div>
            <div class="form-row">
              <label class="form-label">Password <span class="form-label-hint">(min 12 characters)</span></label>
              <div class="input-wrap">
                <input v-model="admin.password" class="form-input" :type="showPw ? 'text' : 'password'" placeholder="••••••••••••" autocomplete="new-password" />
                <button type="button" class="input-eye" @click="showPw = !showPw" tabindex="-1">
                  <Icon :name="showPw ? 'lucide:eye-off' : 'lucide:eye'" size="13" />
                </button>
              </div>
            </div>
            <div class="form-row">
              <label class="form-label">Confirm password</label>
              <input v-model="admin.confirm" class="form-input" type="password" placeholder="••••••••••••" autocomplete="new-password" />
            </div>
          </div>
          <p v-if="adminError" class="field-error">{{ adminError }}</p>
          <div class="setup-card__actions">
            <button class="btn btn--ghost" @click="step--">Back</button>
            <button class="btn btn--primary" @click="nextAdmin">Continue →</button>
          </div>
        </template>

        <!-- ── Step 3: KMS ────────────────────────────────────────── -->
        <template v-else-if="currentStepId === 'kms'">
          <h2 class="setup-card__heading">Key management</h2>
          <p class="setup-card__desc">
            Scutum encrypts secrets at rest using a master key. Choose where to store that key.
            For most self-hosted deployments <strong>Local</strong> is the right choice.
          </p>

          <div class="form-grid">
            <div class="form-row">
              <label class="form-label">KMS provider</label>
              <select v-model="kms.provider" class="form-select">
                <option value="local">Local — key stored on disk, auto-generated</option>
                <option value="vault">HashiCorp Vault</option>
                <option value="aws">AWS KMS</option>
                <option value="gcp">GCP Cloud KMS</option>
                <option value="azure">Azure Key Vault</option>
              </select>
            </div>

            <!-- Local -->
            <template v-if="kms.provider === 'local'">
              <div class="info-note">
                <Icon name="lucide:info" size="13" />
                <span>A 256-bit master key is generated automatically and stored on disk. The key is split into recovery shares in the next step.</span>
              </div>
            </template>

            <!-- Vault -->
            <template v-else-if="kms.provider === 'vault'">
              <div class="form-row">
                <label class="form-label">Vault address</label>
                <input v-model="kms.vault.addr" class="form-input font-mono" placeholder="https://vault.example.com:8200" />
              </div>
              <div class="form-row">
                <label class="form-label">Transit key name</label>
                <input v-model="kms.vault.keyName" class="form-input font-mono" placeholder="scutum" />
              </div>
              <div class="form-row">
                <label class="form-label">Token file <span class="form-label-hint">(optional — path on server)</span></label>
                <input v-model="kms.vault.tokenFile" class="form-input font-mono" placeholder="/run/secrets/vault-token" />
              </div>
            </template>

            <!-- AWS -->
            <template v-else-if="kms.provider === 'aws'">
              <div class="form-row">
                <label class="form-label">AWS region</label>
                <input v-model="kms.aws.region" class="form-input font-mono" placeholder="us-east-1" />
              </div>
              <div class="form-row">
                <label class="form-label">KMS key ID or ARN</label>
                <input v-model="kms.aws.keyId" class="form-input font-mono" placeholder="arn:aws:kms:us-east-1:123456:key/..." />
              </div>
              <div class="form-row">
                <label class="form-label">Access key ID <span class="form-label-hint">(optional — omit to use instance role)</span></label>
                <input v-model="kms.aws.accessKey" class="form-input font-mono" placeholder="AKIAIOSFODNN7EXAMPLE" />
              </div>
              <div class="form-row">
                <label class="form-label">Secret access key <span class="form-label-hint">(optional)</span></label>
                <input v-model="kms.aws.secretKey" class="form-input font-mono" type="password" placeholder="••••••••••••" />
              </div>
            </template>

            <!-- GCP -->
            <template v-else-if="kms.provider === 'gcp'">
              <div class="form-row">
                <label class="form-label">GCP project ID</label>
                <input v-model="kms.gcp.projectId" class="form-input font-mono" placeholder="my-project-id" />
              </div>
              <div class="form-row">
                <label class="form-label">Location ID <span class="form-label-hint">(optional, defaults to global)</span></label>
                <input v-model="kms.gcp.locationId" class="form-input font-mono" placeholder="global" />
              </div>
              <div class="form-row">
                <label class="form-label">Key ring ID</label>
                <input v-model="kms.gcp.keyRingId" class="form-input font-mono" placeholder="scutum-ring" />
              </div>
              <div class="form-row">
                <label class="form-label">Key ID</label>
                <input v-model="kms.gcp.keyId" class="form-input font-mono" placeholder="master-key" />
              </div>
              <div class="form-row">
                <label class="form-label">Service account token file <span class="form-label-hint">(optional)</span></label>
                <input v-model="kms.gcp.tokenFile" class="form-input font-mono" placeholder="/run/secrets/gcp-sa.json" />
              </div>
            </template>

            <!-- Azure -->
            <template v-else-if="kms.provider === 'azure'">
              <div class="form-row">
                <label class="form-label">Key Vault URL</label>
                <input v-model="kms.azure.vaultUrl" class="form-input font-mono" placeholder="https://myvault.vault.azure.net" />
              </div>
              <div class="form-row">
                <label class="form-label">Key name <span class="form-label-hint">(optional, defaults to scutum)</span></label>
                <input v-model="kms.azure.keyName" class="form-input font-mono" placeholder="scutum" />
              </div>
              <div class="form-row">
                <label class="form-label">Tenant ID</label>
                <input v-model="kms.azure.tenantId" class="form-input font-mono" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" />
              </div>
              <div class="form-row">
                <label class="form-label">Client ID</label>
                <input v-model="kms.azure.clientId" class="form-input font-mono" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" />
              </div>
              <div class="form-row">
                <label class="form-label">Token file <span class="form-label-hint">(optional)</span></label>
                <input v-model="kms.azure.tokenFile" class="form-input font-mono" placeholder="/run/secrets/azure-token" />
              </div>
            </template>
          </div>

          <p v-if="kmsError" class="field-error">{{ kmsError }}</p>
          <div class="setup-card__actions">
            <button class="btn btn--ghost" @click="step--">Back</button>
            <button class="btn btn--primary" @click="nextKMS">Continue →</button>
          </div>
        </template>

        <!-- ── Step 4: Recovery keys ──────────────────────────────── -->
        <template v-else-if="currentStepId === 'recovery'">
          <h2 class="setup-card__heading">Emergency recovery keys</h2>
          <p class="setup-card__desc">
            Scutum uses Shamir's Secret Sharing to split your encryption master key into recovery shares.
            If you ever lose access to the server, you can reconstruct the key using the required number of shares.
          </p>

          <div class="warn-note">
            <Icon name="lucide:triangle-alert" size="14" />
            <span>Shares are shown <strong>once</strong> after setup and cannot be retrieved again. Distribute them to trusted people and store offline.</span>
          </div>

          <div v-if="kms.provider !== 'local'" class="info-note">
            <Icon name="lucide:info" size="13" />
            <span>Recovery shares only apply to the <strong>local</strong> KMS provider. Since you chose <strong>{{ kms.provider }}</strong>, the master key is managed externally — recovery shares will not be generated.</span>
          </div>

          <template v-if="kms.provider === 'local'">
            <div class="form-grid">
              <div class="form-row">
                <label class="form-label">
                  Total shares
                  <span class="form-label-hint">({{ recovery.n }})</span>
                </label>
                <input
                  v-model.number="recovery.n"
                  type="range" min="3" max="10" step="1"
                  class="form-range"
                  @input="clampThreshold"
                />
                <div class="range-labels"><span>3</span><span>10</span></div>
              </div>
              <div class="form-row">
                <label class="form-label">
                  Shares required to recover
                  <span class="form-label-hint">({{ recovery.t }} of {{ recovery.n }})</span>
                </label>
                <input
                  v-model.number="recovery.t"
                  type="range" min="2" :max="recovery.n" step="1"
                  class="form-range"
                />
                <div class="range-labels"><span>2</span><span>{{ recovery.n }}</span></div>
              </div>
            </div>

            <div class="recovery-preview">
              <Icon name="lucide:key-round" size="14" />
              <span>
                <strong>{{ recovery.t }} of {{ recovery.n }}</strong> shares needed to recover access.
                You can lose up to <strong>{{ recovery.n - recovery.t }}</strong> share{{ recovery.n - recovery.t !== 1 ? 's' : '' }} and still recover.
              </span>
            </div>
          </template>

          <p v-if="recoveryError" class="field-error">{{ recoveryError }}</p>

          <!-- Restarting overlay — shown while server installs wireguard-go -->
          <div v-if="restarting" class="restarting-note">
            <span class="restarting-spinner" />
            <div>
              <strong>Installing WireGuard…</strong>
              <p>The server is restarting after installing <code>wireguard-go</code>. Setup will resume automatically.</p>
            </div>
          </div>

          <div v-else class="setup-card__actions">
            <button class="btn btn--ghost" @click="step--">Back</button>
            <button class="btn btn--primary" :disabled="submitting" @click="submit">
              <span v-if="submitting" class="btn-spinner" />
              <span v-else>Finish setup →</span>
            </button>
          </div>
        </template>

        <!-- ── Step 5: Done ───────────────────────────────────────── -->
        <template v-else-if="currentStepId === 'done'">
          <div class="done-icon">
            <Icon name="lucide:check-circle" size="32" />
          </div>
          <h2 class="setup-card__heading">Setup complete</h2>

          <!-- WireGuard unavailable warning -->
          <div v-if="result?.wireguard?.warning" class="warn-note">
            <Icon name="lucide:wifi-off" size="14" />
            <span>
              <strong>WireGuard interface not active.</strong>
              The admin account and keys were saved, but the mesh interface could not be started on this host.
              To activate it, load the kernel module (<code>modprobe wireguard</code>) or install
              <code>wireguard-go</code>, then restart Scutum.
            </span>
          </div>

          <!-- Recovery shares — must be saved before signing in -->
          <template v-if="recoveryShares.length">
            <div class="warn-note warn-note--critical">
              <Icon name="lucide:shield-alert" size="14" />
              <span><strong>Save these recovery shares now.</strong> They will not be shown again. Store each share separately in a secure offline location.</span>
            </div>
            <div class="shares-grid">
              <div v-for="(share, i) in recoveryShares" :key="i" class="share-card">
                <div class="share-card__header">
                  <span class="share-card__label">Share {{ i + 1 }} of {{ recoveryShares.length }}</span>
                  <div class="share-card__actions">
                    <button class="share-card__copy" :class="{ copied: copiedIndex === i }" @click="copyShare(share, i)" title="Copy to clipboard">
                      <Icon :name="copiedIndex === i ? 'lucide:check' : 'lucide:copy'" size="12" />
                    </button>
                    <button class="share-card__usb" @click="exportShareToUsb(share, i + 1, recoveryShares.length)" title="Save to USB / IronKey">
                      <Icon name="lucide:usb" size="12" />
                    </button>
                  </div>
                </div>
                <div class="share-card__value font-mono">{{ share }}</div>
              </div>
            </div>
            <div class="shares-hint">
              You need <strong>{{ recovery.t }} of {{ recoveryShares.length }}</strong> shares to recover access.
            </div>

            <label class="ack-row">
              <input v-model="sharesAcknowledged" type="checkbox" class="ack-checkbox" />
              <span>I have saved all {{ recoveryShares.length }} recovery shares in a secure location.</span>
            </label>
          </template>

          <!-- Summary -->
          <div v-if="result" class="summary-list">
            <div class="summary-row">
              <span class="summary-label">Admin account</span>
              <span class="summary-val">{{ admin.username }}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">Install type</span>
              <span class="summary-val">{{ result.install_type }}</span>
            </div>
            <div class="summary-row">
              <span class="summary-label">KMS provider</span>
              <span class="summary-val">{{ result.kms_provider }}</span>
            </div>
            <div v-if="result.wireguard?.public_key" class="summary-row">
              <span class="summary-label">WireGuard public key</span>
              <span class="summary-val summary-val--key font-mono" style="flex:1">{{ result.wireguard.public_key }}</span>
              <button class="share-card__copy" :class="{ copied: copiedPubkey }" @click="copyPubkey(result.wireguard.public_key)" title="Copy public key">
                <Icon :name="copiedPubkey ? 'lucide:check' : 'lucide:copy'" size="12" />
              </button>
            </div>
            <div v-if="result.wireguard?.address" class="summary-row">
              <span class="summary-label">Mesh IP</span>
              <span class="summary-val font-mono">{{ result.wireguard.address }}</span>
            </div>
            <div v-if="result.wireguard?.listen_port" class="summary-row">
              <span class="summary-label">Listen port</span>
              <span class="summary-val font-mono">{{ result.wireguard.listen_port }}</span>
            </div>
          </div>

          <!-- Remote: no local account — point operator to the hub -->
          <div v-if="result?.install_type === 'remote'" class="info-note">
            <Icon name="lucide:info" size="13" />
            <span>
              This node is running as a <strong>remote peer</strong>. There is no local account to sign in to.
              Visit your <strong>hub's UI</strong> and enroll this node using the WireGuard public key above to complete the mesh connection.
            </span>
          </div>

          <div v-else class="setup-card__actions">
            <NuxtLink
              to="/auth/login"
              class="btn btn--primary"
              :class="{ 'btn--disabled': recoveryShares.length && !sharesAcknowledged }"
              @click.prevent="goToLogin"
            >Sign in →</NuxtLink>
          </div>
        </template>

      </div>

      <!-- Step label -->
      <p class="step-label">{{ steps[step]?.label }}</p>

    </div>
  </div>
</template>

<script setup lang="ts">
import type { SetupRequest, KmsConfig } from '~/composables/useApi'

definePageMeta({ layout: 'auth' })

const api    = useApi()
const router = useRouter()

const wgPubkeyCookie  = useCookie('wg_pubkey',  { maxAge: 60 * 60 * 24 * 365 })
const wgAddressCookie = useCookie('wg_address', { maxAge: 60 * 60 * 24 * 365 })

const step = ref(0)

const hubFlow = [
  { id: 'welcome',  label: 'Welcome' },
  { id: 'mesh',     label: 'Mesh' },
  { id: 'account',  label: 'Admin account' },
  { id: 'kms',      label: 'Key management' },
  { id: 'recovery', label: 'Recovery keys' },
  { id: 'done',     label: 'Done' },
]
const remoteFlow = [
  { id: 'welcome', label: 'Welcome' },
  { id: 'mesh',    label: 'Mesh' },
  { id: 'done',    label: 'Done' },
]


// ── Admin ──────────────────────────────────────────────────────────────────
const admin      = reactive({ username: '', password: '', confirm: '' })
const adminError = ref('')
const showPw     = ref(false)

function nextAdmin() {
  adminError.value = ''
  if (!admin.username || !admin.password) {
    adminError.value = 'Username and password are required.'
    return
  }
  if (admin.password.length < 12) {
    adminError.value = 'Password must be at least 12 characters.'
    return
  }
  if (admin.password !== admin.confirm) {
    adminError.value = 'Passwords do not match.'
    return
  }
  step.value++
}

// ── KMS ────────────────────────────────────────────────────────────────────
const kms = reactive({
  provider: 'local' as KmsConfig['provider'],
  vault:  { addr: '', keyName: '', tokenFile: '' },
  aws:    { region: '', keyId: '', accessKey: '', secretKey: '' },
  gcp:    { projectId: '', locationId: '', keyRingId: '', keyId: '', tokenFile: '' },
  azure:  { vaultUrl: '', keyName: '', tenantId: '', clientId: '', tokenFile: '' },
})
const kmsError = ref('')

function nextKMS() {
  kmsError.value = ''
  switch (kms.provider) {
    case 'local':
      break
    case 'vault':
      if (!kms.vault.addr)    { kmsError.value = 'Vault address is required.'; return }
      if (!kms.vault.keyName) { kmsError.value = 'Transit key name is required.'; return }
      break
    case 'aws':
      if (!kms.aws.region) { kmsError.value = 'AWS region is required.'; return }
      if (!kms.aws.keyId)  { kmsError.value = 'KMS key ID is required.'; return }
      break
    case 'gcp':
      if (!kms.gcp.projectId)  { kmsError.value = 'Project ID is required.'; return }
      if (!kms.gcp.keyRingId)  { kmsError.value = 'Key ring ID is required.'; return }
      if (!kms.gcp.keyId)      { kmsError.value = 'Key ID is required.'; return }
      break
    case 'azure':
      if (!kms.azure.vaultUrl)  { kmsError.value = 'Key Vault URL is required.'; return }
      if (!kms.azure.tenantId)  { kmsError.value = 'Tenant ID is required.'; return }
      if (!kms.azure.clientId)  { kmsError.value = 'Client ID is required.'; return }
      break
  }
  step.value++
}

function buildKmsPayload(): KmsConfig {
  switch (kms.provider) {
    case 'vault':
      return {
        provider: 'vault',
        vault: { addr: kms.vault.addr, key_name: kms.vault.keyName, token_file: kms.vault.tokenFile || undefined },
      }
    case 'aws':
      return {
        provider: 'aws',
        aws: {
          region: kms.aws.region, key_id: kms.aws.keyId,
          access_key: kms.aws.accessKey || undefined,
          secret_key: kms.aws.secretKey || undefined,
        },
      }
    case 'gcp':
      return {
        provider: 'gcp',
        gcp: {
          project_id: kms.gcp.projectId,
          location_id: kms.gcp.locationId || undefined,
          key_ring_id: kms.gcp.keyRingId,
          key_id: kms.gcp.keyId,
          token_file: kms.gcp.tokenFile || undefined,
        },
      }
    case 'azure':
      return {
        provider: 'azure',
        azure: {
          vault_url: kms.azure.vaultUrl,
          key_name: kms.azure.keyName || undefined,
          tenant_id: kms.azure.tenantId,
          client_id: kms.azure.clientId,
          token_file: kms.azure.tokenFile || undefined,
        },
      }
    default:
      return { provider: 'local' }
  }
}

// ── Mesh ───────────────────────────────────────────────────────────────────
function randomMeshAddress(): string {
  const b = Math.floor(Math.random() * 101) + 100 // 100–200
  const c = Math.floor(Math.random() * 254) + 1   // 1–254
  return `10.${b}.${c}.1/24`
}

function networkFromCIDR(cidr: string): string {
  const m = cidr.match(/^(\d+\.\d+\.\d+)\.\d+\/(\d+)$/)
  return m ? `${m[1]}.0/${m[2]}` : ''
}

const mesh = reactive({
  installType:   'hub' as 'hub' | 'remote' | 'combined',
  address:       randomMeshAddress(),
  listenPort:    51820,
  mtu:           0,
  hubEndpoint:   '',
  hubPublicKey:  '',
  hubAllowedIPs: '',
  hubHMACKey:    '',
})

// When role changes, reset address and re-derive allowedIPs
watch(() => mesh.installType, (type) => {
  if (type === 'hub' || type === 'combined') {
    if (!mesh.address || mesh.address === '') mesh.address = randomMeshAddress()
  } else {
    mesh.address = '' // remote must pick their own IP on the hub's subnet
  }
})

// Auto-derive hubAllowedIPs from mesh address (network portion of the /24)
watch(() => mesh.address, (addr) => {
  const net = networkFromCIDR(addr)
  if (net) mesh.hubAllowedIPs = net
})
const meshError = ref('')

const steps        = computed(() => mesh.installType === 'remote' ? remoteFlow : hubFlow)
const currentStepId = computed(() => steps.value[step.value]?.id ?? 'welcome')

function nextMesh() {
  meshError.value = ''
  if (!mesh.address) { meshError.value = 'Mesh IP address is required.'; return }
  if (mesh.installType !== 'remote' && !mesh.listenPort) {
    meshError.value = 'Listen port is required for hub and combined installs.'
    return
  }
  if (mesh.installType === 'remote') {
    if (!mesh.hubEndpoint)   { meshError.value = 'Hub endpoint is required.'; return }
    if (!mesh.hubPublicKey)  { meshError.value = 'Hub public key is required.'; return }
    if (!mesh.hubAllowedIPs) { meshError.value = 'Hub allowed IPs are required.'; return }
    submitRemote()
    return
  }
  // combined: hub fields are optional — no validation required
  step.value++
}

// ── Recovery ───────────────────────────────────────────────────────────────
const recovery = reactive({ n: 5, t: 3 })

function clampThreshold() {
  if (recovery.t > recovery.n) recovery.t = recovery.n
  if (recovery.t < 2) recovery.t = 2
}

// ── Submit ─────────────────────────────────────────────────────────────────
const recoveryError      = ref('')
const submitting         = ref(false)
const restarting         = ref(false)
const result             = ref<{ install_type: string; kms_provider: string; wireguard?: { public_key: string; address?: string; listen_port?: number; warning?: string } } | null>(null)
const recoveryShares     = ref<string[]>([])
const sharesAcknowledged = ref(false)
const copiedIndex        = ref<number | null>(null)

function buildPayload(): SetupRequest {
  return {
    install_type: mesh.installType,
    kms:          buildKmsPayload(),
    wireguard: {
      address:         mesh.address,
      listen_port:     mesh.installType !== 'remote' ? mesh.listenPort : undefined,
      mtu:             mesh.mtu > 0 ? mesh.mtu : undefined,
      hub_endpoint:    mesh.installType !== 'hub' && mesh.hubEndpoint   ? mesh.hubEndpoint   : undefined,
      hub_public_key:  mesh.installType !== 'hub' && mesh.hubPublicKey  ? mesh.hubPublicKey  : undefined,
      hub_allowed_ips: mesh.installType !== 'hub' && mesh.hubAllowedIPs ? mesh.hubAllowedIPs : undefined,
      hub_hmac_key:    mesh.installType !== 'hub' && mesh.hubHMACKey    ? mesh.hubHMACKey    : undefined,
    },
    admin:    { username: admin.username, password: admin.password },
    recovery: kms.provider === 'local' ? { n_shares: recovery.n, threshold: recovery.t } : undefined,
  }
}

async function pollUntilAlive() {
  while (true) {
    await new Promise(r => setTimeout(r, 2000))
    try {
      await api.setupStatus()
      return
    } catch {}
  }
}

async function submitRemote() {
  meshError.value = ''
  submitting.value = true
  try {
    const res = await api.doSetup({
      install_type: 'remote',
      kms:          { provider: 'local' },
      wireguard: {
        address:         mesh.address,
        mtu:             mesh.mtu > 0 ? mesh.mtu : undefined,
        hub_endpoint:    mesh.hubEndpoint   || undefined,
        hub_public_key:  mesh.hubPublicKey  || undefined,
        hub_allowed_ips: mesh.hubAllowedIPs || undefined,
        hub_hmac_key:    mesh.hubHMACKey    || undefined,
      },
      admin: { username: '', password: '' },
    })
    handleSuccess(res)
  } catch (e: any) {
    const raw = e?.data ?? e?.message ?? ''
    meshError.value = typeof raw === 'string' && raw.trim() ? raw.trim() : 'Setup failed. Please try again.'
  } finally {
    submitting.value = false
  }
}

async function submit() {
  recoveryError.value = ''
  submitting.value = true
  try {
    const res = await api.doSetup(buildPayload())

    if (res.status === 'restarting') {
      // wireguard-go was just installed; server is restarting.
      restarting.value = true
      submitting.value = false
      await pollUntilAlive()
      restarting.value = false
      // Retry the same payload now that WireGuard is available.
      submitting.value = true
      const retry = await api.doSetup(buildPayload())
      handleSuccess(retry)
      return
    }

    handleSuccess(res)
  } catch (e: any) {
    const raw = e?.data ?? e?.message ?? ''
    recoveryError.value = typeof raw === 'string' && raw.trim()
      ? raw.trim()
      : 'Setup failed. Please try again.'
  } finally {
    submitting.value = false
    restarting.value = false
  }
}

function handleSuccess(res: Awaited<ReturnType<typeof api.doSetup>>) {
  result.value = res as any
  recoveryShares.value = res.recovery_shares ?? []
  if (res.wireguard) {
    wgPubkeyCookie.value  = res.wireguard.public_key
    wgAddressCookie.value = res.wireguard.address ?? ''
  }
  // Tell the global auth middleware setup is now done so /auth/login navigation works.
  const setupDone = useState<boolean | null>('setup-complete')
  setupDone.value = true
  step.value++
}

async function copyShare(share: string, index: number) {
  try {
    await navigator.clipboard.writeText(share)
    copiedIndex.value = index
    setTimeout(() => { copiedIndex.value = null }, 1800)
  } catch {}
}

async function exportShareToUsb(share: string, index: number, total: number) {
  const filename = `scutum-erk-share-${index}-of-${total}.erk`
  if (typeof window !== 'undefined' && 'showSaveFilePicker' in window) {
    try {
      const handle = await (window as any).showSaveFilePicker({
        suggestedName: filename,
        types: [{ description: 'Scutum ERK Share', accept: { 'text/plain': ['.erk'] } }],
      })
      const writable = await handle.createWritable()
      await writable.write(share)
      await writable.close()
    } catch (e: any) { if (e.name !== 'AbortError') throw e }
  } else {
    const a = document.createElement('a')
    a.href = 'data:text/plain;charset=utf-8,' + encodeURIComponent(share)
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
  }
}

const copiedPubkey = ref(false)
async function copyPubkey(key: string) {
  try {
    await navigator.clipboard.writeText(key)
    copiedPubkey.value = true
    setTimeout(() => { copiedPubkey.value = false }, 1800)
  } catch {}
}

function goToLogin() {
  if (recoveryShares.value.length && !sharesAcknowledged.value) return
  router.push('/auth/login')
}
</script>

<style scoped>
.setup-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-base);
  background-image:
    radial-gradient(ellipse at 20% 50%, var(--accent-dim) 0%, transparent 60%),
    radial-gradient(ellipse at 80% 20%, var(--accent-dim) 0%, transparent 50%);
  padding: 2rem 1rem;
}

.setup-shell {
  width: 100%;
  max-width: 560px;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.setup-brand { display: flex; align-items: center; gap: 0.875rem; }
.setup-brand__logo { width: 44px; height: 44px; object-fit: contain; }
.setup-brand__title { margin: 0; font-size: 1.3rem; font-weight: 800; color: var(--text-primary); letter-spacing: -0.02em; }
.setup-brand__sub   { margin: 0; font-size: 0.72rem; color: var(--text-dim); }

.step-track { display: flex; align-items: center; gap: 0; }
.step-track__node {
  width: 26px; height: 26px; border-radius: 50%;
  border: 1px solid var(--border-strong); background: var(--bg-elevated);
  color: var(--text-dim); font-size: 0.72rem; font-weight: 600;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
  transition: border-color 0.2s, background 0.2s, color 0.2s;
}
.step-track__node--active { border-color: var(--accent); background: var(--accent-subtle); color: var(--accent-light); }
.step-track__node--done   { border-color: var(--success); background: var(--success-dim); color: var(--success-light); }
.step-track__check { display: flex; align-items: center; }
.step-track__line { flex: 1; height: 1px; background: var(--border-strong); transition: background 0.2s; }
.step-track__line--done { background: var(--success-dim); }

.setup-card {
  background: var(--bg-surface); border: 1px solid var(--border);
  border-radius: 0.75rem; padding: 2rem;
  display: flex; flex-direction: column; gap: 1.25rem;
}
.setup-card__heading { margin: 0; font-size: 1.1rem; font-weight: 700; color: var(--text-primary); }
.setup-card__desc    { margin: 0; font-size: 0.82rem; color: var(--text-muted); line-height: 1.6; }
.setup-card__actions { display: flex; justify-content: flex-end; gap: 0.625rem; margin-top: 0.5rem; }

.step-label { margin: 0; text-align: center; font-size: 0.72rem; color: var(--text-subtle); }

.form-grid  { display: flex; flex-direction: column; gap: 0.875rem; }
.form-row   { display: flex; flex-direction: column; gap: 0.35rem; }
.form-label { font-size: 0.78rem; color: var(--text-tertiary); }
.form-label-hint { color: var(--text-subtle); font-weight: 400; }
.form-hint { margin: 0.25rem 0 0; font-size: 0.72rem; color: var(--text-dim); line-height: 1.5; }
.form-hint code { font-family: monospace; color: var(--accent-light); }
.form-input, .form-select {
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 0.375rem; padding: 0.55rem 0.8rem;
  font-size: 0.82rem; color: var(--text-primary); font-family: inherit;
  outline: none; transition: border-color 0.15s; width: 100%; box-sizing: border-box;
}
.form-input:focus, .form-select:focus { border-color: var(--accent); }
.form-input::placeholder { color: var(--text-subtle); }
.font-mono { font-family: monospace; }

.form-range {
  width: 100%; accent-color: var(--accent);
  cursor: pointer;
}
.range-labels {
  display: flex; justify-content: space-between;
  font-size: 0.7rem; color: var(--text-subtle); padding: 0 2px;
}

.input-wrap { position: relative; }
.input-wrap .form-input { padding-right: 2.25rem; }
.input-eye {
  position: absolute; right: 0.5rem; top: 50%; transform: translateY(-50%);
  background: none; border: none; color: var(--text-dim); cursor: pointer; padding: 0.2rem; display: flex;
}
.input-eye:hover { color: var(--text-tertiary); }

.info-note, .warn-note {
  display: flex; align-items: flex-start; gap: 0.5rem;
  font-size: 0.78rem; color: var(--text-dim);
  background: var(--border-subtle); border-radius: 0.375rem;
  padding: 0.625rem 0.875rem; line-height: 1.5;
}
.warn-note {
  background: color-mix(in srgb, var(--warning, #92400e) 8%, transparent);
  border: 1px solid color-mix(in srgb, var(--warning, #92400e) 25%, transparent);
  color: var(--warning-text, #d97706);
}
.warn-note--critical {
  background: var(--danger-bg);
  border-color: #7f1d1d55;
  color: var(--danger-lighter);
}
.info-note strong, .warn-note strong { color: inherit; }

.recovery-preview {
  display: flex; align-items: flex-start; gap: 0.5rem;
  font-size: 0.82rem; color: var(--text-muted);
  background: var(--accent-subtle); border: 1px solid var(--accent-dim);
  border-radius: 0.375rem; padding: 0.625rem 0.875rem; line-height: 1.5;
}
.recovery-preview strong { color: var(--accent-light); }

/* ── Shares ─────────────────────────────────────────────────────────────── */
.shares-grid {
  display: flex; flex-direction: column; gap: 0.5rem;
}
.share-card {
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 0.5rem; padding: 0.625rem 0.75rem;
}
.share-card__header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 0.35rem;
}
.share-card__label { font-size: 0.68rem; color: var(--text-dim); font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; }
.share-card__actions { display: flex; align-items: center; gap: 0.25rem; }
.share-card__copy, .share-card__usb {
  background: none; border: none; cursor: pointer; color: var(--text-muted);
  display: flex; padding: 0.15rem; border-radius: 0.2rem;
  transition: color 0.15s, background 0.15s;
}
.share-card__copy:hover, .share-card__usb:hover { color: var(--text-primary); background: var(--hover-bg); }
.share-card__copy.copied { color: var(--success-light); }
.share-card__usb { color: var(--text-dim); }
.share-card__usb:hover { color: var(--accent-light); }
.share-card__value {
  font-size: 0.7rem; color: var(--text-secondary);
  word-break: break-all; line-height: 1.5;
}

.shares-hint {
  font-size: 0.78rem; color: var(--text-dim); text-align: center;
}
.shares-hint strong { color: var(--text-muted); }

.ack-row {
  display: flex; align-items: flex-start; gap: 0.6rem;
  font-size: 0.8rem; color: var(--text-muted); cursor: pointer;
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 0.375rem; padding: 0.75rem 0.875rem;
}
.ack-checkbox { margin-top: 1px; accent-color: var(--accent); cursor: pointer; }

/* ── Done summary ───────────────────────────────────────────────────────── */
.done-icon { display: flex; justify-content: center; color: var(--success-light); filter: drop-shadow(0 0 12px var(--success-glow)); }

.summary-list { display: flex; flex-direction: column; gap: 0; border: 1px solid var(--border); border-radius: 0.5rem; overflow: hidden; }
.summary-row  { display: flex; justify-content: space-between; align-items: baseline; padding: 0.55rem 0.875rem; border-bottom: 1px solid var(--border); gap: 1rem; }
.summary-row:last-child { border-bottom: none; }
.summary-label { font-size: 0.75rem; color: var(--text-dim); flex-shrink: 0; }
.summary-val   { font-size: 0.8rem; color: var(--text-secondary); text-align: right; }
.summary-val--key { font-size: 0.7rem; word-break: break-all; }

/* ── Buttons ────────────────────────────────────────────────────────────── */
.btn {
  display: inline-flex; align-items: center; gap: 0.4rem;
  border-radius: 0.375rem; padding: 0.55rem 1rem;
  font-size: 0.82rem; font-weight: 600; font-family: inherit;
  cursor: pointer; text-decoration: none; border: none;
  transition: background 0.15s, color 0.15s, opacity 0.15s;
}
.btn--primary { background: var(--accent); color: #fff; }
.btn--primary:hover:not(:disabled):not(.btn--disabled) { background: var(--accent-hover); }
.btn--primary:disabled, .btn--disabled { opacity: 0.4; cursor: not-allowed; pointer-events: none; }
.btn--ghost { background: none; border: 1px solid var(--border-strong); color: var(--text-tertiary); }
.btn--ghost:hover { background: var(--border); color: var(--text-primary); }

.btn-spinner {
  width: 14px; height: 14px; border-radius: 50%;
  border: 2px solid rgba(255,255,255,0.3); border-top-color: #fff;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.field-error { font-size: 0.78rem; color: var(--danger-lighter); margin: 0; }

/* ── Restarting state ───────────────────────────────────────────────────── */
.restarting-note {
  display: flex; align-items: flex-start; gap: 0.875rem;
  background: var(--accent-subtle); border: 1px solid var(--accent-dim);
  border-radius: 0.5rem; padding: 0.875rem 1rem;
  font-size: 0.82rem; color: var(--text-muted); line-height: 1.5;
}
.restarting-note strong { color: var(--accent-light); display: block; margin-bottom: 0.2rem; }
.restarting-note p { margin: 0; }
.restarting-note code { font-family: monospace; font-size: 0.8rem; color: var(--accent-light); }
.restarting-spinner {
  flex-shrink: 0; margin-top: 2px;
  width: 18px; height: 18px; border-radius: 50%;
  border: 2px solid var(--accent-dim); border-top-color: var(--accent-light);
  animation: spin 0.9s linear infinite;
}
</style>
