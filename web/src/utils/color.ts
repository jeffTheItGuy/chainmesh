// Deterministic string -> color mapping. Gives each blockchain network
// (or any other id) a consistent accent color everywhere it shows up in
// the dashboard, without needing the backend to assign one.

const PALETTE = [
  '#3452eb', // accent blue
  '#178a56', // success green
  '#c2410c', // burnt orange
  '#7c3aed', // violet
  '#0891b2', // cyan
  '#b8860b', // amber
  '#d6304a', // danger red
  '#4d7c0f', // olive
]

export function colorForId(id: string | undefined | null): string {
  if (!id) return 'var(--text-faint)'
  let hash = 0
  for (let i = 0; i < id.length; i++) {
    hash = (hash << 5) - hash + id.charCodeAt(i)
    hash |= 0
  }
  const index = Math.abs(hash) % PALETTE.length
  return PALETTE[index]
}