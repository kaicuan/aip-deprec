export interface ProxyItem {
  id: number
  raw_input: string
  url: string
  protocol: string
  host: string
  port: number
  username: string
  password?: string
  status: 'online' | 'rate_limited' | 'error'
  latency_ms: number
  egress_ip: string
  cooldown_until: number // unix seconds
  last_checked_at: number
  fail_count: number
  success_count: number
  created_at: number
}

export interface ModelItem {
  id: string
  name: string
  limit?: string
  modalities: string
  reasoning?: boolean
  tool_call?: boolean
  temperature?: boolean
  attachment?: boolean
  reasoning_options?: string
  provider?: string
  lab?: string
  commercial_rate?: string
  fetched_at: number
}

export interface RequestLogItem {
  id: number
  timestamp: number
  model: string
  proxy_url: string
  status_code: number
  duration_ms: number
  ttft_ms: number
  prompt_tokens: number
  completion_tokens: number
  cached_tokens: number
  reasoning_tokens: number
  error_message: string
  is_stream: number
  session_id: string
  request_body: string
  response_body: string
  estimated_savings: number
}

export interface TimeBucketPoint {
  bucket: string
  timestamp: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cached_tokens: number
  reasoning_tokens: number
  success: number
  rate_limited: number
  errors: number
  total_requests: number
  avg_duration_ms?: number
  avg_ttft_ms?: number
  avg_latency_ms: number
}

export interface UsageSummary {
  range: string
  range_minutes: number
  total_requests: number
  total_tokens: number
  total_prompt_tokens: number
  total_completion_tokens: number
  total_cached_tokens: number
  total_reasoning_tokens: number
  cache_hit_rate: number
  avg_tokens_per_request: number
  rpm: number
  tpm: number
  estimated_savings: number
  avg_duration_ms?: number
  avg_ttft_ms?: number
  avg_latency_ms: number
  model_usage: Record<string, number>
  model_tokens?: Record<string, number>
  proxy_usage?: Record<string, number>
  proxy_tokens?: Record<string, number>
  time_buckets: TimeBucketPoint[]
}

export interface ServerHealth {
  status: string
  uptime: string
  active_proxies: number
  total_proxies: number
}

export interface OpenCodeStatus {
  config_file_exists: boolean
  config_path: string
  is_linked: boolean
  synced_model_count: number
  synced_models: string[]
}

const API_BASE = ''

export async function getUsageSummary(range = 'weekly'): Promise<UsageSummary> {
  const res = await fetch(`${API_BASE}/api/usage/summary?range=${encodeURIComponent(range)}`)
  if (!res.ok) throw new Error('Failed to fetch usage summary')
  return res.json()
}

export async function getServerHealth(): Promise<ServerHealth> {
  const res = await fetch(`${API_BASE}/api/health`)
  if (!res.ok) throw new Error('Failed to fetch server health')
  return res.json()
}

export async function getProxies(): Promise<ProxyItem[]> {
  const res = await fetch(`${API_BASE}/api/proxies`)
  if (!res.ok) throw new Error('Failed to fetch proxies')
  return res.json()
}

export async function addProxies(input: string, protocol = 'http'): Promise<{ added_count: number; errors: string[] }> {
  const res = await fetch(`${API_BASE}/api/proxies`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ input, protocol }),
  })
  if (!res.ok) throw new Error('Failed to add proxies')
  return res.json()
}

export async function testProxies(ids?: number[]): Promise<any> {
  const res = await fetch(`${API_BASE}/api/proxies/test`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids: ids || [] }),
  })
  if (!res.ok) throw new Error('Failed to test proxies')
  return res.json()
}

export async function deleteProxies(ids: number[]): Promise<any> {
  const res = await fetch(`${API_BASE}/api/proxies`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids }),
  })
  if (!res.ok) throw new Error('Failed to delete proxies')
  return res.json()
}

export async function getModels(): Promise<ModelItem[]> {
  const res = await fetch(`${API_BASE}/api/models`)
  if (!res.ok) throw new Error('Failed to fetch models')
  return res.json()
}

export async function fetchFreeModels(): Promise<{ count: number; models: ModelItem[]; auto_synced: boolean }> {
  const res = await fetch(`${API_BASE}/api/models/fetch`, {
    method: 'POST',
  })
  if (!res.ok) throw new Error('Failed to fetch models catalog')
  return res.json()
}

export async function getOpenCodeStatus(): Promise<{ status: OpenCodeStatus; snippet: string }> {
  const res = await fetch(`${API_BASE}/api/opencode/status`)
  if (!res.ok) throw new Error('Failed to fetch OpenCode status')
  return res.json()
}

export async function linkOpenCode(): Promise<{ success: boolean; synced_count: number }> {
  const res = await fetch(`${API_BASE}/api/opencode/link`, {
    method: 'POST',
  })
  if (!res.ok) throw new Error('Failed to link OpenCode')
  return res.json()
}

export async function unlinkOpenCode(): Promise<{ success: boolean }> {
  const res = await fetch(`${API_BASE}/api/opencode/unlink`, {
    method: 'POST',
  })
  if (!res.ok) throw new Error('Failed to unlink OpenCode')
  return res.json()
}

export async function getUsageLogs(limit = 50, offset = 0): Promise<{ logs: RequestLogItem[]; total: number }> {
  const res = await fetch(`${API_BASE}/api/usage?limit=${limit}&offset=${offset}`)
  if (!res.ok) throw new Error('Failed to fetch usage logs')
  return res.json()
}

export async function getSingleUsage(id: number): Promise<RequestLogItem> {
  const res = await fetch(`${API_BASE}/api/usage/${id}`)
  if (!res.ok) throw new Error('Failed to fetch request log')
  return res.json()
}
