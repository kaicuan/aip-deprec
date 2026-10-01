import { useEffect, useState } from "react"
import {
  Check,
  CheckCircle2,
  Copy,
  Cpu,
  Layers,
  Link as LinkIcon,
  RefreshCw,
  Sparkles,
  Unlink,
} from "lucide-react"

import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  fetchFreeModels,
  getModels,
  getOpenCodeStatus,
  linkOpenCode,
  unlinkOpenCode,
  type ModelItem,
  type OpenCodeStatus,
} from "@/lib/api"

export function IntegrationPage() {
  const [status, setStatus] = useState<OpenCodeStatus | null>(null)
  const [snippet, setSnippet] = useState<string>("")
  const [models, setModels] = useState<ModelItem[]>([])
  const [loading, setLoading] = useState(true)
  const [fetchingModels, setFetchingModels] = useState(false)
  const [linking, setLinking] = useState(false)
  const [copied, setCopied] = useState(false)
  const [message, setMessage] = useState<string | null>(null)

  const loadData = async () => {
    try {
      setLoading(true)
      const [statusRes, modelsRes] = await Promise.all([
        getOpenCodeStatus(),
        getModels(),
      ])
      setStatus(statusRes.status)
      setSnippet(statusRes.snippet)
      setModels(modelsRes)
    } catch (err) {
      console.error("Failed to load integration status:", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const handleFetchModels = async () => {
    try {
      setFetchingModels(true)
      setMessage(null)
      const res = await fetchFreeModels()
      setModels(res.models)
      setMessage(
        `Discovered ${res.count} active free models!${
          res.auto_synced ? " Automatically updated OpenCode configuration." : ""
        }`
      )
      // Refresh status to update synced count
      const statusRes = await getOpenCodeStatus()
      setStatus(statusRes.status)
      setSnippet(statusRes.snippet)
    } catch (err: any) {
      setMessage(`Failed to fetch models: ${err.message}`)
    } finally {
      setFetchingModels(false)
    }
  }

  const handleLink = async () => {
    try {
      setLinking(true)
      setMessage(null)
      const res = await linkOpenCode()
      if (res.success) {
        setMessage(`Successfully linked AIP to OpenCode with ${res.synced_count} models!`)
      }
      await loadData()
    } catch (err: any) {
      setMessage(`Failed to link OpenCode: ${err.message}`)
    } finally {
      setLinking(false)
    }
  }

  const handleUnlink = async () => {
    try {
      setLinking(true)
      setMessage(null)
      await unlinkOpenCode()
      setMessage("Unlinked AIP provider from OpenCode configuration.")
      await loadData()
    } catch (err: any) {
      setMessage(`Failed to unlink OpenCode: ${err.message}`)
    } finally {
      setLinking(false)
    }
  }

  const handleCopySnippet = () => {
    if (!snippet) return
    navigator.clipboard.writeText(snippet)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="flex flex-1 flex-col gap-6">
      {/* Page Header */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            OpenCode Integration & Models
          </h1>
          <p className="text-sm text-muted-foreground">
            1-click synchronization with ~/.config/opencode/opencode.json and active free models catalog.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={loadData}
            disabled={loading}
          >
            <RefreshCw className={`size-3.5 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </div>
      </div>

      {message && (
        <div className="rounded-xl border border-border bg-muted/50 px-4 py-3 text-sm text-foreground flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="size-4 text-emerald-500 shrink-0" />
            <span>{message}</span>
          </div>
          <Button variant="ghost" size="xs" onClick={() => setMessage(null)}>
            Dismiss
          </Button>
        </div>
      )}

      {/* OpenCode Connection Status Card */}
      <Card>
        <CardHeader className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <CardTitle className="text-base font-semibold">
                OpenCode Configuration State
              </CardTitle>
              {status?.is_linked ? (
                <Badge variant="default" className="bg-emerald-600 hover:bg-emerald-600 text-white">
                  ● Linked
                </Badge>
              ) : (
                <Badge variant="secondary">
                  ○ Not Linked
                </Badge>
              )}
            </div>
            <CardDescription className="font-mono text-xs mt-1">
              Target: {status?.config_path || "~/.config/opencode/opencode.json"}
            </CardDescription>
          </div>
          <CardAction className="flex items-center gap-2">
            {status?.is_linked ? (
              <>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleLink}
                  disabled={linking}
                >
                  <RefreshCw className={`size-3.5 mr-1.5 ${linking ? "animate-spin" : ""}`} />
                  Sync Models ({models.length})
                </Button>
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={handleUnlink}
                  disabled={linking}
                >
                  <Unlink className="size-3.5 mr-1.5" />
                  Unlink
                </Button>
              </>
            ) : (
              <Button
                variant="default"
                size="sm"
                onClick={handleLink}
                disabled={linking}
              >
                <LinkIcon className="size-3.5 mr-1.5" />
                Link to OpenCode
              </Button>
            )}
          </CardAction>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Linking configures AIP as a custom provider inside OpenCode. Requests to free models will automatically route through AIP’s proxy pool on <code className="bg-muted px-1.5 py-0.5 rounded font-mono text-xs">http://localhost:9100/v1</code> with instant 429 failover.
          </p>

          <div className="rounded-xl border border-border bg-muted/30 p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground font-mono">
                Manual Setup Snippet (opencode.jsonc)
              </span>
              <Button
                variant="outline"
                size="xs"
                onClick={handleCopySnippet}
                className="text-xs"
              >
                {copied ? (
                  <>
                    <Check className="size-3 text-emerald-500 mr-1" />
                    Copied!
                  </>
                ) : (
                  <>
                    <Copy className="size-3 mr-1" />
                    Copy JSON
                  </>
                )}
              </Button>
            </div>
            <pre className="overflow-x-auto rounded-lg bg-card p-3 text-xs font-mono text-foreground border border-border max-h-48">
              {snippet || "// No models fetched yet"}
            </pre>
          </div>
        </CardContent>
      </Card>

      {/* Model Catalog Section */}
      <Card>
        <CardHeader className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <Sparkles className="size-4 text-amber-500" />
                Active Free OpenCode Models
              </CardTitle>
              <Badge variant="outline" className="font-mono text-xs">
                {models.length} models
              </Badge>
            </div>
            <CardDescription>
              Filtered directly from models.dev (zero cost input/output, active status).
            </CardDescription>
          </div>
          <CardAction>
            <Button
              variant="default"
              size="sm"
              onClick={handleFetchModels}
              disabled={fetchingModels}
            >
              <RefreshCw className={`size-3.5 mr-1.5 ${fetchingModels ? "animate-spin" : ""}`} />
              {fetchingModels ? "Fetching..." : "Fetch Models Now"}
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          {models.length > 0 ? (
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {models.map((model) => {
                const modalities = model.modalities ? model.modalities.split(",") : ["text"]
                const parseLimit = (limitStr?: string): { context: number; output: number } => {
                  if (!limitStr) return { context: 0, output: 0 }
                  try {
                    const parsed = JSON.parse(limitStr)
                    return {
                      context: typeof parsed.context === "number" ? parsed.context : 0,
                      output: typeof parsed.output === "number" ? parsed.output : 0,
                    }
                  } catch {
                    return { context: 0, output: 0 }
                  }
                }
                const limit = parseLimit(model.limit)

                const formatLimit = (num: number) => {
                  if (num >= 1000000) return `${(num / 1000000).toFixed(1).replace(/\.0$/, "")}M`
                  if (num >= 1000) return `${Math.round(num / 1000)}k`
                  return `${num}`
                }

                return (
                  <div
                    key={model.id}
                    className="flex flex-col justify-between rounded-xl border border-border bg-card p-4 transition-all hover:border-primary/50 hover:shadow-xs"
                  >
                    <div>
                      <div className="flex items-start justify-between gap-3">
                        <div className="flex items-center gap-2.5 min-w-0">
                          <div className="size-8 rounded-lg bg-muted/60 p-1.5 flex items-center justify-center border border-border shrink-0">
                            <img
                              src={`/logos/${model.lab || "opencode"}.svg`}
                              alt={model.lab || model.name}
                              className="size-4.5 object-contain dark:invert"
                              onError={(e) => {
                                (e.currentTarget as HTMLImageElement).src = "/logos/opencode.svg"
                              }}
                            />
                          </div>
                          <div className="min-w-0">
                            <span className="font-semibold text-sm text-foreground tracking-tight block truncate">
                              {model.name}
                            </span>
                            <div className="font-mono text-[11px] text-muted-foreground select-all truncate">
                              {model.id}
                            </div>
                          </div>
                        </div>
                        <Badge variant="secondary" className="font-mono text-[10px] uppercase shrink-0">
                          Free
                        </Badge>
                      </div>
                    </div>

                    <div className="mt-4 pt-3 border-t border-border flex items-center justify-between text-xs text-muted-foreground">
                      <div className="flex items-center gap-1 font-mono">
                        <Layers className="size-3 text-muted-foreground" />
                        <span>Ctx: {formatLimit(limit.context)}</span>
                        {limit.output > 0 && (
                          <span>/ Out: {formatLimit(limit.output)}</span>
                        )}
                      </div>
                      <div className="flex items-center gap-1">
                        {modalities.slice(0, 3).map((mod) => (
                          <Badge
                            key={mod}
                            variant="outline"
                            className="text-[10px] px-1 py-0 h-4 border-border font-mono capitalize"
                          >
                            {mod}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <Cpu className="size-10 stroke-1 text-muted-foreground/40 mb-3" />
              <p className="text-base font-medium text-foreground">
                No models discovered yet
              </p>
              <p className="text-xs text-muted-foreground max-w-sm mt-1 mb-4">
                Click &quot;Fetch Models Now&quot; to fetch the latest catalog of free models from models.dev.
              </p>
              <Button
                variant="default"
                size="sm"
                onClick={handleFetchModels}
                disabled={fetchingModels}
              >
                <RefreshCw className={`size-3.5 mr-1.5 ${fetchingModels ? "animate-spin" : ""}`} />
                Fetch Models Now
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
