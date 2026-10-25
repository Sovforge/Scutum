import { create } from 'zustand'

const SESSION_KEY = 'scutum_selected_node'

interface NodeSelectionState {
  selectedNodeId: string | null
  selectNode: (id: string | null) => void
}

// Which remote node the hub UI is currently "pointed at" for Docker/Kubernetes
// calls (proxied via the X-Target-Node header, see lib/api.ts). Global and
// session-persisted because it's set from the topbar switcher and read by
// pages (Containers, Kubernetes, Terminal, Observability) that don't
// otherwise share state.
export const useNodeSelection = create<NodeSelectionState>((set) => ({
  selectedNodeId: sessionStorage.getItem(SESSION_KEY),
  selectNode: (id) => {
    if (id) sessionStorage.setItem(SESSION_KEY, id)
    else sessionStorage.removeItem(SESSION_KEY)
    set({ selectedNodeId: id })
  },
}))
