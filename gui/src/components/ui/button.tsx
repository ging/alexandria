// Button primitives of the console: labelled buttons and square icon buttons in four visual variants.
// Primary actions carry the warm acid gradient while secondary controls stay strictly neutral.
import type { ButtonHTMLAttributes, ReactNode, Ref } from 'react'
import { Loader2 } from 'lucide-react'
import { cn } from '@/lib/cn'

// Variant is the visual weight of a button.
export type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'

const variants: Record<Variant, string> = {
  primary: 'bg-accent-gradient text-accent-fg border-transparent shadow-2xs hover:opacity-92 active:opacity-85',
  secondary: 'bg-raised text-fg border-line-strong hover:bg-hover',
  ghost: 'bg-transparent text-muted border-transparent hover:bg-hover hover:text-fg',
  danger: 'bg-transparent text-danger border-danger/40 hover:bg-danger-soft',
}

// ButtonProps extends the native button with variant, size, icon and loading state.
export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: 'sm' | 'md'
  icon?: ReactNode
  loading?: boolean
  ref?: Ref<HTMLButtonElement>
}

// Button renders a compact labelled action.
export function Button({ variant = 'secondary', size = 'md', icon, loading, className, children, disabled, type = 'button', ...rest }: ButtonProps) {
  return (
    <button
      type={type}
      disabled={disabled || loading}
      className={cn(
        'inline-flex shrink-0 items-center justify-center gap-1.5 rounded-md border font-medium whitespace-nowrap transition-all select-none disabled:opacity-50',
        size === 'sm' ? 'h-7 px-2.5 text-xs' : 'h-8 px-3 text-sm',
        variants[variant],
        className,
      )}
      {...rest}
    >
      {loading ? <Loader2 className="size-3.5 animate-spin" /> : icon}
      {children}
    </button>
  )
}

// IconButton renders a square icon-only action that requires an accessible label.
export function IconButton({ label, className, size = 'md', children, ...rest }: Omit<ButtonProps, 'icon'> & { label: string }) {
  return (
    <Button aria-label={label} title={label} variant="ghost" size={size} className={cn(size === 'sm' ? 'w-7 px-0' : 'w-8 px-0', className)} {...rest}>
      {children}
    </Button>
  )
}
