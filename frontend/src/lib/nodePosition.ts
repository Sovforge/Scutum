export type GeoCoords = { lat: number; lng: number }

// Deterministic, local, and entirely made up — this hashes a node's own
// WireGuard mesh IP into a stable globe position. It is NOT geolocation:
// the mesh address is a private 10.100.0.x range and carries no real-world
// location information, so the result never reflects where the hardware
// actually is, only a consistent-looking placement for the same IP. No
// third party is involved and nothing is ever looked up over the network —
// publishing a self-hosted node's real physical location would be an
// infosec liability for this product, not a feature (see the mockData.ts
// comment on NODES for the fuller reasoning).
export function positionFromMeshIp(ip: string): GeoCoords {
  let hash = 0
  for (let i = 0; i < ip.length; i++) {
    hash = (hash * 31 + ip.charCodeAt(i)) >>> 0
  }
  const latSeed = (hash % 1000) / 1000
  const lngSeed = (Math.floor(hash / 1000) % 1000) / 1000
  return {
    lat: -60 + latSeed * 120, // stay off the poles for a nicer spread
    lng: -180 + lngSeed * 360,
  }
}
