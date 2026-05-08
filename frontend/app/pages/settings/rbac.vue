<template>
  <div class="settings-page">
    <div class="settings-layout">

      <!-- Left nav -->
      <nav class="settings-nav">
        <NuxtLink to="/settings" class="settings-nav__item" exact-active-class="settings-nav__item--active">
          <Icon name="lucide:sliders-horizontal" size="15" /> General
        </NuxtLink>
        <div class="settings-nav__divider" />
        <NuxtLink to="/settings/rbac"    class="settings-nav__item" active-class="settings-nav__item--active">
          <Icon name="lucide:shield" size="15" /> RBAC
        </NuxtLink>
        <NuxtLink to="/settings/secrets" class="settings-nav__item" active-class="settings-nav__item--active">
          <Icon name="lucide:key-round" size="15" /> Secrets / KMS
        </NuxtLink>
      </nav>

      <div class="settings-content">

        <!-- ── Roles ───────────────────────────────────────────────────── -->
        <UiCard title="Roles">
          <template #header-right>
            <button class="action-btn action-btn--primary" @click="openNewRole">
              <Icon name="lucide:plus" size="13" /> New Role
            </button>
          </template>
          <div v-if="rolesError" class="api-error">{{ rolesError }}</div>
          <div v-else-if="rolesLoading" class="loading-row">Loading…</div>
          <table v-else class="data-table">
            <thead>
              <tr>
                <th>Role</th>
                <th>Description</th>
                <th>Permissions</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="role in roles" :key="role.id" class="table-row">
                <td class="cell--name">
                  <span class="role-badge" :class="`role-badge--${roleColor(role.name)}`">{{ role.name }}</span>
                </td>
                <td class="cell--muted">{{ role.description || '—' }}</td>
                <td>
                  <div class="perm-chips">
                    <span v-for="p in role.perms" :key="p" class="perm-chip">{{ p }}</span>
                    <span v-if="!role.perms?.length" class="cell--muted" style="font-size:0.75rem">—</span>
                  </div>
                </td>
                <td class="cell--actions">
                  <template v-if="pendingDeleteRole === role.id">
                    <span class="delete-confirm-label">Delete?</span>
                    <button class="icon-btn" @click="pendingDeleteRole = null">Cancel</button>
                    <button class="icon-btn icon-btn--danger" @click="confirmDeleteRole(role.id)">Confirm</button>
                  </template>
                  <template v-else>
                    <button class="icon-btn" title="Edit" @click="openEditRole(role)">
                      <Icon name="lucide:pencil" size="13" />
                    </button>
                    <button class="icon-btn icon-btn--danger" title="Delete" @click="pendingDeleteRole = role.id">
                      <Icon name="lucide:trash-2" size="13" />
                    </button>
                  </template>
                </td>
              </tr>
              <tr v-if="!roles.length">
                <td colspan="4" class="empty-row">No roles defined yet.</td>
              </tr>
            </tbody>
          </table>
        </UiCard>

        <!-- ── Users ───────────────────────────────────────────────────── -->
        <UiCard title="Users">
          <template #header-right>
            <button class="action-btn action-btn--primary" @click="openNewUser">
              <Icon name="lucide:user-plus" size="13" /> Create User
            </button>
          </template>
          <div v-if="usersError" class="api-error">{{ usersError }}</div>
          <div v-else-if="usersLoading" class="loading-row">Loading…</div>
          <table v-else class="data-table">
            <thead>
              <tr>
                <th>Username</th>
                <th>Roles</th>
                <th>Created</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="u in users" :key="u.id" class="table-row">
                <td class="cell--name">
                  <div class="avatar">{{ initials(u.username) }}</div>
                  {{ u.username }}
                </td>
                <td>
                  <div class="perm-chips">
                    <span v-for="r in (u.roles ?? [])" :key="r" class="role-badge" :class="`role-badge--${roleColor(r)}`">{{ r }}</span>
                    <span v-if="!u.roles?.length" class="cell--muted" style="font-size:0.75rem">—</span>
                  </div>
                </td>
                <td class="cell--muted">{{ formatDate(u.created_at) }}</td>
                <td class="cell--actions">
                  <template v-if="pendingDeleteUser === u.id">
                    <span class="delete-confirm-label">Delete?</span>
                    <button class="icon-btn" @click="pendingDeleteUser = null">Cancel</button>
                    <button class="icon-btn icon-btn--danger" @click="confirmDeleteUser(u.id)">Confirm</button>
                  </template>
                  <template v-else>
                    <button class="icon-btn" title="Edit" @click="openEditUser(u)">
                      <Icon name="lucide:pencil" size="13" />
                    </button>
                    <button class="icon-btn icon-btn--danger" title="Delete user" @click="pendingDeleteUser = u.id">
                      <Icon name="lucide:user-x" size="13" />
                    </button>
                  </template>
                </td>
              </tr>
              <tr v-if="!users.length">
                <td colspan="4" class="empty-row">No users found.</td>
              </tr>
            </tbody>
          </table>
        </UiCard>

      </div>
    </div>
  </div>

  <!-- ── Modals ──────────────────────────────────────────────────────────── -->
  <Teleport to="body">
    <Transition name="modal">

      <!-- Create / Edit User -->
      <div v-if="activeModal === 'user'" class="modal-backdrop" @click.self="closeModal">
        <div class="modal">
          <div class="modal__header">
            <h3 class="modal__title">{{ editingUser._new ? 'Create User' : `Edit: ${editingUser.username}` }}</h3>
            <button class="modal__close" @click="closeModal"><Icon name="lucide:x" size="16" /></button>
          </div>

          <div class="modal__body">
            <div class="form-grid">
              <div class="form-row">
                <label class="form-label">Username</label>
                <input v-model="editingUser.username" class="form-input" placeholder="alice" :disabled="!editingUser._new" />
              </div>

              <div class="form-row">
                <label class="form-label">
                  {{ editingUser._new ? 'Password' : 'New password' }}
                  <span v-if="!editingUser._new" class="form-label-hint">(leave blank to keep unchanged)</span>
                </label>
                <div class="input-wrap">
                  <input v-model="editingUser.password" class="form-input" :type="showPw ? 'text' : 'password'" placeholder="••••••••" />
                  <button type="button" class="input-eye" @click="showPw = !showPw" tabindex="-1">
                    <Icon :name="showPw ? 'lucide:eye-off' : 'lucide:eye'" size="13" />
                  </button>
                </div>
              </div>

              <div class="form-row">
                <label class="form-label">Roles</label>
                <div class="role-checkbox-list">
                  <label v-for="r in roles" :key="r.id" class="role-checkbox">
                    <input type="checkbox" :value="r.name" v-model="editingUser.selectedRoles" />
                    <span class="role-badge" :class="`role-badge--${roleColor(r.name)}`">{{ r.name }}</span>
                    <span class="cell--muted" style="font-size:0.75rem">{{ r.description }}</span>
                  </label>
                </div>
              </div>
            </div>
            <p v-if="modalError" class="form-error">{{ modalError }}</p>
          </div>

          <div class="modal__footer">
            <button class="action-btn action-btn--ghost" @click="closeModal">Cancel</button>
            <button class="action-btn action-btn--primary" :disabled="modalSaving" @click="saveUser">
              {{ modalSaving ? 'Saving…' : editingUser._new ? 'Create user' : 'Save changes' }}
            </button>
          </div>
        </div>
      </div>

      <!-- Create / Edit Role -->
      <div v-else-if="activeModal === 'role'" class="modal-backdrop" @click.self="closeModal">
        <div class="modal modal--wide">
          <div class="modal__header">
            <h3 class="modal__title">{{ editingRole._new ? 'New Role' : `Edit role: ${editingRole._origName}` }}</h3>
            <button class="modal__close" @click="closeModal"><Icon name="lucide:x" size="16" /></button>
          </div>

          <div class="modal__body">
            <div class="form-grid">
              <div class="form-row">
                <label class="form-label">Role name</label>
                <input v-model="editingRole.name" class="form-input" placeholder="e.g. developer" />
              </div>
              <div class="form-row">
                <label class="form-label">Description</label>
                <input v-model="editingRole.description" class="form-input" placeholder="Short description of this role's purpose" />
              </div>

              <!-- Permission matrix -->
              <div class="section-divider"><span>Permissions</span></div>
              <div class="perm-matrix">
                <div class="perm-matrix__head">
                  <span>Resource</span>
                  <span>None</span>
                  <span>Read</span>
                  <span>Write</span>
                  <span>Admin (*)</span>
                </div>
                <div v-for="res in permResources" :key="res" class="perm-matrix__row">
                  <span class="perm-matrix__res">{{ res }}</span>
                  <label v-for="lvl in permLevels" :key="lvl" class="perm-radio">
                    <input type="radio" :name="`perm-${res}`" :value="lvl" v-model="editingRole.permMatrix[res]" />
                  </label>
                </div>
              </div>

              <!-- Preview -->
              <div class="perm-preview">
                <span class="form-label">Generated permissions</span>
                <div class="perm-chips mt-xs">
                  <span v-for="p in previewPerms" :key="p" class="perm-chip">{{ p }}</span>
                  <span v-if="!previewPerms.length" class="form-label">— none —</span>
                </div>
              </div>
            </div>
            <p v-if="modalError" class="form-error">{{ modalError }}</p>
          </div>

          <div class="modal__footer">
            <button class="action-btn action-btn--ghost" @click="closeModal">Cancel</button>
            <button class="action-btn action-btn--primary" :disabled="modalSaving" @click="saveRole">
              {{ modalSaving ? 'Saving…' : editingRole._new ? 'Create role' : 'Save changes' }}
            </button>
          </div>
        </div>
      </div>

    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
definePageMeta({ layout: 'default' })

const api = useApi()

// ── Types ──────────────────────────────────────────────────────────────────
type PermLevel = 'none' | 'read' | 'write' | 'admin'

// ── Remote data ────────────────────────────────────────────────────────────
const roles      = ref<RoleRecord[]>([])
const users      = ref<UserRecord[]>([])
const rolesLoading = ref(true)
const usersLoading = ref(true)
const rolesError   = ref('')
const usersError   = ref('')

async function loadRoles() {
  rolesLoading.value = true
  rolesError.value = ''
  try {
    roles.value = await api.listRoles()
  } catch (e: any) {
    rolesError.value = e?.data?.error ?? 'Failed to load roles'
  } finally {
    rolesLoading.value = false
  }
}

async function loadUsers() {
  usersLoading.value = true
  usersError.value = ''
  try {
    users.value = await api.listUsers()
  } catch (e: any) {
    usersError.value = e?.data?.error ?? 'Failed to load users'
  } finally {
    usersLoading.value = false
  }
}

onMounted(() => { loadRoles(); loadUsers() })

// ── Permissions ────────────────────────────────────────────────────────────
const permResources = ['nodes', 'docker', 'kubernetes', 'git', 'storage', 'wireguard', 'plugins', 'sync', 'admin']
const permLevels: PermLevel[] = ['none', 'read', 'write', 'admin']

function permsToMatrix(perms: string[]): Record<string, PermLevel> {
  const m: Record<string, PermLevel> = {}
  for (const r of permResources) m[r] = 'none'
  for (const p of perms) {
    const [res, action] = p.split(':')
    if (res && action) m[res] = action === '*' ? 'admin' : action as PermLevel
  }
  return m
}

function matrixToPerms(m: Record<string, PermLevel>): string[] {
  return Object.entries(m)
    .filter(([, l]) => l !== 'none')
    .map(([res, l]) => `${res}:${l === 'admin' ? '*' : l}`)
}

// ── Modal state ────────────────────────────────────────────────────────────
const activeModal  = ref<'user' | 'role' | null>(null)
const modalError   = ref('')
const modalSaving  = ref(false)
const showPw       = ref(false)

const editingUser = reactive({
  _new: true, _origId: '',
  username: '', password: '', selectedRoles: [] as string[],
})

const editingRole = reactive({
  _new: true, _origName: '', _origId: '',
  name: '', description: '',
  permMatrix: {} as Record<string, PermLevel>,
})

const previewPerms = computed(() => matrixToPerms(editingRole.permMatrix))

function closeModal() { activeModal.value = null; modalError.value = ''; modalSaving.value = false }

// ── User modal ─────────────────────────────────────────────────────────────
function openNewUser() {
  Object.assign(editingUser, { _new: true, _origId: '', username: '', password: '', selectedRoles: [] })
  showPw.value = false
  modalError.value = ''
  activeModal.value = 'user'
}

function openEditUser(u: UserRecord) {
  Object.assign(editingUser, { _new: false, _origId: u.id, username: u.username, password: '', selectedRoles: [...(u.roles ?? [])] })
  showPw.value = false
  modalError.value = ''
  activeModal.value = 'user'
}

async function saveUser() {
  modalError.value = ''
  if (!editingUser.username.trim()) { modalError.value = 'Username is required.'; return }
  if (editingUser._new && !editingUser.password) { modalError.value = 'Password is required for new users.'; return }
  if (editingUser.password && editingUser.password.length < 12) {
    modalError.value = 'Password must be at least 12 characters.'; return
  }

  modalSaving.value = true
  try {
    if (editingUser._new) {
      await api.createUser({ username: editingUser.username, password: editingUser.password, roles: editingUser.selectedRoles })
    } else {
      const payload: { username?: string; password?: string; roles?: string[] } = { roles: editingUser.selectedRoles }
      if (editingUser.password) payload.password = editingUser.password
      await api.updateUser(editingUser._origId, payload)
    }
    await loadUsers()
    closeModal()
  } catch (e: any) {
    modalError.value = e?.data?.error ?? 'Save failed'
    modalSaving.value = false
  }
}

// ── Role modal ─────────────────────────────────────────────────────────────
function openNewRole() {
  const m: Record<string, PermLevel> = {}
  for (const r of permResources) m[r] = 'none'
  Object.assign(editingRole, { _new: true, _origName: '', _origId: '', name: '', description: '', permMatrix: m })
  modalError.value = ''
  activeModal.value = 'role'
}

function openEditRole(role: RoleRecord) {
  Object.assign(editingRole, {
    _new: false, _origName: role.name, _origId: role.id,
    name: role.name, description: role.description,
    permMatrix: permsToMatrix(role.perms ?? []),
  })
  modalError.value = ''
  activeModal.value = 'role'
}

async function saveRole() {
  modalError.value = ''
  const name = editingRole.name.trim().toLowerCase().replace(/\s+/g, '-')
  if (!name) { modalError.value = 'Role name is required.'; return }

  const perms = matrixToPerms(editingRole.permMatrix)
  modalSaving.value = true
  try {
    if (editingRole._new) {
      await api.createRole({ name, description: editingRole.description, perms })
    } else {
      await api.updateRole(editingRole._origId, { name, description: editingRole.description, perms })
    }
    await loadRoles()
    closeModal()
  } catch (e: any) {
    modalError.value = e?.data?.error ?? 'Save failed'
    modalSaving.value = false
  }
}

// ── Inline delete ──────────────────────────────────────────────────────────
const pendingDeleteUser = ref<string | null>(null)
const pendingDeleteRole = ref<string | null>(null)

async function confirmDeleteUser(id: string) {
  try {
    await api.deleteUser(id)
    await loadUsers()
  } catch {
    usersError.value = 'Delete failed'
  }
  pendingDeleteUser.value = null
}

async function confirmDeleteRole(id: string) {
  try {
    await api.deleteRole(id)
    await loadRoles()
  } catch {
    rolesError.value = 'Delete failed'
  }
  pendingDeleteRole.value = null
}

// ── Helpers ────────────────────────────────────────────────────────────────
function initials(name: string) { return (name ?? '?')[0]!.toUpperCase() }

const COLOR_MAP: Record<string, string> = { admin: 'purple', operator: 'blue', viewer: 'gray', developer: 'green', gitops: 'green' }
function roleColor(name: string): string {
  if (COLOR_MAP[name]) return COLOR_MAP[name]!
  const colors = ['purple', 'blue', 'green', 'orange', 'gray']
  let h = 0; for (const c of name) h = (h * 31 + c.charCodeAt(0)) & 0xffff
  return colors[h % colors.length]!
}

function formatDate(iso: string): string {
  if (!iso) return '—'
  try { return new Date(iso).toLocaleDateString() } catch { return iso }
}
</script>

<style scoped>
.settings-page    { padding: 1.5rem; }
.settings-layout  { display: grid; grid-template-columns: 200px 1fr; gap: 1.5rem; align-items: start; }
.settings-content { display: flex; flex-direction: column; gap: 1.25rem; }

/* Nav */
.settings-nav {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
}
.settings-nav__item {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
  font-size: 0.82rem;
  color: var(--text-muted);
  cursor: pointer;
  background: none;
  border: none;
  text-align: left;
  text-decoration: none;
  transition: color 0.15s, background 0.15s;
  width: 100%;
}
.settings-nav__item:hover       { color: var(--text-primary); background: var(--hover-bg); }
.settings-nav__item--active     { color: var(--accent-light); background: var(--accent-dim); }
.settings-nav__divider          { height: 1px; background: var(--border); margin: 0.25rem 0; }

/* Table */
.data-table { width: 100%; border-collapse: collapse; font-size: 0.82rem; }
.data-table th {
  text-align: left;
  padding: 0.5rem 0.75rem;
  color: var(--text-muted);
  font-weight: 500;
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid var(--border);
}
.data-table td       { padding: 0.625rem 0.75rem; border-bottom: 1px solid var(--border); color: var(--text-secondary); vertical-align: middle; }
.table-row:last-child td { border-bottom: none; }
.table-row:hover     { background: var(--hover-subtle); }
.cell--name    { display: flex; align-items: center; gap: 0.625rem; color: var(--text-primary); font-weight: 500; }
.cell--muted   { color: var(--text-muted); }
.cell--num     { color: var(--text-tertiary); }
.cell--actions { display: flex; align-items: center; gap: 0.25rem; justify-content: flex-end; white-space: nowrap; }
.empty-row     { text-align: center; color: var(--text-muted); font-style: italic; padding: 1.5rem !important; }

.loading-row, .api-error {
  padding: 1rem 0.75rem;
  font-size: 0.82rem;
  color: var(--text-muted);
}
.api-error { color: var(--danger-light); }

/* Inline delete confirm */
.delete-confirm-label {
  font-size: 0.75rem;
  color: var(--danger-light);
  margin-right: 0.25rem;
}

/* Role badge */
.role-badge {
  display: inline-flex;
  padding: 0.15rem 0.5rem;
  border-radius: 0.25rem;
  font-size: 0.7rem;
  font-weight: 600;
}
.role-badge--purple { background: var(--role-admin-bg);  color: #c084fc; border: 1px solid var(--role-admin-border); }
.role-badge--blue   { background: var(--role-op-bg);     color: #60a5fa; border: 1px solid var(--role-op-border);    }
.role-badge--green  { background: var(--role-dev-bg);    color: var(--success-light); border: 1px solid var(--role-dev-border); }
.role-badge--orange { background: rgba(194,65,12,.13);   color: #fb923c; border: 1px solid rgba(194,65,12,.3);       }
.role-badge--red    { background: var(--danger-bg);      color: var(--danger-light);  border: 1px solid var(--danger-border);   }
.role-badge--gray   { background: var(--role-gray-bg);   color: var(--text-muted); border: 1px solid var(--border-strong);    }

/* Perm chips */
.perm-chips { display: flex; flex-wrap: wrap; gap: 0.25rem; }
.perm-chip {
  background: var(--bg-overlay);
  border: 1px solid var(--border-strong);
  border-radius: 0.2rem;
  font-size: 0.68rem;
  font-family: monospace;
  color: var(--text-tertiary);
  padding: 0.1rem 0.35rem;
}

/* Avatar */
.avatar {
  width: 26px; height: 26px;
  background: linear-gradient(135deg, var(--accent), var(--accent-secondary));
  border-radius: 0.375rem;
  display: flex; align-items: center; justify-content: center;
  font-size: 0.65rem; font-weight: 700; color: #fff;
  flex-shrink: 0;
}

/* Buttons */
.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.3rem 0.625rem;
  border-radius: 0.375rem;
  font-size: 0.75rem;
  font-family: inherit;
  cursor: pointer;
  transition: all 0.15s;
  border: 1px solid transparent;
}
.action-btn--primary { background: var(--accent-subtle); border-color: var(--accent-soft); color: var(--accent-light); }
.action-btn--primary:hover { background: var(--accent-tint); }
.action-btn--primary:disabled { opacity: 0.5; cursor: not-allowed; }
.action-btn--ghost {
  background: none;
  border: 1px solid var(--border-strong);
  color: var(--text-tertiary);
  padding: 0.35rem 0.875rem;
  font-size: 0.8rem;
  font-weight: 500;
}
.action-btn--ghost:hover { background: var(--hover-bg); color: var(--text-primary); }

.icon-btn {
  background: none;
  border: 1px solid var(--border-strong);
  border-radius: 0.25rem;
  color: var(--text-muted);
  padding: 0.25rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  font-size: 0.72rem;
  font-family: inherit;
  transition: all 0.15s;
}
.icon-btn:hover           { color: var(--text-primary); border-color: var(--border-hover); }
.icon-btn--danger:hover   { color: var(--danger-light); border-color: #7f1d1d; }

/* Role checkboxes in user modal */
.role-checkbox-list { display: flex; flex-direction: column; gap: 0.5rem; }
.role-checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  font-size: 0.82rem;
}
.role-checkbox input { accent-color: var(--accent); width: 14px; height: 14px; cursor: pointer; flex-shrink: 0; }

/* ── Modal ──────────────────────────────────────────────────────────────── */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.55);
  backdrop-filter: blur(2px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
  padding: 1rem;
  overflow-y: auto;
}
.modal {
  width: 100%;
  max-width: 480px;
  background: var(--bg-surface);
  border: 1px solid var(--border-strong);
  border-radius: 0.75rem;
  box-shadow: 0 24px 64px var(--shadow-lg);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  margin: auto;
}
.modal--wide { max-width: 680px; }

.modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.modal__title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-primary);
}
.modal__close {
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  padding: 0.2rem;
  border-radius: 0.25rem;
  display: flex;
  transition: color 0.15s, background 0.15s;
}
.modal__close:hover { color: var(--text-primary); background: var(--hover-bg); }

.modal__body {
  padding: 1.25rem;
  overflow-y: auto;
}
.modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  padding: 0.875rem 1.25rem;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

/* ── Form elements ──────────────────────────────────────────────────────── */
.form-grid    { display: flex; flex-direction: column; gap: 0.875rem; }
.form-row     { display: flex; flex-direction: column; gap: 0.35rem; }
.form-row-2   { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
.form-label   { font-size: 0.75rem; color: var(--text-tertiary); }
.form-label-hint { color: var(--text-subtle); font-weight: 400; }
.form-error   { font-size: 0.75rem; color: var(--danger-light); margin: 0.5rem 0 0; }

.form-input {
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 0.375rem;
  padding: 0.5rem 0.75rem;
  font-size: 0.82rem;
  color: var(--text-primary);
  font-family: inherit;
  outline: none;
  width: 100%;
  box-sizing: border-box;
  transition: border-color 0.15s;
}
.form-input:focus        { border-color: var(--accent); }
.form-input:disabled     { opacity: 0.5; cursor: not-allowed; }
.form-input::placeholder { color: var(--text-subtle); }

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

/* Section divider inside modal */
.section-divider {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin: 0.25rem 0;
}
.section-divider::before,
.section-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--border);
}
.section-divider span {
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--text-subtle);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  white-space: nowrap;
}

/* Permission matrix */
.perm-matrix {
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  overflow: hidden;
  font-size: 0.78rem;
}
.perm-matrix__head,
.perm-matrix__row {
  display: grid;
  grid-template-columns: 1fr repeat(4, 56px);
  align-items: center;
}
.perm-matrix__head {
  background: var(--bg-elevated);
  border-bottom: 1px solid var(--border);
  padding: 0.4rem 0.75rem;
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.perm-matrix__head span:not(:first-child),
.perm-matrix__row label { text-align: center; }
.perm-matrix__row {
  padding: 0.35rem 0.75rem;
  border-bottom: 1px solid var(--border-faint);
}
.perm-matrix__row:last-child { border-bottom: none; }
.perm-matrix__row:hover      { background: var(--hover-subtle); }
.perm-matrix__res { color: var(--text-secondary); font-family: monospace; font-size: 0.75rem; }
.perm-radio { display: flex; justify-content: center; cursor: pointer; }
.perm-radio input { accent-color: var(--accent); width: 14px; height: 14px; cursor: pointer; }

.perm-preview { display: flex; flex-direction: column; gap: 0.35rem; }
.mt-xs { margin-top: 0.25rem; }

/* Modal transition */
.modal-enter-active, .modal-leave-active { transition: opacity 0.18s ease; }
.modal-enter-active .modal, .modal-leave-active .modal { transition: transform 0.18s ease, opacity 0.18s ease; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-from .modal, .modal-leave-to .modal { transform: translateY(-10px) scale(0.97); opacity: 0; }
</style>
