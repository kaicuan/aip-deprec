import { useEffect, useMemo, useState } from "react"
import {
  Brain,
  Check,
  Copy,
  Cpu,
  ExternalLink,
  Eye,
  Layers,
  Minus,
  Paperclip,
  Plug,
  Plus,
  RefreshCw,
  RotateCw,
  Sparkles,
  Wrench,
} from "lucide-react"
import { cn } from "cn"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { toast } from "@/components/ui/toast"
import { OpenCodeIntegrationCard } from "@/components/integration/opencode-integration-card"
import {
  fetchFreeModels,
  getModels,
  getOpenCodeStatus,
  linkOpenCode,
  unlinkOpenCode,
  type ModelItem,
  type OpenCodeStatus,
} from "@/lib/api"

interface ModelCatalogDiff {
  added: string[]
  removed: string[]
}

function diffModels(catalogIds: string[], syncedIds: string[]): ModelCatalogDiff {
  return {
    added: catalogIds.filter((id) => !syncedIds.includes(id)).sort(),
    removed: syncedIds.filter((id) => !catalogIds.includes(id)).sort(),
  }
}

function describeDiff(diff: ModelCatalogDiff): string {
  const parts: string[] = []
  if (diff.added.length > 0) parts.push(`${diff.added.length} added`)
  if (diff.removed.length > 0) parts.push(`${diff.removed.length} removed`)
  return parts.length > 0 ? parts.join(", ") : "no changes"
}

function normalizeStatus(status: OpenCodeStatus | null): OpenCodeStatus | null {
  if (!status) return null
  return { ...status, synced_models: [...(status.synced_models || [])].sort() }
}

function parseLimit(limitStr?: string): { context: number; output: number } {
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

function formatLimit(num: number) {
  if (num >= 1000000) return `${(num / 1000000).toFixed(1).replace(/\.0$/, "")}M`
  if (num >= 1000) return `${Math.round(num / 1000)}k`
  return `${num}`
}

type SyncState = "unlinked" | "checking" | "synced" | "stale"

const SYNC_LABELS: Record<SyncState, string> = {
  unlinked: "Not Linked",
  checking: "Checking…",
  synced: "Synced",
  stale: "Sync",
}

function SyncModelsButton({
  state,
  pendingCount,
  onSync,
}: {
  state: SyncState
  pendingCount: number
  onSync: () => void
}) {
  return (
    <Button
      variant="outline"
      size="sm"
      onClick={onSync}
      disabled={state !== "stale"}
      title={
        state === "unlinked"
          ? "Link OpenCode first to sync models"
          : state === "stale"
            ? `Apply ${pendingCount} pending change${pendingCount === 1 ? "" : "s"} to your OpenCode config`
            : undefined
      }
      className={cn(
        "h-8 shrink-0",
        state === "synced" &&
          "border-success/40 bg-success/10 text-success hover:bg-success/15 hover:text-success",
        state === "stale" &&
          "border-primary/50 bg-primary/10 text-primary hover:bg-primary/15 hover:text-primary"
      )}
    >
      {state === "checking" && <RefreshCw className="size-3.5 animate-spin" />}
      {state === "synced" && <Check className="size-3.5" />}
      {state === "stale" && <RotateCw className="size-3.5" />}
      {state === "unlinked" && <Plug className="size-3.5" />}
      {SYNC_LABELS[state]}
      {state === "stale" && (
        <span className="ml-0.5 rounded-full bg-primary px-1.5 text-[10px] font-semibold leading-4 text-primary-foreground tabular-nums">
          {pendingCount}
        </span>
      )}
    </Button>
  )
}

export function IntegrationPage() {
  const [status, setStatus] = useState<OpenCodeStatus | null>(null)
  const [models, setModels] = useState<ModelItem[]>([])
  const [copiedModelId, setCopiedModelId] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [fetchingModels, setFetchingModels] = useState(false)
  const [linking, setLinking] = useState(false)
  const [unlinking, setUnlinking] = useState(false)
  const [syncDialog, setSyncDialog] = useState<ModelCatalogDiff | null>(null)

  const sortModels = (list: ModelItem[]) => [...list].sort((a, b) => a.id.localeCompare(b.id))

  const isLinked = !!status?.is_linked
  const changes = useMemo(
    () => diffModels(models.map((m) => m.id), status?.synced_models || []),
    [models, status?.synced_models]
  )
  const changeCount = changes.added.length + changes.removed.length
  const pendingAdded = syncDialog?.added ?? []
  const pendingRemoved = syncDialog?.removed ?? []

  const syncState: SyncState = loading || fetchingModels || linking
    ? "checking"
    : !isLinked
      ? "unlinked"
      : changeCount > 0
        ? "stale"
        : "synced"

  const handleCopyModelId = (id: string, e?: React.MouseEvent) => {
    e?.stopPropagation()
    navigator.clipboard.writeText(id)
    setCopiedModelId(id)
    setTimeout(() => {
      setCopiedModelId((curr) => (curr === id ? null : curr))
    }, 1500)
  }

  const loadData = async () => {
    try {
      setLoading(true)
      const [statusResult, modelsResult] = await Promise.allSettled([
        getOpenCodeStatus(),
        getModels(),
      ])

      if (statusResult.status === "fulfilled") {
        setStatus(normalizeStatus(statusResult.value))
      } else {
        setStatus(null)
        console.error("Failed to load integration status:", statusResult.reason)
      }

      if (modelsResult.status === "fulfilled") {
        setModels(sortModels(modelsResult.value))
      } else {
        console.error("Failed to load models:", modelsResult.reason)
      }
    } catch (err) {
      console.error("Failed to load integration data:", err)
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
      const previousModelIds = models.map((m) => m.id)
      const newModels = sortModels(await fetchFreeModels())
      setModels(newModels)

      const statusRes = normalizeStatus(await getOpenCodeStatus())
      setStatus(statusRes)

      const catalogAdded = newModels.filter((m) => !previousModelIds.includes(m.id)).length
      const catalogRemoved = previousModelIds.filter(
        (id) => !newModels.some((m) => m.id === id)
      ).length
      const catalogChanged = catalogAdded > 0 || catalogRemoved > 0

      if (statusRes?.is_linked) {
        const pending = diffModels(newModels.map((m) => m.id), statusRes.synced_models)
        const pendingTotal = pending.added.length + pending.removed.length

        if (pendingTotal > 0) {
          toast.add({
            title: "Catalog Updated",
            description: `OpenCode is out of date (${describeDiff(pending)}). Press Sync to push the changes over.`,
            type: "info",
          })
          return
        }
      }

      if (!catalogChanged && previousModelIds.length > 0) {
        toast.add({
          title: "No Changes Detected",
          description: "Model catalog is already up to date.",
          type: "info",
        })
      } else {
        toast.add({
          title: "Catalog Updated",
          description:
            previousModelIds.length > 0
              ? `Added ${catalogAdded}, removed ${catalogRemoved}.`
              : `${newModels.length} free models discovered.`,
          type: "success",
        })
      }
    } catch (err: any) {
      toast.add({
        title: "Fetch Failed",
        description: err.message || "Failed to fetch model catalog.",
        type: "error",
      })
    } finally {
      setFetchingModels(false)
    }
  }

  const handleLink = async () => {
    try {
      setLinking(true)
      setStatus(normalizeStatus(await linkOpenCode()))
      toast.add({
        title: "OpenCode Linked",
        description:
          "The aip provider is registered. Restart OpenCode to load it into a running session.",
        type: "success",
      })
    } catch (err: any) {
      toast.add({
        title: "Link Failed",
        description: err.message || "Failed to communicate with OpenCode configuration.",
        type: "error",
      })
    } finally {
      setLinking(false)
    }
  }

  const openSyncReview = () => {
    if (changeCount === 0) return
    setSyncDialog(changes)
  }

  const handleSync = async () => {
    try {
      setLinking(true)
      const applied = diffModels(models.map((m) => m.id), status?.synced_models || [])
      setStatus(normalizeStatus(await linkOpenCode()))
      toast.add({
        title: "Models Synced",
        description: `${describeDiff(applied)} written to your OpenCode config. Restart OpenCode to apply.`,
        type: "success",
      })
    } catch (err: any) {
      toast.add({
        title: "Sync Failed",
        description: err.message || "Failed to write to your OpenCode configuration.",
        type: "error",
      })
    } finally {
      setLinking(false)
    }
  }

  const handleUnlink = async () => {
    try {
      setUnlinking(true)
      setStatus(normalizeStatus(await unlinkOpenCode()))
      toast.add({
        title: "OpenCode Unlinked",
        description: "The aip provider was removed. Other providers were left untouched.",
        type: "success",
      })
      return true
    } catch (err: any) {
      toast.add({
        title: "Unlink Failed",
        description: err.message || "Failed to update your OpenCode configuration.",
        type: "error",
      })
      return false
    } finally {
      setUnlinking(false)
    }
  }

  return (
    <div className="flex flex-1 flex-col gap-6">
      {/* Page Header */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            Integration &amp; Models
          </h1>
          <p className="text-sm text-muted-foreground">
            Connect AIP to OpenCode, then manage the free upstream models it exposes.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={loadData}
            disabled={loading}
          >
            <RefreshCw className={cn("size-3.5 mr-1.5", loading && "animate-spin")} />
            Refresh
          </Button>
        </div>
      </div>

      {/* OpenCode Integration */}
      <OpenCodeIntegrationCard
        status={status}
        loading={loading}
        pending={linking || unlinking || fetchingModels}
        linking={linking}
        unlinking={unlinking}
        models={models}
        onLink={handleLink}
        onUnlink={handleUnlink}
        onRetry={loadData}
      />

      {/* Model Catalog Section */}
      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="flex items-center gap-2">
              <CardTitle className="text-base font-semibold flex items-center gap-2">
                <Sparkles className="size-4 text-amber-500" />
                Available Models
              </CardTitle>
              <Badge variant="outline" className="font-mono text-xs">
                {models.length}
              </Badge>
            </div>
            <CardDescription className="text-xs">
              Synchronized from the{" "}
              <a
                href="https://models.dev"
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-0.5 font-medium text-foreground underline underline-offset-3 hover:text-primary transition-colors"
              >
                models.dev
                <ExternalLink className="size-2.5" />
              </a>{" "}
              upstream catalog for OpenCode.
            </CardDescription>
          </div>
          <div className="flex shrink-0 items-center gap-2 self-start sm:self-auto">
            <SyncModelsButton
              state={syncState}
              pendingCount={changeCount}
              onSync={openSyncReview}
            />
            <Button
              variant="outline"
              size="sm"
              onClick={handleFetchModels}
              disabled={fetchingModels}
              className="h-8"
            >
              <RefreshCw className={`size-3.5 ${fetchingModels ? "animate-spin" : ""}`} />
              {fetchingModels ? "Fetching..." : "Fetch Upstream"}
            </Button>
          </div>
        </CardHeader>

        <CardContent>
          {models.length > 0 ? (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
              {models.map((model) => {
                const limit = parseLimit(model.limit)
                const isLinked = status?.synced_models?.includes(model.id)
                const hasVision = model.modalities?.toLowerCase().includes("image")
                const isCopied = copiedModelId === model.id

                return (
                  <div
                    key={model.id}
                    className={cn(
                      "group flex justify-between overflow-hidden rounded-xl border bg-card transition-all duration-200 hover:shadow-xs",
                      isLinked
                        ? "border-emerald-500/30 hover:border-emerald-500/60"
                        : "border-border hover:border-primary/40"
                    )}
                  >
                    <div className="flex flex-1 flex-col justify-between p-4 min-w-0">
                      <div>
                        {/* Top Header: Logo, Name, Model Slug */}
                        <div className="flex items-start gap-3 min-w-0">
                          <div className="size-9 rounded-lg bg-muted/60 p-1.5 flex items-center justify-center border border-border shrink-0 mt-0.5">
                            <img
                              src={`/logos/${model.lab || "opencode"}.svg`}
                              alt={model.lab || model.name}
                              className="size-5 object-contain dark:invert"
                              onError={(e) => {
                                ;(e.currentTarget as HTMLImageElement).src = "/logos/opencode.svg"
                              }}
                            />
                          </div>
                          <div className="min-w-0">
                            <h3
                              className="font-semibold text-sm text-foreground tracking-tight truncate leading-snug"
                              title={model.name}
                            >
                              {model.name}
                            </h3>
                            <button
                              type="button"
                              onClick={(e) => handleCopyModelId(model.id, e)}
                              title="Click to copy model ID"
                              className="group/slug inline-flex items-center gap-1.5 mt-0.5 text-xs font-mono text-muted-foreground hover:text-foreground transition-colors cursor-pointer max-w-full text-left"
                            >
                              <span className="truncate select-all">
                                {model.id}
                              </span>
                              <span className="shrink-0 text-muted-foreground/60 group-hover/slug:text-foreground transition-colors">
                                {isCopied ? (
                                  <Check className="size-3 text-emerald-500" />
                                ) : (
                                  <Copy className="size-3 opacity-50 group-hover/slug:opacity-100 transition-opacity" />
                                )}
                              </span>
                            </button>
                          </div>
                        </div>

                        {/* Capabilities Row */}
                        <div className="flex flex-wrap items-center gap-1.5 mt-3">
                          {model.reasoning && (
                            <Badge
                              variant="outline"
                              className="text-[10px] font-medium gap-1 px-1.5 py-0 h-4.5 border-amber-500/30 text-amber-600 dark:text-amber-400 bg-amber-500/10"
                            >
                              <Brain className="size-2.5" />
                              Reasoning
                            </Badge>
                          )}
                          {model.tool_call && (
                            <Badge
                              variant="outline"
                              className="text-[10px] font-medium gap-1 px-1.5 py-0 h-4.5 border-blue-500/30 text-blue-600 dark:text-blue-400 bg-blue-500/10"
                            >
                              <Wrench className="size-2.5" />
                              Tools
                            </Badge>
                          )}
                          {hasVision && (
                            <Badge
                              variant="outline"
                              className="text-[10px] font-medium gap-1 px-1.5 py-0 h-4.5 border-purple-500/30 text-purple-600 dark:text-purple-400 bg-purple-500/10"
                            >
                              <Eye className="size-2.5" />
                              Vision
                            </Badge>
                          )}
                          {model.attachment && (
                            <Badge
                              variant="outline"
                              className="text-[10px] font-medium gap-1 px-1.5 py-0 h-4.5 border-border text-muted-foreground bg-muted/40"
                            >
                              <Paperclip className="size-2.5" />
                              Files
                            </Badge>
                          )}
                          {!model.reasoning && !model.tool_call && !hasVision && !model.attachment && (
                            <span className="text-[10px] font-mono text-muted-foreground/60 py-0.5">
                              General text
                            </span>
                          )}
                        </div>
                      </div>

                      {/* Card Footer: Context Limits */}
                      <div className="mt-3.5 pt-2.5 border-t border-border flex items-center gap-1.5 font-mono text-[11px] text-muted-foreground">
                        <Layers className="size-3 text-muted-foreground shrink-0" />
                        <span>{formatLimit(limit.context)} ctx</span>
                        {limit.output > 0 && (
                          <span className="text-muted-foreground/60">• {formatLimit(limit.output)} out</span>
                        )}
                      </div>
                    </div>

                    {/* Full-height vertical Linked column */}
                    {isLinked && (
                      <div
                        title="Linked to OpenCode"
                        className="flex w-6 shrink-0 items-center justify-center border-l border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                      >
                        <span className="[writing-mode:vertical-rl] rotate-180 font-mono text-[9px] font-semibold uppercase tracking-widest select-none">
                          Linked
                        </span>
                      </div>
                    )}
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
                Click &quot;Fetch Upstream&quot; to fetch the latest catalog of free models from models.dev.
              </p>
              <Button
                variant="default"
                size="sm"
                onClick={handleFetchModels}
                disabled={fetchingModels}
              >
                <RefreshCw className={`size-3.5 mr-1.5 ${fetchingModels ? "animate-spin" : ""}`} />
                Fetch Upstream
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Sync Review Dialog */}
      <Dialog open={!!syncDialog} onOpenChange={(open) => !open && setSyncDialog(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <RotateCw className="size-4 text-primary" />
              Sync {changeCount} change{changeCount === 1 ? "" : "s"} to OpenCode?
            </DialogTitle>
            <DialogDescription className="text-xs">
              AIP rewrites the <span className="font-mono">provider.aip.models</span> block
              from your current catalog. Other providers and settings stay untouched.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 py-1">
            {pendingAdded.length > 0 && (
              <div className="space-y-1.5">
                <div className="flex items-center gap-1.5 text-xs font-medium text-success">
                  <Plus className="size-3.5" />
                  <span>Added ({pendingAdded.length})</span>
                </div>
                <div className="flex max-h-24 flex-wrap gap-1 overflow-y-auto rounded-lg border border-border bg-card p-2">
                  {pendingAdded.map((id) => (
                    <Badge
                      key={id}
                      variant="outline"
                      className="border-success/30 bg-success/10 font-mono text-[10px] text-success"
                    >
                      +{id}
                    </Badge>
                  ))}
                </div>
              </div>
            )}

            {pendingRemoved.length > 0 && (
              <div className="space-y-1.5">
                <div className="flex items-center gap-1.5 text-xs font-medium text-destructive">
                  <Minus className="size-3.5" />
                  <span>Removed ({pendingRemoved.length})</span>
                </div>
                <div className="flex max-h-24 flex-wrap gap-1 overflow-y-auto rounded-lg border border-border bg-card p-2">
                  {pendingRemoved.map((id) => (
                    <Badge
                      key={id}
                      variant="outline"
                      className="border-destructive/30 bg-destructive/10 font-mono text-[10px] text-destructive"
                    >
                      -{id}
                    </Badge>
                  ))}
                </div>
              </div>
            )}
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button variant="outline" size="sm" onClick={() => setSyncDialog(null)}>
              Cancel
            </Button>
            <Button
              size="sm"
              disabled={linking}
              onClick={() => {
                setSyncDialog(null)
                handleSync()
              }}
            >
              <RotateCw className="size-3.5" />
              Sync to OpenCode
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
