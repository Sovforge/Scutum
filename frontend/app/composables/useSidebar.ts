export function useSidebar() {
  const open = useState('sidebar-open', () => false)
  const toggle = () => (open.value = !open.value)
  const close  = () => (open.value = false)
  return { open, toggle, close }
}
