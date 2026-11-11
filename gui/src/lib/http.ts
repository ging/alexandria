// Fetch wrapper shared by the hub and client API modules: JSON handling, bearer injection and problem+json errors.
// Every non-2xx response becomes an ApiError carrying the RFC 9457 problem document of the Alexandria F5 catalog.
import { accessToken } from '@/lib/oidc'

// Problem is the problem+json document returned by both binaries.
export interface Problem {
  type?: string
  title: string
  status: number
  detail?: string
  code?: string
  severity?: string
  path?: string
  line?: number
  findings?: Problem[]
}

// ApiError is thrown for every failed request, including network failures (status 0).
export class ApiError extends Error {
  readonly status: number
  readonly problem: Problem
  constructor(problem: Problem) {
    super(problem.detail || problem.title)
    this.name = 'ApiError'
    this.status = problem.status
    this.problem = problem
  }
}

// RequestOptions describes one API call.
export interface RequestOptions {
  method?: string
  body?: unknown
  raw?: string
  contentType?: string
  ifMatch?: string
  auth?: boolean
  signal?: AbortSignal
}

// ApiResponse pairs a parsed body with the response metadata the UI needs.
export interface ApiResponse<T> {
  data: T
  status: number
  etag: string | null
  headers: Headers
  text: string
  url: string
}

// request performs one call and parses JSON bodies, throwing ApiError on failure.
export async function request<T = unknown>(url: string, o: RequestOptions = {}): Promise<ApiResponse<T>> {
  const headers: Record<string, string> = { Accept: 'application/ld+json, application/json, */*' }
  let body: string | undefined
  if (o.raw !== undefined) {
    body = o.raw
    headers['Content-Type'] = o.contentType ?? 'application/json'
  } else if (o.body !== undefined) {
    body = JSON.stringify(o.body)
    headers['Content-Type'] = 'application/json'
  }
  if (o.ifMatch) headers['If-Match'] = o.ifMatch
  const token = o.auth ? await accessToken() : undefined
  if (token) headers.Authorization = `Bearer ${token}`

  let res: Response
  try {
    res = await fetch(url, { method: o.method ?? 'GET', headers, body, signal: o.signal })
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') throw e
    throw new ApiError({ title: 'Unreachable', status: 0, detail: 'The service did not answer. Check that it is running and that the proxy target is correct.' })
  }
  const text = await res.text()
  const type = res.headers.get('Content-Type') ?? ''
  const isJSON = type.includes('json')
  if (!res.ok) {
    let problem: Problem = { title: res.statusText || 'Request failed', status: res.status, detail: text.slice(0, 400) || undefined }
    if (isJSON) {
      try {
        problem = { ...(JSON.parse(text) as Problem), status: res.status }
      } catch {
        // Keep the generic problem when the body is not valid JSON.
      }
    }
    // Vite answers 500/502 with an empty or HTML body when the upstream is down.
    if ((res.status === 502 || res.status === 500) && !isJSON) {
      problem = { title: 'Unreachable', status: 0, detail: 'The upstream service is not reachable through the proxy.' }
    }
    throw new ApiError(problem)
  }
  let data: unknown = text
  if (isJSON && text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = text
    }
  }
  return { data: data as T, status: res.status, etag: res.headers.get('ETag'), headers: res.headers, text, url: res.url }
}

// asApiError normalizes unknown thrown values into an ApiError.
export function asApiError(e: unknown): ApiError {
  if (e instanceof ApiError) return e
  return new ApiError({ title: 'Unexpected error', status: 0, detail: e instanceof Error ? e.message : String(e) })
}
