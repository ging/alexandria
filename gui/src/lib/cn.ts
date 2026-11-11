// Class name helper combining clsx conditionals with Tailwind conflict resolution.
// Used by every component so later classes reliably override earlier ones.
import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

// cn merges class values into one deduplicated Tailwind class string.
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
