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
  provider: string
  lab?: string
  context_length: number
  output_limit: number
  modalities: string
  is_free: number
  fetched_at: number
  provider_npm?: string
}

export interface RequestLogItem {
  id: number
  timestamp: number
  model: string
  proxy_url: string
  status_code: number
  duration_ms: number
  prompt_tokens: number
  completion_tokens: number
  cache_read_tokens: number
  error_message: string
  is_stream: number
  session_id: string
  request_body: string
  response_body: string
}

export interface StatsData {
  total_requests: number
  success_rate: number
  total_tokens: number
  total_prompt_tokens: number
  total_completion_tokens: number
  active_proxies_count: number
  total_proxies_count: number
  model_usage: Record<string, number>
}

export interface OpenCodeStatus {
  config_file_exists: boolean
  config_path: string
  is_linked: boolean
  synced_model_count: number
  synced_models: string[]
}

const API_BASE = ''

export async function getStats(): Promise<StatsData> {
  const res = await fetch(`${API_BASE}/api/stats`)
  if (!res.ok) throw new Error('Failed to fetch stats')
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
