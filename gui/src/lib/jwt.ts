// Minimal JWT payload decoding for display purposes only: subject, roles and expiry of a pasted access token.
// Signatures are never verified here; the hub is the only authority that validates tokens (checks A1–A5).

// Claims is the subset of at+jwt claims the console displays.
export interface Claims {
  iss?: string
  sub?: string
  aud?: string | string[]
  exp?: number
  iat?: number
  roles?: string[]
  [k: string]: unknown
}

// decodeJwt returns the payload claims of a compact JWT, or null when it is malformed.
export function decodeJwt(token: string): Claims | null {
  const part = token.replace(/\s+/g, '').split('.')[1]
  if (!part) return null
  try {
    const b64 = part.replace(/-/g, '+').replace(/_/g, '/')
    const bin = atob(b64.padEnd(Math.ceil(b64.length / 4) * 4, '='))
    return JSON.parse(new TextDecoder().decode(Uint8Array.from(bin, (c) => c.charCodeAt(0)))) as Claims
  } catch {
    return null
  }
}

// isAdmin reports whether the claims carry the hub admin role.
export function isAdmin(c: Claims | null, role = 'vhub-admin'): boolean {
  return Array.isArray(c?.roles) && c.roles.includes(role)
}
