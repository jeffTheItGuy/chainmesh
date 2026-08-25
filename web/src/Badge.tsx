import type { ReactNode } from 'react'

type Tone = 'success' | 'danger' | 'neutral' | 'accent' | 'violet'

interface BadgeProps {
  tone?: Tone
  children: ReactNode
}

const TONE_CLASS: Record<Tone, string> = {
  success: 'badge badge--success',
  danger: 'badge badge--danger',
  neutral: 'badge badge--neutral',
  accent: 'badge badge--accent',
  violet: 'badge badge--violet',
}

export default function Badge({ tone = 'neutral', children }: BadgeProps) {
  return (
    <span className={TONE_CLASS[tone]}>
      <span className="badge-dot" />
      {children}
    </span>
  )
}