import { createElement } from 'react'
import icons from '../lib/icons.json'

// Lucide icon geometry exported from the approved design, drawn at stroke 1.8.
export type IconName = keyof typeof icons
const shapes = icons as unknown as Record<IconName, [string, Record<string, string>][]>

export function Icon({ name, className = 'icon' }: { name: IconName; className?: string }) {
  return (
    <svg className={className} aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round">
      {shapes[name].map(([tag, attrs], i) => createElement(tag, { key: i, ...attrs }))}
    </svg>
  )
}
