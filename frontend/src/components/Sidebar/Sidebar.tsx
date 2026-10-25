import { Link, useLocation } from 'react-router-dom'
import {
  Activity,
  Box,
  Boxes,
  ChevronsLeft,
  Cpu,
  GitBranch,
  HardDrive,
  LayoutDashboard,
  Network,
  Puzzle,
  ScrollText,
  Server,
  Settings,
  Terminal,
  Tv,
  X,
  type LucideIcon,
} from 'lucide-react'
import styles from './Sidebar.module.css'

export type NavItem = {
  label: string
  icon: LucideIcon
  to?: string // omitted = page not built yet
}

// Full section list matches the real app (see the old Nuxt frontend's nav);
// only Dashboard exists as a real route in this rebuild so far — the rest
// show as visibly disabled rather than dead links. Exported so the command
// palette can search the same list instead of duplicating it.
export const NAV_ITEMS: NavItem[] = [
  { label: 'Dashboard', icon: LayoutDashboard, to: '/dashboard' },
  { label: 'Nodes', icon: Server, to: '/nodes' },
  { label: 'Monitoring', icon: Cpu, to: '/monitoring' },
  { label: 'Containers', icon: Box, to: '/containers' },
  { label: 'Kubernetes', icon: Boxes, to: '/kubernetes' },
  { label: 'Network', icon: Network, to: '/network' },
  { label: 'Storage', icon: HardDrive, to: '/storage' },
  { label: 'Observability', icon: Activity, to: '/observability' },
  { label: 'Terminal', icon: Terminal, to: '/terminal' },
  { label: 'GitOps', icon: GitBranch, to: '/gitops' },
  { label: 'Plugins', icon: Puzzle, to: '/plugins' },
  { label: 'Audit Log', icon: ScrollText, to: '/audit' },
  { label: 'Settings', icon: Settings, to: '/settings' },
]

function Sidebar({
  collapsed,
  onToggleCollapse,
  mobileOpen,
  onCloseMobile,
}: {
  collapsed: boolean
  onToggleCollapse: () => void
  mobileOpen: boolean
  onCloseMobile: () => void
}) {
  const location = useLocation()

  return (
    <>
      {mobileOpen && <div className={styles.scrim} onClick={onCloseMobile} aria-hidden="true" />}
      <aside className={`${styles.sidebar} ${collapsed ? styles.collapsed : ''} ${mobileOpen ? styles.mobileOpen : ''}`}>
        <div className={styles.header}>
          <Link to="/" className="brand-mark">
            <img src="/logo.svg" alt="" className="brand-mark__logo" />
            {!collapsed && <span className="brand-mark__word">SCUTUM</span>}
          </Link>
          <button type="button" className={styles.mobileClose} onClick={onCloseMobile} aria-label="Close menu">
            <X size={18} />
          </button>
        </div>

        <nav className={styles.nav}>
          {NAV_ITEMS.map((item) => {
            const Icon = item.icon
            const active = item.to === location.pathname
            const body = (
              <>
                <Icon size={17} className={styles.icon} />
                {!collapsed && <span className={styles.label}>{item.label}</span>}
                {!collapsed && !item.to && <span className={styles.soon}>Soon</span>}
              </>
            )
            return item.to ? (
              <Link
                key={item.label}
                to={item.to}
                className={active ? `${styles.item} ${styles.itemActive}` : styles.item}
                onClick={onCloseMobile}
              >
                {body}
              </Link>
            ) : (
              <span key={item.label} className={`${styles.item} ${styles.itemDisabled}`} aria-disabled="true">
                {body}
              </span>
            )
          })}
        </nav>

        <div className={styles.footer}>
          <Link to="/tv" className={styles.item} onClick={onCloseMobile}>
            <Tv size={17} className={styles.icon} />
            {!collapsed && <span className={styles.label}>TV Mode</span>}
          </Link>
          <button
            type="button"
            className={styles.collapseBtn}
            onClick={onToggleCollapse}
            aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            <ChevronsLeft size={16} className={collapsed ? styles.flip : undefined} />
            {!collapsed && <span>Collapse</span>}
          </button>
        </div>
      </aside>
    </>
  )
}

export default Sidebar
