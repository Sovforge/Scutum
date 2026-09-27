import { useEffect, useState } from 'react'
import { ChevronDown, Home, Server } from 'lucide-react'
import { listNodes, type NodeRecord } from '../../lib/api'
import { useNodeSelection } from '../../lib/nodeSelection'
import styles from './NodeSelect.module.css'

// Lets the hub UI "point at" any enrolled node for Docker/Kubernetes calls,
// proxied via X-Target-Node — see lib/nodeSelection.ts for the shared store
// this reads/writes, and lib/api.ts for how it's applied to requests.
function NodeSelect() {
  const { selectedNodeId, selectNode } = useNodeSelection()
  const [nodes, setNodes] = useState<NodeRecord[]>([])
  const [open, setOpen] = useState(false)

  useEffect(() => {
    listNodes()
      .then(setNodes)
      .catch(() => {})
  }, [])

  const remoteNodes = nodes.filter((n) => n.type !== 'hub')
  const selected = nodes.find((n) => n.id === selectedNodeId)

  return (
    <div className={styles.wrap}>
      <button type="button" className={styles.trigger} onClick={() => setOpen((v) => !v)}>
        <Server size={12} className={styles.icon} />
        <span className={styles.label}>{selected?.name ?? 'Local'}</span>
        <ChevronDown size={11} className={open ? styles.chevronOpen : styles.chevron} />
      </button>

      {open && (
        <>
          <div className={styles.scrim} onClick={() => setOpen(false)} />
          <div className={styles.menu}>
            <button
              type="button"
              className={selectedNodeId === null ? `${styles.option} ${styles.optionActive}` : styles.option}
              onClick={() => {
                selectNode(null)
                setOpen(false)
              }}
            >
              <Home size={12} />
              <span className={styles.optionLabel}>Local (this hub)</span>
            </button>
            {remoteNodes.length > 0 && <div className={styles.divider} />}
            {remoteNodes.map((n) => (
              <button
                key={n.id}
                type="button"
                className={selectedNodeId === n.id ? `${styles.option} ${styles.optionActive}` : styles.option}
                onClick={() => {
                  selectNode(n.id)
                  setOpen(false)
                }}
              >
                <Server size={12} />
                <span className={styles.optionLabel}>{n.name}</span>
                <span className={styles.optionRole}>{n.type}</span>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}

export default NodeSelect
