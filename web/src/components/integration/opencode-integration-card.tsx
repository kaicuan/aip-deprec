import { useEffect, useMemo, useState } from "react"
import { AlertTriangle, Check, Copy, Plug, PlugZap, RefreshCw, Unlink } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
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
  DialogTrigger,
} from "@/components/ui/dialog"
import { toast } from "@/components/ui/toast"
import type { ModelItem, OpenCodeStatus } from "@/lib/api"
import { generateOpenCodeSnippet } from "@/lib/opencode"

type Phase = "loading" | "unavailable" | "unlinked" | "linked"

export interface OpenCodeIntegrationCardProps {
  status: OpenCodeStatus | null
  loading: boolean
  pending: boolean
  linking: boolean
  unlinking: boolean
  models: ModelItem[]
  onLink: () => Promise<void>
  onUnlink: () => Promise<boolean>
  onRetry: () => Promise<void>
}

function resolvePhase(status: OpenCodeStatus | null, loading: boolean): Phase {
  if (loading) return "loading"
  if (!status) return "unavailable"
  return status.is_linked ? "linked" : "unlinked"
}

function CopyButton({
  value,
  label,
  compact = false,
}: {
  value: string
  label: string
  compact?: boolean
}) {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) return
    const timer = setTimeout(() => setCopied(false), 1500)
    return () => clearTimeout(timer)
  }, [copied])

  return (
    <Button
      type="button"
      variant={compact ? "ghost" : "outline"}
      size={compact ? "icon-xs" : "sm"}
      aria-label={copied ? "Copied" : label}
      title={copied ? "Copied" : label}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(value)
          setCopied(true)
        } catch {
          toast.add({
            title: "Couldn't copy",
            description: "Select and copy the text manually.",
            type: "error",
          })
        }
      }}
    >
      {copied ? <Check data-icon="inline-start" /> : <Copy data-icon="inline-start" />}
      {!compact && (copied ? "Copied" : label)}
    </Button>
  )
}

function ConfigurationDialog({
  status,
  models,
  disabled,
}: {
  status: OpenCodeStatus | null
  models: ModelItem[]
  disabled: boolean
}) {
  const snippet = useMemo(() => generateOpenCodeSnippet(models), [models])
  const configPath = status?.config_path || "~/.config/opencode/opencode.json"

  return (
    <Dialog>
      <DialogTrigger
        render={<Button variant="outline" size="sm" className="w-full" disabled={disabled} />}
      >
        Set up manually
      </DialogTrigger>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Set up OpenCode manually</DialogTitle>
          <DialogDescription>
            AIP manages the config on the machine running its daemon. Use this snippet if
            OpenCode reads a different config file.
          </DialogDescription>
        </DialogHeader>

        <div className="flex min-w-0 flex-col gap-2">
          <p className="font-medium">Config file</p>
          <div className="flex items-start gap-2">
            <code className="min-w-0 break-all text-xs text-muted-foreground">{configPath}</code>
            <CopyButton value={configPath} label="Copy config path" compact />
          </div>
        </div>

        <div className="flex min-w-0 flex-col gap-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="font-medium">AIP provider snippet</p>
            {snippet && <CopyButton value={snippet} label="Copy snippet" />}
          </div>
          {snippet ? (
            <>
              <p className="text-sm text-muted-foreground">
                Merge the <code>provider.aip</code> entry into an existing config, or paste the
                whole snippet into a new file. Restart OpenCode after saving.
              </p>
              <pre
                tabIndex={0}
                aria-label="OpenCode configuration snippet"
                className="max-h-64 overflow-auto rounded-lg bg-muted p-4 text-xs"
              >
                <code>{snippet}</code>
              </pre>
            </>
          ) : (
            <p className="text-sm text-muted-foreground">
              Fetch the model catalog below to generate the provider snippet.
            </p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

function UnlinkDialog({
  configPath,
  syncedCount,
  pending,
  unlinking,
  onUnlink,
}: {
  configPath: string
  syncedCount: number
  pending: boolean
  unlinking: boolean
  onUnlink: () => Promise<boolean>
}) {
  const [open, setOpen] = useState(false)

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!pending) setOpen(nextOpen)
      }}
    >
      <DialogTrigger
        render={<Button variant="destructive" size="sm" className="w-full" disabled={pending} />}
      >
        <Unlink data-icon="inline-start" />
        Unlink
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Unlink AIP from OpenCode?</DialogTitle>
          <DialogDescription>
            This removes <code>provider.aip</code> from{" "}
            <code className="break-all">{configPath}</code>
            {syncedCount > 0 &&
              `, including its ${syncedCount} ${syncedCount === 1 ? "model" : "models"}`}
            . Other providers and settings stay as they are.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            variant="outline"
            size="sm"
            className="w-32"
            disabled={pending}
            onClick={() => setOpen(false)}
          >
            Keep linked
          </Button>
          <Button
            variant="destructive"
            size="sm"
            className="w-32"
            disabled={pending}
            onClick={async () => {
              if (await onUnlink()) setOpen(false)
            }}
          >
            {unlinking ? (
              <RefreshCw data-icon="inline-start" className="animate-spin" />
            ) : (
              <Unlink data-icon="inline-start" />
            )}
            {unlinking ? "Unlinking…" : "Unlink"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function OpenCodeIntegrationCard({
  status,
  loading,
  pending,
  linking,
  unlinking,
  models,
  onLink,
  onUnlink,
  onRetry,
}: OpenCodeIntegrationCardProps) {
  const phase = resolvePhase(status, loading)
  const hasModels = models.length > 0
  const syncedCount = status?.synced_models?.length ?? 0

  const state = {
    loading: {
      label: "Checking connection",
      icon: <RefreshCw data-icon="inline-start" className="animate-spin" aria-hidden="true" />,
      badgeClassName: "border-border bg-muted/50 text-muted-foreground",
    },
    unavailable: {
      label: "Status unavailable",
      icon: <AlertTriangle data-icon="inline-start" aria-hidden="true" />,
      badgeClassName: "border-destructive/30 bg-destructive/10 text-destructive",
    },
    unlinked: {
      label: "Not linked",
      icon: <Plug data-icon="inline-start" aria-hidden="true" />,
      badgeClassName: "border-warning/30 bg-warning/10 text-warning",
    },
    linked: {
      label: "Linked",
      icon: <Check data-icon="inline-start" aria-hidden="true" />,
      badgeClassName: "border-success/30 bg-success/10 text-success",
    },
  }[phase]

  const description = {
    loading: "Checking the OpenCode configuration…",
    unavailable: "AIP couldn't read the OpenCode config. Retry to check the link state.",
    unlinked: !hasModels
      ? "Fetch the model catalog below before linking AIP to OpenCode."
      : status?.config_file_exists
        ? "Link AIP to register its provider in your existing config."
        : "Link AIP to create the OpenCode config and register its provider.",
    linked: "Restart OpenCode if the provider doesn't appear in its model picker.",
  }[phase]

  return (
    <Card
      size="sm"
      className="w-full max-w-sm self-start"
      aria-labelledby="opencode-integration-title"
      aria-busy={loading || pending}
    >
      <CardHeader className="flex flex-row items-center gap-3">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-lg border border-border bg-muted">
          <img src="/logos/opencode.svg" alt="" className="size-5 dark:invert" />
        </div>
        <CardTitle id="opencode-integration-title" className="font-semibold">
          OpenCode
        </CardTitle>
      </CardHeader>

      <CardContent className="flex flex-col gap-3">
        <div className="flex items-center justify-between gap-3" role="status">
          <span className="text-sm text-muted-foreground">Status</span>
          <Badge
            variant="outline"
            className={`h-7 shrink-0 gap-1.5 rounded-lg px-2.5 font-semibold ${state.badgeClassName}`}
          >
            {state.icon}
            {state.label}
          </Badge>
        </div>

        <CardDescription className="leading-relaxed">{description}</CardDescription>
      </CardContent>

      <CardFooter className="flex flex-col gap-2">
        {phase === "linked" && status && (
          <UnlinkDialog
            configPath={status.config_path}
            syncedCount={syncedCount}
            pending={pending}
            unlinking={unlinking}
            onUnlink={onUnlink}
          />
        )}
        {phase === "unlinked" && hasModels && (
          <Button size="sm" className="w-full" disabled={pending} onClick={onLink}>
            {linking ? (
              <RefreshCw data-icon="inline-start" className="animate-spin" />
            ) : (
              <PlugZap data-icon="inline-start" />
            )}
            {linking ? "Linking…" : "Link AIP"}
          </Button>
        )}
        {phase === "unavailable" && (
          <Button
            variant="outline"
            size="sm"
            className="w-full"
            disabled={pending}
            onClick={onRetry}
          >
            <RefreshCw data-icon="inline-start" />
            Retry
          </Button>
        )}
        {(phase === "unlinked" || phase === "unavailable") && hasModels && (
          <ConfigurationDialog status={status} models={models} disabled={pending} />
        )}
      </CardFooter>
    </Card>
  )
}
