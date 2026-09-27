import { useState } from 'react'
import { Bell, Boxes, KeyRound, Settings as SettingsIcon, Shield, ShieldAlert, Users, Webhook } from 'lucide-react'
import AppShell from '../../components/AppShell/AppShell'
import Tabs, { type TabItem } from '../../components/ui/Tabs/Tabs'
import UsersRolesTab from './tabs/UsersRolesTab'
import RecoveryTab from './tabs/RecoveryTab'
import SecretsTab from './tabs/SecretsTab'
import ScimTab from './tabs/ScimTab'
import WebhooksTab from './tabs/WebhooksTab'
import BackupTab from './tabs/BackupTab'
import AlertsTab from './tabs/AlertsTab'
import GeneralTab from './tabs/GeneralTab'
import type { RoleRecord, UserRecord } from '../../lib/api'
import styles from './Settings.module.css'

// Federation lives on the Network page (wired to the real
// /federation/peers endpoints there) — not duplicated here.
type Tab = 'users' | 'recovery' | 'secrets' | 'scim' | 'webhooks' | 'backup' | 'alerts' | 'general'

const TABS: TabItem[] = [
  { id: 'users', label: 'Users & Roles', icon: Users },
  { id: 'recovery', label: 'Recovery', icon: ShieldAlert },
  { id: 'secrets', label: 'Secrets', icon: KeyRound },
  { id: 'scim', label: 'SCIM', icon: Boxes },
  { id: 'webhooks', label: 'Webhooks', icon: Webhook },
  { id: 'backup', label: 'Backup', icon: Shield },
  { id: 'alerts', label: 'Alerts', icon: Bell },
  { id: 'general', label: 'General', icon: SettingsIcon },
]

function Settings() {
  const [tab, setTab] = useState<Tab>('users')

  // Owned here (not inside UsersRolesTab) so the header stamp below stays
  // live as users/roles are added or removed. UsersRolesTab does the actual
  // fetching (it's the default tab, so it mounts immediately) and writes
  // results back through these setters.
  const [users, setUsers] = useState<UserRecord[]>([])
  const [roles, setRoles] = useState<RoleRecord[]>([])

  return (
    <AppShell title="Settings">
      <div className={styles.page}>
        <div className={styles.reportHeader}>
          <div>
            <p className="eyebrow">Administration</p>
            <h2 className={styles.reportTitle}>Settings</h2>
          </div>
          <span className="stamp">
            {users.length} users · {roles.length} roles
          </span>
        </div>

        <Tabs tabs={TABS} active={tab} onChange={(id) => setTab(id as Tab)} />

        {tab === 'users' && <UsersRolesTab users={users} setUsers={setUsers} roles={roles} setRoles={setRoles} />}
        {tab === 'recovery' && <RecoveryTab />}
        {tab === 'secrets' && <SecretsTab />}
        {tab === 'scim' && <ScimTab />}
        {tab === 'webhooks' && <WebhooksTab />}
        {tab === 'backup' && <BackupTab />}
        {tab === 'alerts' && <AlertsTab />}
        {tab === 'general' && <GeneralTab />}
      </div>
    </AppShell>
  )
}

export default Settings
