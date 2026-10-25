import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type WheelEvent as ReactWheelEvent } from 'react'
import { Minus, Plus } from 'lucide-react'
import styles from './TopologyMap.module.css'

export type NodeStatus = 'healthy' | 'degraded' | 'offline'
export type MeshNode = {
  id: string
  name: string
  role: 'hub' | 'remote' | 'combined'
  status: NodeStatus
  // Arbitrary position on the globe, just to spread nodes out visually —
  // deliberately not real geolocation. See mockData.ts for why.
  lat: number
  lng: number
}

const STATUS_COLOR: Record<NodeStatus, string> = {
  healthy: 'var(--blueprint-bright)',
  degraded: 'var(--rust-bright)',
  offline: 'var(--danger)',
}

const STATUS_LABEL: Record<NodeStatus, string> = {
  healthy: 'Healthy',
  degraded: 'Degraded',
  offline: 'Offline',
}

const CENTER = 200
const GLOBE_RADIUS = 150
const DEG = Math.PI / 180

type Vec3 = readonly [number, number, number]
type Point2D = { x: number; y: number; front: boolean }

function toVec3(lat: number, lng: number): Vec3 {
  const phi = (90 - lat) * DEG
  const theta = (lng + 180) * DEG
  return [-Math.sin(phi) * Math.cos(theta), Math.cos(phi), Math.sin(phi) * Math.sin(theta)]
}

// Yaw (around the vertical axis, driven by horizontal drag) then pitch
// (around the horizontal axis, driven by vertical drag).
function rotate([x, y, z]: Vec3, rotX: number, rotY: number): Vec3 {
  const cosY = Math.cos(rotY)
  const sinY = Math.sin(rotY)
  const x1 = x * cosY + z * sinY
  const z1 = -x * sinY + z * cosY
  const cosX = Math.cos(rotX)
  const sinX = Math.sin(rotX)
  const y2 = y * cosX - z1 * sinX
  const z2 = y * sinX + z1 * cosX
  return [x1, y2, z2]
}

function project(v: Vec3): Point2D {
  return { x: CENTER + v[0] * GLOBE_RADIUS, y: CENTER - v[1] * GLOBE_RADIUS, front: v[2] > 0.02 }
}

// Spherical linear interpolation — so a connection between two nodes
// follows the globe's curvature (a "flight path"), instead of a straight
// chord that would visibly cut through the sphere.
function slerp(a: Vec3, b: Vec3, t: number): Vec3 {
  const dot = Math.max(-1, Math.min(1, a[0] * b[0] + a[1] * b[1] + a[2] * b[2]))
  const theta = Math.acos(dot) * t
  const relX = b[0] - a[0] * dot
  const relY = b[1] - a[1] * dot
  const relZ = b[2] - a[2] * dot
  const len = Math.sqrt(relX * relX + relY * relY + relZ * relZ) || 1
  const rx = relX / len
  const ry = relY / len
  const rz = relZ / len
  const cosT = Math.cos(theta)
  const sinT = Math.sin(theta)
  return [a[0] * cosT + rx * sinT, a[1] * cosT + ry * sinT, a[2] * cosT + rz * sinT]
}

// Graticule: latitude/longitude grid lines, sampled once at module scope
// (they don't depend on rotation — only the projection below does).
const PARALLELS: Vec3[][] = [-60, -30, 0, 30, 60].map((lat) => {
  const pts: Vec3[] = []
  for (let lng = 0; lng <= 360; lng += 8) pts.push(toVec3(lat, lng))
  return pts
})
const MERIDIANS: Vec3[][] = Array.from({ length: 12 }, (_, i) => i * 30).map((lng) => {
  const pts: Vec3[] = []
  for (let lat = -90; lat <= 90; lat += 8) pts.push(toVec3(lat, lng))
  return pts
})

// Draws only the front-facing segments of a sampled curve, starting a new
// subpath wherever it dips behind the sphere — so the far side of the grid
// doesn't draw a stray line straight across the globe's face.
function buildPath(points: Point2D[]): string {
  let d = ''
  let penDown = false
  for (const p of points) {
    if (!p.front) {
      penDown = false
      continue
    }
    d += `${penDown ? 'L' : 'M'} ${p.x.toFixed(1)} ${p.y.toFixed(1)} `
    penDown = true
  }
  return d.trim()
}

// Puts the real node cluster (Frankfurt/London/NYC/Ashburn) roughly
// front-and-center on first paint, tilted to favor the northern hemisphere
// they all sit in — see the derivation in the PR/commit notes if this ever
// needs adjusting; it's draggable regardless, so it doesn't have to be exact.
const INITIAL_ROT_Y = -0.95
const INITIAL_ROT_X = 0.3
const MIN_ZOOM = 0.7
const MAX_ZOOM = 2.8

function TopologyMap({
  nodes,
  maxWidth,
  tvMode = false,
}: {
  nodes: readonly MeshNode[]
  maxWidth?: number
  // TV mode is a passive, unattended display — no mouse/keyboard nearby, so
  // the drag/zoom hint and on-screen zoom buttons would just be clutter.
  // Dragging and wheel-zoom still work if a pointer happens to be present;
  // this only hides the affordances that assume one.
  tvMode?: boolean
}) {
  const [rotY, setRotY] = useState(INITIAL_ROT_Y)
  const [rotX, setRotX] = useState(INITIAL_ROT_X)
  const [zoom, setZoom] = useState(1)
  const draggingRef = useRef(false)
  const dragStartRef = useRef<{ x: number; y: number; rotX: number; rotY: number } | null>(null)

  function zoomBy(factor: number) {
    setZoom((z) => Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, z * factor)))
  }

  function onWheel(e: ReactWheelEvent<SVGSVGElement>) {
    e.preventDefault()
    zoomBy(e.deltaY > 0 ? 0.9 : 1.1)
  }

  // Slow idle auto-spin, paused while the user is actively dragging.
  useEffect(() => {
    let raf = 0
    function tick() {
      if (!draggingRef.current) setRotY((r) => r + 0.0015)
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [])

  function onPointerDown(e: ReactPointerEvent<SVGSVGElement>) {
    draggingRef.current = true
    dragStartRef.current = { x: e.clientX, y: e.clientY, rotX, rotY }
    e.currentTarget.setPointerCapture(e.pointerId)
  }
  function onPointerMove(e: ReactPointerEvent<SVGSVGElement>) {
    if (!draggingRef.current || !dragStartRef.current) return
    const dx = e.clientX - dragStartRef.current.x
    const dy = e.clientY - dragStartRef.current.y
    setRotY(dragStartRef.current.rotY + dx * 0.006)
    setRotX(Math.max(-1.3, Math.min(1.3, dragStartRef.current.rotX - dy * 0.006)))
  }
  function onPointerUp() {
    draggingRef.current = false
    dragStartRef.current = null
  }

  const rotatedParallels = PARALLELS.map((loop) => loop.map((v) => project(rotate(v, rotX, rotY))))
  const rotatedMeridians = MERIDIANS.map((loop) => loop.map((v) => project(rotate(v, rotX, rotY))))

  const nodePositions = nodes.map((node) => {
    const vec = toVec3(node.lat, node.lng)
    return { node, vec, ...project(rotate(vec, rotX, rotY)) }
  })

  const primaryHub = nodePositions.find((n) => n.node.role === 'hub') ?? nodePositions[0]

  const edgeArcs = primaryHub
    ? nodePositions
        .filter((n) => n.node.id !== primaryHub.node.id)
        .map((n) => {
          const steps = 24
          const pts = Array.from({ length: steps + 1 }, (_, i) => project(rotate(slerp(primaryHub.vec, n.vec, i / steps), rotX, rotY)))
          return { id: n.node.id, status: n.node.status, path: buildPath(pts) }
        })
    : []

  const viewSize = 400 / zoom
  const viewOffset = (400 - viewSize) / 2

  return (
    <div className={tvMode ? `${styles.wrap} ${styles.wrapFill}` : styles.wrap}>
      <div className={styles.stage}>
        <svg
          className={styles.svg}
          viewBox={`${viewOffset} ${viewOffset} ${viewSize} ${viewSize}`}
          role="img"
          aria-label="Mesh node locations on a rotatable, zoomable globe"
          style={maxWidth ? { maxWidth } : undefined}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerLeave={onPointerUp}
          onWheel={onWheel}
        >
        <defs>
          <radialGradient id="globeShade" cx="35%" cy="30%" r="75%">
            <stop offset="0%" stopColor="var(--ink-raised)" />
            <stop offset="100%" stopColor="var(--ink)" />
          </radialGradient>
          <radialGradient id="globeLimb" cx="50%" cy="50%" r="50%">
            <stop offset="82%" stopColor="var(--blueprint)" stopOpacity="0" />
            <stop offset="100%" stopColor="var(--blueprint)" stopOpacity="0.3" />
          </radialGradient>
          <linearGradient id="beamGradient" x1="0" y1="1" x2="0" y2="0">
            <stop offset="0%" stopColor="var(--blueprint-bright)" stopOpacity="0.5" />
            <stop offset="100%" stopColor="var(--blueprint-bright)" stopOpacity="0" />
          </linearGradient>
          <radialGradient id="emitterGlow" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor="var(--blueprint-bright)" stopOpacity="0.9" />
            <stop offset="100%" stopColor="var(--blueprint-bright)" stopOpacity="0" />
          </radialGradient>
          <pattern id="scanlines" width="4" height="4" patternUnits="userSpaceOnUse">
            <line x1="0" y1="0.5" x2="4" y2="0.5" stroke="var(--blueprint-bright)" strokeWidth="1" />
          </pattern>
          <clipPath id="globeClip">
            <circle cx={CENTER} cy={CENTER} r={GLOBE_RADIUS} />
          </clipPath>
        </defs>

        {/* Hologram projector — a small pedestal below the globe with a
            light cone reaching up into it, so this reads as "a projection"
            rather than a ball floating in space. */}
        <g className={styles.projector} aria-hidden="true">
          <path d={`M ${CENTER - 10} 380 L ${CENTER - 42} 344 L ${CENTER + 42} 344 L ${CENTER + 10} 380 Z`} fill="url(#beamGradient)" />
          <ellipse className={styles.pedestal} cx={CENTER} cy={387} rx={54} ry={9} />
          <ellipse className={styles.pedestalRim} cx={CENTER} cy={384} rx={54} ry={9} />
          <ellipse cx={CENTER} cy={384} rx={20} ry={5} fill="url(#emitterGlow)" />
          <ellipse className={styles.emitter} cx={CENTER} cy={384} rx={9} ry={2.4} />
        </g>

        <circle cx={CENTER} cy={CENTER} r={GLOBE_RADIUS} fill="url(#globeShade)" />

        <g className={styles.graticule} aria-hidden="true">
          {rotatedParallels.map((loop, i) => (
            <path key={`p${i}`} d={buildPath(loop)} />
          ))}
          {rotatedMeridians.map((loop, i) => (
            <path key={`m${i}`} d={buildPath(loop)} />
          ))}
        </g>

        <circle className={styles.limb} cx={CENTER} cy={CENTER} r={GLOBE_RADIUS} fill="url(#globeLimb)" />
        <circle className={styles.outline} cx={CENTER} cy={CENTER} r={GLOBE_RADIUS} />

        {edgeArcs.map((arc) => (
          <path key={arc.id} className={styles.edge} d={arc.path} style={{ stroke: STATUS_COLOR[arc.status] }} />
        ))}

        {nodePositions.map(({ node, x, y, front }) => {
          const isHub = node.role === 'hub'
          return (
            <g key={node.id} transform={`translate(${x} ${y})`} className={front ? styles.nodeFront : styles.nodeBack}>
              <circle
                className={isHub ? `${styles.node} ${styles.hubNode}` : styles.node}
                r={isHub ? 7 : 5}
                style={{ fill: STATUS_COLOR[node.status] }}
              />
              {front && (
                <text className={isHub ? `${styles.label} ${styles.hubLabel}` : styles.label} y={isHub ? -14 : 14}>
                  {node.name}
                </text>
              )}
            </g>
          )
        })}

        <circle
          className={styles.scanlines}
          cx={CENTER}
          cy={CENTER}
          r={GLOBE_RADIUS}
          fill="url(#scanlines)"
          clipPath="url(#globeClip)"
          aria-hidden="true"
        />
        </svg>

        {!tvMode && (
          <div className={styles.zoomControls}>
            <button type="button" onClick={() => zoomBy(1.25)} aria-label="Zoom in">
              <Plus size={14} />
            </button>
            <button type="button" onClick={() => zoomBy(0.8)} aria-label="Zoom out">
              <Minus size={14} />
            </button>
          </div>
        )}
      </div>

      {!tvMode && <p className={styles.hint}>Drag to rotate · scroll or use the controls to zoom</p>}

      <div className={styles.legend}>
        {(Object.keys(STATUS_LABEL) as NodeStatus[]).map((status) => (
          <span key={status} className={styles.legendItem}>
            <span className={styles.legendDot} style={{ background: STATUS_COLOR[status] }} />
            {STATUS_LABEL[status]}
          </span>
        ))}
      </div>
    </div>
  )
}

export default TopologyMap
