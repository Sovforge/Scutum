<template>
  <canvas ref="canvas" class="mesh-graph" />
</template>

<script setup lang="ts">
import { useResizeObserver, useRafFn } from '@vueuse/core'

export type NodeStatus = 'healthy' | 'degraded' | 'offline' | 'pending'
export type EdgeQuality = 'good' | 'degraded' | 'dead'

export interface MeshNode {
  id: string
  label: string
  role: string
  status: NodeStatus
}

export interface MeshEdge {
  source: string
  target: string
  quality: EdgeQuality
  latency?: string
}

const props = defineProps<{ nodes: MeshNode[]; edges: MeshEdge[] }>()

// ─── Internal sim state ────────────────────────────────────────────────────
interface SimNode extends MeshNode {
  x: number; y: number; vx: number; vy: number
}

const canvas = ref<HTMLCanvasElement | null>(null)
let simNodes: SimNode[] = []
let particleTick = 0
let settled = false
let settledFrames = 0

function initSim(w: number, h: number) {
  const cx = w / 2, cy = h / 2
  const r = Math.min(w, h) * 0.28
  simNodes = props.nodes.map((n, i) => {
    const angle = (i / props.nodes.length) * Math.PI * 2 - Math.PI / 2
    return { ...n, x: cx + r * Math.cos(angle), y: cy + r * Math.sin(angle), vx: 0, vy: 0 }
  })
  settled = false
  settledFrames = 0
}

function tick(w: number, h: number) {
  if (settled) return
  const cx = w / 2, cy = h / 2
  const nodeMap = Object.fromEntries(simNodes.map(n => [n.id, n]))

  // Center pull
  for (const n of simNodes) {
    n.vx += (cx - n.x) * 0.003
    n.vy += (cy - n.y) * 0.003
  }

  // Repulsion between all nodes
  for (let i = 0; i < simNodes.length; i++) {
    for (let j = i + 1; j < simNodes.length; j++) {
      const a = simNodes[i]!, b = simNodes[j]!
      const dx = b.x - a.x, dy = b.y - a.y
      const d = Math.sqrt(dx * dx + dy * dy) || 1
      const f = 9000 / (d * d)
      const fx = (dx / d) * f, fy = (dy / d) * f
      a.vx -= fx; a.vy -= fy
      b.vx += fx; b.vy += fy
    }
  }

  // Spring attraction along edges
  for (const e of props.edges) {
    const a = nodeMap[e.source], b = nodeMap[e.target]
    if (!a || !b) continue
    const dx = b.x - a.x, dy = b.y - a.y
    const d = Math.sqrt(dx * dx + dy * dy) || 1
    const ideal = Math.min(w, h) * 0.32
    const f = (d - ideal) * 0.006
    const fx = (dx / d) * f, fy = (dy / d) * f
    a.vx += fx; a.vy += fy
    b.vx -= fx; b.vy -= fy
  }

  // Integrate
  let maxV = 0
  for (const n of simNodes) {
    n.vx *= 0.82; n.vy *= 0.82
    n.x += n.vx; n.y += n.vy
    maxV = Math.max(maxV, Math.abs(n.vx) + Math.abs(n.vy))
  }

  if (maxV < 0.15) {
    settledFrames++
    if (settledFrames > 30) settled = true
  } else {
    settledFrames = 0
  }
}

// ─── Colors ────────────────────────────────────────────────────────────────
const STATUS_COLOR: Record<NodeStatus, string> = {
  healthy:  '#22c55e',
  degraded: '#f59e0b',
  offline:  '#ef4444',
  pending:  '#64748b',
}
const EDGE_COLOR: Record<EdgeQuality, string> = {
  good:     '#22c55e',
  degraded: '#f59e0b',
  dead:     '#ef444488',
}
const ROLE_ACCENT: Record<string, string> = {
  hub:      '#a78bfa',
  remote:   '#fb923c',
  combined: '#06b6d4',
}

// ─── Draw ──────────────────────────────────────────────────────────────────
function draw(ctx: CanvasRenderingContext2D, w: number, h: number) {
  ctx.clearRect(0, 0, w, h)
  const nodeMap = Object.fromEntries(simNodes.map(n => [n.id, n]))
  particleTick = (particleTick + 0.004) % 1

  // Edges
  for (const e of props.edges) {
    const a = nodeMap[e.source], b = nodeMap[e.target]
    if (!a || !b) continue
    const color = EDGE_COLOR[e.quality]

    ctx.save()
    ctx.strokeStyle = color
    ctx.globalAlpha = e.quality === 'dead' ? 0.25 : 0.35
    ctx.lineWidth = 1.5
    if (e.quality === 'dead') ctx.setLineDash([4, 6])
    else if (e.quality === 'degraded') ctx.setLineDash([6, 3])
    ctx.beginPath()
    ctx.moveTo(a.x, a.y)
    ctx.lineTo(b.x, b.y)
    ctx.stroke()
    ctx.restore()

    // Travelling particle (only on live edges)
    if (e.quality !== 'dead') {
      const t = (particleTick + props.edges.indexOf(e) * 0.17) % 1
      const px = a.x + (b.x - a.x) * t
      const py = a.y + (b.y - a.y) * t
      ctx.save()
      ctx.globalAlpha = 0.9
      ctx.shadowColor = color
      ctx.shadowBlur = 8
      ctx.fillStyle = '#fff'
      ctx.beginPath()
      ctx.arc(px, py, 2.5, 0, Math.PI * 2)
      ctx.fill()
      ctx.restore()
    }
  }

  // Nodes
  for (const n of simNodes) {
    const statusColor = STATUS_COLOR[n.status]
    const roleColor   = ROLE_ACCENT[n.role] ?? '#94a3b8'
    const isOnline    = n.status !== 'offline'
    const radius      = n.role === 'controller' ? 18 : 14

    // Outer glow ring
    if (isOnline) {
      const grad = ctx.createRadialGradient(n.x, n.y, radius, n.x, n.y, radius + 16)
      grad.addColorStop(0, statusColor + '55')
      grad.addColorStop(1, statusColor + '00')
      ctx.fillStyle = grad
      ctx.beginPath()
      ctx.arc(n.x, n.y, radius + 16, 0, Math.PI * 2)
      ctx.fill()
    }

    // Node circle fill
    ctx.save()
    ctx.shadowColor = isOnline ? statusColor : 'transparent'
    ctx.shadowBlur = isOnline ? 18 : 0
    const bg = ctx.createRadialGradient(n.x - radius * 0.3, n.y - radius * 0.3, 1, n.x, n.y, radius)
    bg.addColorStop(0, roleColor + 'cc')
    bg.addColorStop(1, roleColor + '44')
    ctx.fillStyle = bg
    ctx.beginPath()
    ctx.arc(n.x, n.y, radius, 0, Math.PI * 2)
    ctx.fill()

    // Status ring
    ctx.strokeStyle = statusColor
    ctx.lineWidth = isOnline ? 2 : 1
    ctx.globalAlpha = isOnline ? 0.9 : 0.4
    ctx.stroke()
    ctx.restore()

    // Label — read from CSS vars so they update with theme
    const rootStyle = getComputedStyle(document.documentElement)
    const labelColor = rootStyle.getPropertyValue('--text-primary').trim() || '#e2e8f0'
    const roleColor2 = rootStyle.getPropertyValue('--text-muted').trim()   || '#64748b'
    ctx.save()
    ctx.font = `500 11px "Inter", system-ui, sans-serif`
    ctx.textAlign = 'center'
    ctx.fillStyle = labelColor
    ctx.globalAlpha = 0.9
    ctx.fillText(n.label, n.x, n.y + radius + 14)
    ctx.font = `10px monospace`
    ctx.fillStyle = roleColor2
    ctx.fillText(n.role, n.x, n.y + radius + 26)
    ctx.restore()
  }
}

// ─── Resize + loop ─────────────────────────────────────────────────────────
let dpr = 1
let cw = 0, ch = 0

function resize(el: HTMLCanvasElement) {
  dpr = window.devicePixelRatio || 1
  const rect = el.getBoundingClientRect()
  cw = rect.width; ch = rect.height
  el.width  = cw * dpr
  el.height = ch * dpr
  initSim(cw, ch)
}

useResizeObserver(canvas, () => {
  if (canvas.value) resize(canvas.value)
})

onMounted(() => {
  if (canvas.value) resize(canvas.value)
})

// Re-initialise the simulation whenever the node list changes (e.g. after the
// API call on the parent page resolves). Without this, initSim runs once on
// mount with an empty array and the settled flag prevents any further drawing.
watch(() => props.nodes, () => {
  if (cw > 0) initSim(cw, ch)
})

useRafFn(() => {
  const el = canvas.value
  if (!el || cw === 0) return
  const ctx = el.getContext('2d')
  if (!ctx) return
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  tick(cw, ch)
  draw(ctx, cw, ch)
})
</script>

<style scoped>
.mesh-graph {
  width: 100%;
  height: 100%;
  display: block;
}
</style>
