import { useId } from 'react'

// Lucid's mark: an L with a spark. The same drawing is public/favicon.svg.
export default function Logo({ size = 32, className }: { size?: number; className?: string }) {
  // Each copy needs its own gradient id: two on a page would otherwise share the first one's.
  const gradient = useId()
  return (
    // Rounded like its square, so a shadow on it follows the corners.
    <svg viewBox="0 0 32 32" width={size} height={size} className={className} style={{ borderRadius: '25%' }} aria-hidden>
      <defs>
        <linearGradient id={gradient} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#6366f1" />
          <stop offset="1" stopColor="#8b5cf6" />
        </linearGradient>
      </defs>
      <rect width="32" height="32" rx="8" fill={`url(#${gradient})`} />
      <path d="M11 8.5v15h10.5" fill="none" stroke="#fff" strokeWidth="3.4" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx="21.5" cy="10.5" r="2.6" fill="#7dd3fc" />
    </svg>
  )
}
