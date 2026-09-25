import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import {
  Activity,
  ArrowUpRight,
  CheckCircle2,
  Cpu,
  Layers,
  RefreshCw,
  Zap,
} from "lucide-react"

import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { getStats, type StatsData } from "@/lib/api"

export function OverviewPage() {
  const [stats, setStats] = useState<StatsData | null>(null)
  const [loading, setLoading] = useState(true)

  const loadData = async () => {
    try {
      setLoading(true)
      const statsData = await getStats()
      setStats(statsData)
    } catch (err) {
      console.error("Failed to load overview data:", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  return (
    <div className="flex flex-1 flex-col gap-6">
      {/* Top action header */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            Gateway Overview
          </h1>
          <p className="text-sm text-muted-foreground">
            Real-time proxy pooling, request metrics, and OpenCode failover status.
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

      {/* Metric Cards with Jump Links */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {/* Total Requests */}
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
              {stats?.total_requests.toLocaleString() ?? "0"}
            </div>
            <p className="text-xs text-muted-foreground mt-1 flex items-center gap-1">
              <Activity className="size-3 text-emerald-500" />
              Processed via gateway
            </p>
          </CardContent>
        </Card>

        {/* Success Rate */}
        <Card className="relative overflow-hidden transition-all hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Success Rate
            </CardTitle>
            <CardAction>
              <CheckCircle2 className="size-4 text-emerald-500" />
            </CardAction>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold tracking-tight text-foreground font-mono">
              {stats ? `${stats.success_rate.toFixed(1)}%` : "100.0%"}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Automatic 429 failover enabled
            </p>
          </CardContent>
        </Card>

        {/* Total Tokens */}
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
              {stats?.total_tokens.toLocaleString() ?? "0"}
            </div>
            <div className="text-[11px] text-muted-foreground mt-1 font-mono flex items-center gap-2">
              <span>In: {stats?.total_prompt_tokens.toLocaleString() ?? 0}</span>
              <span>•</span>
              <span>Out: {stats?.total_completion_tokens.toLocaleString() ?? 0}</span>
            </div>
          </CardContent>
        </Card>

        {/* Active Proxies */}
        <Card className="relative overflow-hidden transition-all hover:shadow-md">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Active Proxies
            </CardTitle>
            <CardAction>
              <Button
                variant="ghost"
                size="icon-xs"
                render={<Link to="/proxies" />}
                className="text-muted-foreground hover:text-foreground"
                aria-label="Manage proxies"
              >
                <ArrowUpRight className="size-4" />
              </Button>
            </CardAction>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold tracking-tight text-foreground font-mono">
              {stats?.active_proxies_count ?? 0}{" "}
              <span className="text-sm font-normal text-muted-foreground font-sans">
                / {stats?.total_proxies_count ?? 0} total
              </span>
            </div>
            <p className="text-xs text-muted-foreground mt-1 flex items-center gap-1.5">
              <span className="size-2 rounded-full bg-emerald-500 animate-pulse" />
              Ready in round-robin pool
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
            Relative distribution of requests handled across OpenCode free models.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {stats && Object.keys(stats.model_usage || {}).length > 0 ? (
            <div className="space-y-3">
              {Object.entries(stats.model_usage).map(([model, count]) => {
                const total = stats.total_requests || 1
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
              <p className="text-sm font-medium">No traffic recorded yet</p>
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
