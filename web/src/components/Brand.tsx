import type { HTMLAttributes } from 'react'

export type BrandSize = 'small' | 'medium' | 'large'

export interface BrandProps extends Omit<HTMLAttributes<HTMLDivElement>, 'children'> {
  /** Applies a stable size variant for the consuming layout to style. */
  size?: BrandSize
  /** Marks the compact layout variant without changing the accessible brand name. */
  compact?: boolean
  /** Allows the logo alternative text to match the surrounding context. */
  logoAlt?: 'Allblu' | 'Allblu Logo'
}

export function Brand({
  className,
  compact = false,
  logoAlt = 'Allblu Logo',
  size = 'medium',
  ...props
}: BrandProps) {
  const classes = ['allblu-brand', `allblu-brand--${size}`, compact && 'allblu-brand--compact', className]
    .filter(Boolean)
    .join(' ')

  return (
    <div className={classes} data-compact={compact || undefined} {...props}>
      <img className="allblu-brand__logo" src="/logo.jpg" alt={logoAlt} />
      <span className="allblu-brand__name">Allblu</span>
    </div>
  )
}
