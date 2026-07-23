import { useState, type Dispatch, type FormEvent, type SetStateAction } from 'react'
import { Pencil, Plus, Shield, Trash2, Users } from 'lucide-react'
import Badge from '../../../components/ui/Badge/Badge'
import Button from '../../../components/ui/Button/Button'
import TextField from '../../../components/ui/TextField/TextField'
import PasswordField from '../../../components/ui/PasswordField/PasswordField'
import Select from '../../../components/ui/Select/Select'
import Checkbox from '../../../components/ui/Checkbox/Checkbox'
import Section from '../../../components/ui/Section/Section'
import Table from '../../../components/ui/Table/Table'
import EmptyState from '../../../components/ui/EmptyState/EmptyState'
import { useToast } from '../../../components/ui/Toast/ToastProvider'
import {
  RESOURCES,
  permLevel,
  setPermLevel,
  type PermLevel,
  type Resource,
  type RoleRecord,
  type UserRecord,
} from '../settingsData'
import styles from '../Settings.module.css'

const PERM_LEVELS: { value: PermLevel; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'read', label: 'Read' },
  { value: 'write', label: 'Write' },
  { value: 'admin', label: 'Admin' },
]

const LEVEL_VARIANT: Record<PermLevel, 'neutral' | 'success' | 'warning' | 'danger'> = {
  none: 'neutral',
  read: 'neutral',
  write: 'warning',
  admin: 'danger',
}

function UsersRolesTab({
  users,
  setUsers,
  roles,
  setRoles,
}: {
  users: UserRecord[]
  setUsers: Dispatch<SetStateAction<UserRecord[]>>
  roles: RoleRecord[]
  setRoles: Dispatch<SetStateAction<RoleRecord[]>>
}) {
  const toast = useToast()

  // ── Users ────────────────────────────────────────────────────────
  const [showUserForm, setShowUserForm] = useState(false)
  const [editingUserId, setEditingUserId] = useState<string | null>(null)
  const [userUsername, setUserUsername] = useState('')
  const [userPassword, setUserPassword] = useState('')
  const [userRoleIds, setUserRoleIds] = useState<string[]>([])
  const [userError, setUserError] = useState('')

  function openAddUser() {
    setEditingUserId(null)
    setUserUsername('')
    setUserPassword('')
    setUserRoleIds([])
    setUserError('')
    setShowUserForm(true)
  }

  function openEditUser(u: UserRecord) {
    setEditingUserId(u.id)
    setUserUsername(u.username)
    setUserPassword('')
    setUserRoleIds(u.roles)
    setUserError('')
    setShowUserForm(true)
  }

  function toggleUserRole(roleId: string) {
    setUserRoleIds((prev) => (prev.includes(roleId) ? prev.filter((r) => r !== roleId) : [...prev, roleId]))
  }

  function submitUser(e: FormEvent) {
    e.preventDefault()
    if (!userUsername) {
      setUserError('Username is required.')
      return
    }
    if (!editingUserId && (!userPassword || userPassword.length < 12)) {
      setUserError('Password must be at least 12 characters.')
      return
    }
    if (userPassword && userPassword.length < 12) {
      setUserError('Password must be at least 12 characters.')
      return
    }
    setUserError('')
    if (editingUserId) {
      setUsers((prev) => prev.map((u) => (u.id === editingUserId ? { ...u, username: userUsername, roles: userRoleIds } : u)))
      toast(`${userUsername} updated`)
    } else {
      setUsers((prev) => [...prev, { id: `usr_${userUsername}`, username: userUsername, roles: userRoleIds, createdAt: 'just now' }])
      toast(`${userUsername} created`)
    }
    setShowUserForm(false)
  }

  function deleteUser(u: UserRecord) {
    setUsers((prev) => prev.filter((x) => x.id !== u.id))
    toast(`${u.username} removed`, 'danger')
  }

  // ── Roles ────────────────────────────────────────────────────────
  const [showRoleForm, setShowRoleForm] = useState(false)
  const [editingRoleId, setEditingRoleId] = useState<string | null>(null)
  const [roleName, setRoleName] = useState('')
  const [roleDescription, setRoleDescription] = useState('')
  const [rolePerms, setRolePerms] = useState<string[]>([])
  const [roleError, setRoleError] = useState('')

  function openAddRole() {
    setEditingRoleId(null)
    setRoleName('')
    setRoleDescription('')
    setRolePerms([])
    setRoleError('')
    setShowRoleForm(true)
  }

  function openEditRole(r: RoleRecord) {
    setEditingRoleId(r.id)
    setRoleName(r.name)
    setRoleDescription(r.description)
    setRolePerms(r.perms)
    setRoleError('')
    setShowRoleForm(true)
  }

  function changeRolePerm(resource: Resource, level: PermLevel) {
    setRolePerms((prev) => setPermLevel(prev, resource, level))
  }

  function submitRole(e: FormEvent) {
    e.preventDefault()
    if (!roleName) {
      setRoleError('Role name is required.')
      return
    }
    setRoleError('')
    if (editingRoleId) {
      setRoles((prev) => prev.map((r) => (r.id === editingRoleId ? { ...r, name: roleName, description: roleDescription, perms: rolePerms } : r)))
      toast(`${roleName} updated`)
    } else {
      setRoles((prev) => [...prev, { id: `role_${roleName}`, name: roleName, description: roleDescription, perms: rolePerms }])
      toast(`${roleName} created`)
    }
    setShowRoleForm(false)
  }

  function deleteRole(r: RoleRecord) {
    const inUse = users.some((u) => u.roles.includes(r.id))
    setRoles((prev) => prev.filter((x) => x.id !== r.id))
    toast(inUse ? `${r.name} removed — some users referenced it` : `${r.name} removed`, 'danger')
  }

  function roleNameOf(id: string) {
    return roles.find((r) => r.id === id)?.name ?? id
  }

  return (
    <>
      <Section action={<Button variant="ghost" onClick={openAddUser}><Plus size={14} />Add user</Button>}>
        {showUserForm && (
          <form className={styles.inlineForm} onSubmit={submitUser}>
            <div className={styles.formFields}>
              <TextField label="Username" id="userUsername" value={userUsername} onChange={(e) => setUserUsername(e.target.value)} />
              <PasswordField
                label={editingUserId ? 'New password (optional)' : 'Password'}
                id="userPassword"
                value={userPassword}
                onChange={(e) => setUserPassword(e.target.value)}
                hint={<p className={styles.fieldHint}>Minimum 12 characters{editingUserId ? ' — leave blank to keep unchanged' : ''}.</p>}
              />
            </div>
            <div className={styles.roleChecks}>
              <span className={styles.fieldLabel}>Roles</span>
              <div className={styles.roleCheckList}>
                {roles.map((r) => (
                  <Checkbox key={r.id} checked={userRoleIds.includes(r.id)} onChange={() => toggleUserRole(r.id)}>
                    {r.name}
                  </Checkbox>
                ))}
              </div>
            </div>
            {userError && <p className={styles.formError}>{userError}</p>}
            <div className={styles.formActions}>
              <Button type="submit">{editingUserId ? 'Save user' : 'Create user'}</Button>
            </div>
          </form>
        )}

        {users.length === 0 ? (
          <EmptyState icon={Users} title="No users" description="Add a user to grant access to this Scutum instance." />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Username</th>
                <th>Roles</th>
                <th>Created</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id}>
                  <td className="cell-name">{u.username}</td>
                  <td>
                    <div className={styles.badgeRow}>
                      {u.roles.map((rid) => (
                        <Badge variant="neutral" key={rid}>
                          {roleNameOf(rid)}
                        </Badge>
                      ))}
                    </div>
                  </td>
                  <td className="cell-muted">{u.createdAt}</td>
                  <td>
                    <div className={styles.rowActions}>
                      <button type="button" onClick={() => openEditUser(u)} aria-label={`Edit ${u.username}`}>
                        <Pencil size={14} />
                      </button>
                      <button type="button" className={styles.rowActionDanger} onClick={() => deleteUser(u)} aria-label={`Remove ${u.username}`}>
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Section>

      <Section action={<Button variant="ghost" onClick={openAddRole}><Plus size={14} />Add role</Button>}>
        {showRoleForm && (
          <form className={styles.inlineForm} onSubmit={submitRole}>
            <div className={styles.formFields}>
              <TextField label="Name" id="roleName" value={roleName} onChange={(e) => setRoleName(e.target.value)} placeholder="ops-oncall" />
              <TextField
                label="Description"
                id="roleDescription"
                value={roleDescription}
                onChange={(e) => setRoleDescription(e.target.value)}
                placeholder="On-call operations"
              />
            </div>
            <div className={styles.permMatrix}>
              <span className={styles.fieldLabel}>Permissions</span>
              <div className={styles.permGrid}>
                {RESOURCES.map((resource) => (
                  <div className={styles.permRow} key={resource}>
                    <span className={styles.permResource}>{resource}</span>
                    <Select
                      label=""
                      id={`perm-${resource}`}
                      aria-label={`${resource} permission level`}
                      value={rolePerms.find((p) => p.startsWith(`${resource}:`))?.split(':')[1] ?? 'none'}
                      onChange={(e) => changeRolePerm(resource, e.target.value as PermLevel)}
                      options={PERM_LEVELS}
                    />
                  </div>
                ))}
              </div>
            </div>
            {roleError && <p className={styles.formError}>{roleError}</p>}
            <div className={styles.formActions}>
              <Button type="submit">{editingRoleId ? 'Save role' : 'Create role'}</Button>
            </div>
          </form>
        )}

        {roles.length === 0 ? (
          <EmptyState icon={Shield} title="No roles" description="Add a role to define what permissions a user gets." />
        ) : (
          <Table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Description</th>
                <th>Permissions</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {roles.map((r) => (
                <tr key={r.id}>
                  <td className="cell-name">{r.name}</td>
                  <td className="cell-muted">{r.description}</td>
                  <td>
                    <div className={styles.badgeRow}>
                      {RESOURCES.filter((resource) => permLevel(r, resource) !== 'none').map((resource) => (
                        <Badge variant={LEVEL_VARIANT[permLevel(r, resource)]} key={resource}>
                          {resource}:{permLevel(r, resource)}
                        </Badge>
                      ))}
                    </div>
                  </td>
                  <td>
                    <div className={styles.rowActions}>
                      <button type="button" onClick={() => openEditRole(r)} aria-label={`Edit ${r.name}`}>
                        <Pencil size={14} />
                      </button>
                      <button type="button" className={styles.rowActionDanger} onClick={() => deleteRole(r)} aria-label={`Remove ${r.name}`}>
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Section>
    </>
  )
}

export default UsersRolesTab
