import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import {
  ArrowUpRight,
  BotMessageSquare,
  Coins,
  Cpu,
  DollarSign,
  Gauge,
  Network,
  RefreshCw,
} from "lucide-react"
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { getUsageSummary, type UsageSummary } from "@/lib/api"
import { UsageChart } from "@/components/usage-chart"
import { DistributionChart } from "@/components/distribution-chart"

function formatCompact(num: number): string {
  if (num >= 1_000_000) {
    return `${(num / 1_000_000).toFixed(2).replace(/\.?0+$/, "")}M`
  }
  if (num >= 1_000) {
    return `${(num / 1_000).toFixed(1).replace(/\.?0+$/, "")}k`
  }
  return num.toLocaleString()
}

function cleanModelLabel(model: string): string {
  if (!model) return "Unknown"
  const parts = model.split("/")
  return parts.length > 1 ? parts.slice(1).join("/") : model
}

function cleanProxyLabel(url: string): string {
  if (!url || url === "direct") return "Direct"
  try {
    const parsed = new URL(url)
    return parsed.host
  } catch {
    const cleaned = url.replace(/^.*@/, "").replace(/^[a-z0-9]+:\/\//, "")
    return cleaned || url
  }
}

export function OverviewPage() {
  const [range, setRange] = useState<"today" | "weekly" | "monthly">("today")
  const [summary, setSummary] = useState<UsageSummary | null>(null)
  const [loading, setLoading] = useState(true)

  const loadData = async (targetRange: "today" | "weekly" | "monthly" = range) => {
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

  const handleRangeChange = (newRange: "today" | "weekly" | "monthly") => {
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
            Token telemetry, traffic velocity, rate limit resilience, and load distribution.
          </p>
        </div>
        <div className="flex items-center gap-2.5">
          {/* Range toggle: Today, Weekly, Monthly (resets on UTC) */}
          <Tabs
            value={range}
            onValueChange={(val) => handleRangeChange(val as typeof range)}
          >
            <TabsList>
              <TabsTrigger value="today">Today</TabsTrigger>
              <TabsTrigger value="weekly">Weekly</TabsTrigger>
              <TabsTrigger value="monthly">Monthly</TabsTrigger>
            </TabsList>
          </Tabs>

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
              <Coins className="size-4 text-amber-500" />
            </CardAction>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold tracking-tight text-foreground font-mono">
              {summary?.total_tokens.toLocaleString() ?? "0"}
            </div>
            <div className="text-[11px] text-muted-foreground mt-1 font-mono flex flex-wrap items-center gap-x-1.5 gap-y-0.5">
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
              <BotMessageSquare className="size-4 text-emerald-500" />
            </CardAction>
          </CardHeader>
          <CardContent className="flex items-end justify-between">
            <div>
              <div className="text-2xl font-bold tracking-tight text-foreground font-mono">
                {summary?.total_requests.toLocaleString() ?? "0"}
              </div>
              <p className="text-xs text-muted-foreground mt-1 flex items-center gap-1 font-mono">
                Avg {summary ? Math.round(summary.avg_tokens_per_request).toLocaleString() : 0} tokens/req
              </p>
            </div>
            <Button
              variant="ghost"
              size="icon-lg"
              render={<Link to="/usage" />}
              className="size-10 shrink-0"
              aria-label="View usage"
            >
              <ArrowUpRight className="size-5.5" />
            </Button>
          </CardContent>
        </Card>

        {/* Card 3: Average Latency (TTFT) */}
        <Card className="relative overflow-hidden transition-all hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Avg Latency
            </CardTitle>
            <CardAction>
              <Gauge className="size-4 text-blue-500" />
            </CardAction>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold tracking-tight text-foreground font-mono">
              {summary && ((summary.avg_ttft_ms && summary.avg_ttft_ms > 0) ? summary.avg_ttft_ms : summary.avg_latency_ms) > 0 ? (
                <>
                  {((summary.avg_ttft_ms && summary.avg_ttft_ms > 0)
                    ? summary.avg_ttft_ms
                    : summary.avg_latency_ms
                  ).toLocaleString()}{" "}
                  <span className="text-sm font-normal text-muted-foreground font-sans">
                    ms
                  </span>
                </>
              ) : (
                "—"
              )}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Time to first token (TTFT)
            </p>
          </CardContent>
        </Card>

        {/* Card 4: Estimated Savings */}
        <Card className="relative overflow-hidden transition-all hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Est. Savings
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

      {/* Chart 1: Usage Activity (Tokens & Requests Stacked Bar Chart) */}
      <UsageChart
        timeBuckets={summary?.time_buckets ?? []}
        totalRequests={summary?.total_requests ?? 0}
        range={range}
        formatCompact={formatCompact}
      />

      {/* Secondary Row: Model Distribution & Proxy Distribution */}
      <div className="grid gap-4 lg:grid-cols-2">
        {/* Chart 2: Model Distribution (Donut Active + Horizontal Bar) */}
        <DistributionChart
          title="Model Distribution"
          description="Traffic and token volume handled across upstream models."
          icon={<Cpu className="size-4 text-primary" />}
          requestsData={summary?.model_usage ?? {}}
          tokensData={summary?.model_tokens ?? {}}
          cleanLabel={cleanModelLabel}
          emptyMessage="Make requests via OpenCode CLI to see model breakdown."
          formatCompact={formatCompact}
        />

        {/* Chart 3: Proxy Distribution (Donut Active + Horizontal Bar) */}
        <DistributionChart
          title="Proxy Distribution"
          description="Traffic and token load balance across proxy pool nodes."
          icon={<Network className="size-4 text-blue-500" />}
          requestsData={summary?.proxy_usage ?? {}}
          tokensData={summary?.proxy_tokens ?? {}}
          cleanLabel={cleanProxyLabel}
          emptyMessage="Add upstream proxies in /proxies to see routing distribution."
          formatCompact={formatCompact}
        />
      </div>
    </div>
  )
}
