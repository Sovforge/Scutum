import type { NodeRecord } from '~/composables/useApi'

const SESSION_KEY = 'scutum_selected_node'

export const useNodesStore = defineStore('nodes', () => {
  const nodes      = ref<NodeRecord[]>([])
  const selectedId = ref<string | null>(
    import.meta.client ? (sessionStorage.getItem(SESSION_KEY) ?? null) : null
  )

  const selected = computed(() => nodes.value.find(n => n.id === selectedId.value) ?? null)

  async function load() {
    try {
      nodes.value = await useApi().listNodes()
      // Clear selection if the saved node no longer exists
      if (selectedId.value && !nodes.value.find(n => n.id === selectedId.value)) {
        select(null)
      }
    } catch {}
  }

  function select(id: string | null) {
    selectedId.value = id
    if (import.meta.client) {
      if (id) sessionStorage.setItem(SESSION_KEY, id)
      else sessionStorage.removeItem(SESSION_KEY)
    }
  }

  return { nodes, selectedId, selected, load, select }
})
