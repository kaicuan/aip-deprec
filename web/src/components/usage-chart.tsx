import { useMemo, useState } from "react"
import { BotMessageSquare, Coins, Layers } from "lucide-react"
import {
  Bar,
  BarChart,
  CartesianGrid,
  XAxis,
  YAxis,
} from "recharts"

import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  type ChartConfig,
} from "@/components/ui/chart"
import type { TimeBucketPoint } from "@/lib/api"

interface UsageChartProps {
  timeBuckets: TimeBucketPoint[]
  totalRequests: number
  range: "today" | "weekly" | "monthly"
  formatCompact: (num: number) => string
}

const tokensConfig = {
  fresh_prompt_tokens: {
    label: "Input (Uncached)",
    color: "var(--chart-1)",
  },
  cached_tokens: {
    label: "Cached",
    color: "var(--chart-2)",
  },
  visible_completion_tokens: {
    label: "Output",
    color: "var(--chart-3)",
  },
  reasoning_tokens: {
    label: "Reasoning",
    color: "var(--chart-4)",
  },
} satisfies ChartConfig

const requestsConfig = {
  success: {
    label: "Success",
    color: "var(--success)",
  },
  rate_limited: {
    label: "Rate Limited",
    color: "var(--warning)",
  },
  errors: {
    label: "Errors",
    color: "var(--destructive)",
  },
} satisfies ChartConfig

export function UsageChart({
  timeBuckets,
  totalRequests,
  range,
  formatCompact,
}: UsageChartProps) {
  const [mode, setMode] = useState<"tokens" | "requests">("tokens")

  const rangeLabel =
    range === "today" ? "Today (UTC)" : range === "weekly" ? "Weekly (UTC)" : "Monthly (UTC)"

  const formatXAxisTick = (val: string) => {
    if (range === "weekly" && val.includes(" ")) {
      const [day, time] = val.split(" ")
      return time === "00:00" ? day : "12p"
    }
    return val
  }

  const enrichedBuckets = useMemo(() => {
    return timeBuckets.map((pt) => {
      const cached = pt.cached_tokens ?? 0
      const reasoning = pt.reasoning_tokens ?? 0
      return {
        ...pt,
        fresh_prompt_tokens: Math.max(0, pt.prompt_tokens - cached),
        visible_completion_tokens: Math.max(0, pt.completion_tokens - reasoning),
      }
    })
  }, [timeBuckets])

  const activeConfig = mode === "tokens" ? tokensConfig : requestsConfig

  return (
    <Card className="overflow-hidden">
      <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between pb-3">
        <div>
          <CardTitle className="text-base font-semibold flex items-center gap-2">
            {mode === "tokens" ? (
              <Coins className="size-4 text-amber-500" />
            ) : (
              <BotMessageSquare className="size-4 text-emerald-500" />
            )}
            Usage Activity
          </CardTitle>
          <CardDescription>
            {mode === "tokens"
              ? `Token throughput, cache absorption, and output balance across ${rangeLabel}.`
              : `Request volume, 429 rate limit absorption, and error count across ${rangeLabel}.`}
          </CardDescription>
        </div>

        <CardAction className="flex items-center gap-3">
          {/* Mode Switcher: Tokens | Requests */}
          <Tabs
            value={mode}
            onValueChange={(val: string | null) => {
              if (val === "tokens" || val === "requests") {
                setMode(val)
              }
            }}
          >
            <TabsList>
              <TabsTrigger value="tokens">
                <Coins className="size-3" />
                Tokens
              </TabsTrigger>
              <TabsTrigger value="requests">
                <BotMessageSquare className="size-3" />
                Requests
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </CardAction>
      </CardHeader>

      <CardContent className="pt-0 -px-2 sm:px-(--card-spacing)">
        {totalRequests === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-center text-muted-foreground">
            <Layers className="size-8 stroke-1 text-muted-foreground/50 mb-2" />
            <p className="text-sm font-medium">No traffic recorded for {rangeLabel}</p>
            <p className="text-xs text-muted-foreground mt-0.5">
              Make requests via the proxy gateway to populate live time-series telemetry.
            </p>
          </div>
        ) : (
          <ChartContainer config={activeConfig} className="aspect-auto h-[230px] w-full">
            <BarChart
              accessibilityLayer
              data={enrichedBuckets}
              margin={{ left: 8, right: 8, top: 8, bottom: 0 }}
            >
              <CartesianGrid vertical={false} strokeDasharray="3 3" className="stroke-border/40" />
              <XAxis
                dataKey="bucket"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                interval="preserveStart"
                minTickGap={32}
                tickFormatter={formatXAxisTick}
                className="text-[11px] font-mono"
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                domain={[0, "auto"]}
                allowDecimals={false}
                tickFormatter={formatCompact}
                className="text-[11px] font-mono"
              />
              <ChartTooltip
                cursor={{ fill: "var(--muted)/30" }}
                content={({ active, payload, label }) => {
                  if (!active || !payload?.length) return null
                  const pt = payload[0].payload as TimeBucketPoint & { fresh_prompt_tokens: number; visible_completion_tokens: number }

                  if (mode === "tokens") {
                    const total = pt.total_tokens || (pt.prompt_tokens + pt.completion_tokens)
                    return (
                      <div className="grid min-w-48 gap-1.5 rounded-xl bg-popover px-3 py-2 text-xs text-popover-foreground shadow-lg ring-1 ring-border">
                        <div className="font-semibold text-foreground">{label}</div>
                        <div className="grid gap-1">
                          <div className="flex items-center justify-between gap-3">
                            <span className="flex items-center gap-1.5 text-muted-foreground">
                              <span className="size-2 rounded-[2px]" style={{ backgroundColor: tokensConfig.fresh_prompt_tokens.color }} />
                              Input (Uncached)
                            </span>
                            <span className="font-mono font-medium text-foreground">
                              {pt.fresh_prompt_tokens.toLocaleString()}
                            </span>
                          </div>
                          <div className="flex items-center justify-between gap-3">
                            <span className="flex items-center gap-1.5 text-muted-foreground">
                              <span className="size-2 rounded-[2px]" style={{ backgroundColor: tokensConfig.cached_tokens.color }} />
                              Cached
                            </span>
                            <span className="font-mono font-medium text-foreground">
                              {(pt.cached_tokens ?? 0).toLocaleString()}
                            </span>
                          </div>
                          <div className="flex items-center justify-between gap-3">
                            <span className="flex items-center gap-1.5 text-muted-foreground">
                              <span className="size-2 rounded-[2px]" style={{ backgroundColor: tokensConfig.visible_completion_tokens.color }} />
                              Output
                            </span>
                            <span className="font-mono font-medium text-foreground">
                              {pt.visible_completion_tokens.toLocaleString()}
                            </span>
                          </div>
                          <div className="flex items-center justify-between gap-3">
                            <span className="flex items-center gap-1.5 text-muted-foreground">
                              <span className="size-2 rounded-[2px]" style={{ backgroundColor: tokensConfig.reasoning_tokens.color }} />
                              Reasoning
                            </span>
                            <span className="font-mono font-medium text-foreground">
                              {(pt.reasoning_tokens ?? 0).toLocaleString()}
                            </span>
                          </div>
                          <div className="flex items-center justify-between gap-3 pt-1 border-t border-border/60 text-[11px]">
                            <span className="font-medium text-foreground">Total</span>
                            <span className="font-mono font-bold text-foreground">
                              {total.toLocaleString()}
                            </span>
                          </div>
                        </div>
                      </div>
                    )
                  }

                  const totalReqs = pt.total_requests || (pt.success + pt.rate_limited + pt.errors)
                  return (
                    <div className="grid min-w-44 gap-1.5 rounded-xl bg-popover px-3 py-2 text-xs text-popover-foreground shadow-lg ring-1 ring-border">
                      <div className="font-semibold text-foreground">{label}</div>
                      <div className="grid gap-1">
                        <div className="flex items-center justify-between gap-3">
                          <span className="flex items-center gap-1.5 text-muted-foreground">
                            <span className="size-2 rounded-[2px]" style={{ backgroundColor: requestsConfig.success.color }} />
                            Success (2xx)
                          </span>
                          <span className="font-mono font-medium text-foreground">
                            {pt.success.toLocaleString()}
                          </span>
                        </div>
                        <div className="flex items-center justify-between gap-3">
                          <span className="flex items-center gap-1.5 text-muted-foreground">
                            <span className="size-2 rounded-[2px]" style={{ backgroundColor: requestsConfig.rate_limited.color }} />
                            Rate Limited (429)
                          </span>
                          <span className="font-mono font-medium text-foreground">
                            {pt.rate_limited.toLocaleString()}
                          </span>
                        </div>
                        <div className="flex items-center justify-between gap-3">
                          <span className="flex items-center gap-1.5 text-muted-foreground">
                            <span className="size-2 rounded-[2px]" style={{ backgroundColor: requestsConfig.errors.color }} />
                            Errors
                          </span>
                          <span className="font-mono font-medium text-foreground">
                            {pt.errors.toLocaleString()}
                          </span>
                        </div>
                        <div className="flex items-center justify-between gap-3 pt-1 border-t border-border/60 text-[11px]">
                          <span className="font-medium text-foreground">Total</span>
                          <span className="font-mono font-bold text-foreground">
                            {totalReqs.toLocaleString()}
                          </span>
                        </div>
                      </div>
                    </div>
                  )
                }}
              />
              <ChartLegend content={<ChartLegendContent />} itemSorter={null} />
              {mode === "tokens" ? (
                <>
                  <Bar
                    dataKey="fresh_prompt_tokens"
                    stackId="a"
                    fill="var(--color-fresh_prompt_tokens)"
                    radius={[0, 0, 0, 0]}
                  />
                  <Bar
                    dataKey="cached_tokens"
                    stackId="a"
                    fill="var(--color-cached_tokens)"
                    radius={[0, 0, 0, 0]}
                  />
                  <Bar
                    dataKey="visible_completion_tokens"
                    stackId="a"
                    fill="var(--color-visible_completion_tokens)"
                    radius={[0, 0, 0, 0]}
                  />
                  <Bar
                    dataKey="reasoning_tokens"
                    stackId="a"
                    fill="var(--color-reasoning_tokens)"
                    radius={[3, 3, 0, 0]}
                  />
                </>
              ) : (
                <>
                  <Bar
                    dataKey="success"
                    stackId="a"
                    fill="var(--color-success)"
                    radius={[0, 0, 0, 0]}
                  />
                  <Bar
                    dataKey="rate_limited"
                    stackId="a"
                    fill="var(--color-rate_limited)"
                    radius={[0, 0, 0, 0]}
                  />
                  <Bar
                    dataKey="errors"
                    stackId="a"
                    fill="var(--color-errors)"
                    radius={[3, 3, 0, 0]}
                  />
                </>
              )}
            </BarChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}
