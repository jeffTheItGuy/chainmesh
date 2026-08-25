interface IconProps {
  size?: number
  className?: string
}

const strokeProps = {
  fill: 'none' as const,
  stroke: 'currentColor' as const,
  strokeWidth: 1.6,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
}

export function IconActivity({ size = 15, className }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" className={className} {...strokeProps}>
      <path d="M2 13h4l2.5-7 4 14 3-11 2 4h4.5" />
    </svg>
  )
}

export function IconServer({ size = 15, className }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" className={className} {...strokeProps}>
      <rect x="3" y="4" width="18" height="7" rx="1.5" />
      <rect x="3" y="13" width="18" height="7" rx="1.5" />
      <circle cx="7" cy="7.5" r="0.9" fill="currentColor" stroke="none" />
      <circle cx="7" cy="16.5" r="0.9" fill="currentColor" stroke="none" />
    </svg>
  )
}

export function IconLink({ size = 15, className }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" className={className} {...strokeProps}>
      <rect x="2.5" y="8" width="9" height="8" rx="4" transform="rotate(-20 7 12)" />
      <rect x="12.5" y="8" width="9" height="8" rx="4" transform="rotate(-20 17 12)" />
    </svg>
  )
}

export function IconUsers({ size = 15, className }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" className={className} {...strokeProps}>
      <circle cx="9" cy="8" r="3" />
      <path d="M3.5 20c0-3.3 2.5-6 5.5-6s5.5 2.7 5.5 6" />
      <circle cx="17.5" cy="9" r="2.3" />
      <path d="M15.5 14.2c2.5.3 4.5 2.6 4.5 5.8" />
    </svg>
  )
}

export function IconGauge({ size = 15, className }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" className={className} {...strokeProps}>
      <path d="M4 15a8 8 0 1 1 16 0" />
      <path d="M12 15l4-5" />
      <circle cx="12" cy="15" r="1" fill="currentColor" stroke="none" />
    </svg>
  )
}

export function IconCube({ size = 15, className }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" className={className} {...strokeProps}>
      <path d="M12 3l8 4.5v9L12 21l-8-4.5v-9L12 3z" />
      <path d="M4 7.5L12 12l8-4.5" />
      <path d="M12 12v9" />
    </svg>
  )
}

export function IconRefresh({ size = 14, className }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" className={className} {...strokeProps}>
      <path d="M20.5 12a8.5 8.5 0 1 1-2.5-6" />
      <path d="M20.5 4.5v5.5H15" />
    </svg>
  )
}

export function IconCopy({ size = 13, className }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" className={className} {...strokeProps}>
      <rect x="8" y="8" width="12" height="12" rx="1.5" />
      <path d="M5.5 16H4.5A1.5 1.5 0 0 1 3 14.5v-10A1.5 1.5 0 0 1 4.5 3h10A1.5 1.5 0 0 1 16 4.5v1" />
    </svg>
  )
}