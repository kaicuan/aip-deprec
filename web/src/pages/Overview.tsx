import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import {
  ArrowUpRight,
  Cpu,
  DollarSign,
  Gauge,
  Layers,
  RefreshCw,
  Zap,
} from "lucide-react"
import { cn } from "cn"

import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { getUsageSummary, type UsageSummary } from "@/lib/api"

function formatCompact(num: number): string {
  if (num >= 1_000_000) {
    return `${(num / 1_000_000).toFixed(2).replace(/\.?0+$/, "")}M`
  }
  return num.toLocaleString()
}

export function OverviewPage() {
  const [range, setRange] = useState<"24h" | "7d" | "30d">("7d")
  const [summary, setSummary] = useState<UsageSummary | null>(null)
  const [loading, setLoading] = useState(true)

  const loadData = async (targetRange: "24h" | "7d" | "30d" = range) => {
    try {
      setLoading(true)
      const data = await getUsageSummary(targetRange)
      setSummary(data)
    } catch (err) {
      console.error("Failed to load overview data:", err)
    } finally {
      setLoading(false)
    }
  }

  const handleRangeChange = (newRange: "24h" | "7d" | "30d") => {
    setRange(newRange)
    loadData(newRange)
  }

  useEffect(() => {
    loadData()
  }, [])

  return (
    <div className="flex flex-1 flex-col gap-6">
      {/* Top action header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            Gateway Overview
          </h1>
          <p className="text-sm text-muted-foreground">
            Token telemetry, request velocity, estimated cost savings, and model traffic distribution.
          </p>
        </div>
        <div className="flex items-center gap-2.5">
          {/* Range toggle: 24H, 7D (default), 30D */}
          <div className="flex items-center rounded-xl border border-border bg-muted/50 p-1 text-muted-foreground">
            {(["24h", "7d", "30d"] as const).map((r) => (
              <Button
                key={r}
                variant={range === r ? "secondary" : "ghost"}
                size="xs"
                onClick={() => handleRangeChange(r)}
                className={cn(
                  "px-3 py-1 text-xs font-medium uppercase transition-all rounded-lg",
                  range === r
                    ? "bg-background text-foreground shadow-xs font-semibold"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {r}
              </Button>
            ))}
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={() => loadData(range)}
            disabled={loading}
          >
            <RefreshCw className={`size-3.5 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </div>
      </div>

      {/* 4 Summary Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {/* Card 1: Total Tokens */}
        <Card className="relative overflow-hidden transition-all hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total Tokens
            </CardTitle>
            <CardAction>
              <Zap className="size-4 text-amber-500" />
            </CardAction>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold tracking-tight text-foreground font-mono">
              {summary?.total_tokens.toLocaleString() ?? "0"}
            </div>
            <div className="text-[11px] text-muted-foreground mt-1 font-mono flex items-center gap-1.5 truncate">
              <span>
                In: {summary ? formatCompact(summary.total_prompt_tokens) : "0"} ({summary ? Math.round(summary.cache_hit_rate) : 0}%)
              </span>
              <span>•</span>
              <span>Out: {summary ? formatCompact(summary.total_completion_tokens) : "0"}</span>
            </div>
          </CardContent>
        </Card>

        {/* Card 2: Total Requests */}
        <Card className="relative overflow-hidden transition-all hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total Requests
            </CardTitle>
            <CardAction>
              <Button
                variant="ghost"
                size="icon-xs"
                render={<Link to="/usage" />}
                className="text-muted-foreground hover:text-foreground"
                aria-label="View usage"
              >
                <ArrowUpRight className="size-4" />
              </Button>
            </CardAction>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold tracking-tight text-foreground font-mono">
              {summary?.total_requests.toLocaleString() ?? "0"}
            </div>
            <p className="text-xs text-muted-foreground mt-1 flex items-center gap-1 font-mono">
              Avg {summary ? Math.round(summary.avg_tokens_per_request).toLocaleString() : 0} tokens/req
            </p>
          </CardContent>
        </Card>

        {/* Card 3: RPM & TPM */}
        <Card className="relative overflow-hidden transition-all hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              RPM & TPM
            </CardTitle>
            <CardAction>
              <Gauge className="size-4 text-blue-500" />
            </CardAction>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold tracking-tight text-foreground font-mono">
              {summary ? summary.rpm.toFixed(2) : "0.00"}{" "}
              <span className="text-sm font-normal text-muted-foreground font-sans">
                RPM
              </span>
            </div>
            <p className="text-xs text-muted-foreground mt-1 font-mono">
              {summary ? summary.tpm.toLocaleString(undefined, { maximumFractionDigits: 1 }) : "0"} TPM
            </p>
          </CardContent>
        </Card>

        {/* Card 4: Estimated Savings */}
        <Card className="relative overflow-hidden transition-all hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Estimated Savings
            </CardTitle>
            <CardAction>
              <DollarSign className="size-4 text-emerald-500" />
            </CardAction>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold tracking-tight text-foreground font-mono">
              ${summary ? summary.estimated_savings.toFixed(2) : "0.00"}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Saved vs standard API rates
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Model Distribution & Insights */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base font-semibold flex items-center gap-2">
            <Cpu className="size-4 text-primary" />
            Model Usage & Traffic Distribution
          </CardTitle>
          <CardDescription>
            Relative distribution of requests handled across OpenCode free models ({range.toUpperCase()}).
          </CardDescription>
        </CardHeader>
        <CardContent>
          {summary && Object.keys(summary.model_usage || {}).length > 0 ? (
            <div className="space-y-3">
              {Object.entries(summary.model_usage).map(([model, count]) => {
                const total = summary.total_requests || 1
                const percentage = Math.round((count / total) * 100)
                return (
                  <div key={model} className="space-y-1">
                    <div className="flex items-center justify-between text-xs font-mono">
                      <span className="font-medium text-foreground">{model}</span>
                      <span className="text-muted-foreground">
                        {count.toLocaleString()} reqs ({percentage}%)
                      </span>
                    </div>
                    <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                      <div
                        className="h-full bg-primary transition-all duration-500 rounded-full"
                        style={{ width: `${Math.max(percentage, 3)}%` }}
                      />
                    </div>
                  </div>
                )
              })}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center py-6 text-center text-muted-foreground">
              <Layers className="size-8 stroke-1 text-muted-foreground/50 mb-2" />
              <p className="text-sm font-medium">No traffic recorded for this period</p>
              <p className="text-xs text-muted-foreground mt-0.5">
                Start making requests from OpenCode CLI to see live model trends.
              </p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
