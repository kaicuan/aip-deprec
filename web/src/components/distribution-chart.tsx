import { useState } from "react"
import { BotMessageSquare, Coins, Layers } from "lucide-react"
import {
  Bar,
  BarChart,
  Cell,
  LabelList,
  Pie,
  PieChart,
  Sector,
  XAxis,
  YAxis,
} from "recharts"

import type { PieSectorShapeProps } from "recharts/types/polar/Pie"

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
  ChartTooltip,
  type ChartConfig,
} from "@/components/ui/chart"
import { cn } from "cn"

interface DistributionChartProps {
  title: string
  description: string
  icon: React.ReactNode
  requestsData: Record<string, number>
  tokensData: Record<string, number>
  cleanLabel?: (key: string) => string
  emptyMessage: string
  formatCompact: (num: number) => string
}

const PALETTE = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
  "var(--muted-foreground)",
]

export function DistributionChart({
  title,
  description,
  icon,
  requestsData,
  tokensData,
  cleanLabel,
  emptyMessage,
  formatCompact,
}: DistributionChartProps) {
  const [metric, setMetric] = useState<"tokens" | "requests">("tokens")

  const rawData = metric === "tokens" ? tokensData : requestsData

  // Filter & sort descending
  const sortedEntries = Object.entries(rawData || {})
    .filter(([, val]) => val > 0)
    .sort((a, b) => b[1] - a[1])

  const total = sortedEntries.reduce((sum, [, val]) => sum + val, 0)

  // Top 5 + grouped Other
  const top5 = sortedEntries.slice(0, 5)
  const remainder = sortedEntries.slice(5)
  const otherTotal = remainder.reduce((sum, [, val]) => sum + val, 0)

  const slices = top5.map(([rawName, value], idx) => {
    const name = cleanLabel ? cleanLabel(rawName) : rawName
    return {
      key: `slice_${idx}`,
      name,
      fullName: rawName,
      value,
      share: total > 0 ? (value / total) * 100 : 0,
      fill: PALETTE[idx % PALETTE.length],
    }
  })

  if (otherTotal > 0) {
    slices.push({
      key: "slice_other",
      name: `Other (${remainder.length})`,
      fullName: `${remainder.length} other items`,
      value: otherTotal,
      share: total > 0 ? (otherTotal / total) * 100 : 0,
      fill: PALETTE[5],
    })
  }

  // Generate dynamic chart config
  const chartConfig = slices.reduce<ChartConfig>(
    (acc, slice) => {
      acc[slice.key] = {
        label: slice.name,
        color: slice.fill,
      }
      return acc
    },
    {
      value: { label: metric === "tokens" ? "Tokens" : "Requests" },
    }
  )

  return (
    <Card className="flex flex-col">
      <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between pb-2">
        <div>
          <CardTitle className="text-base font-semibold flex items-center gap-2">
            {icon}
            {title}
          </CardTitle>
          <CardDescription>{description}</CardDescription>
        </div>

        <CardAction>
          {/* Toggle Tokens | Requests */}
          <Tabs
            value={metric}
            onValueChange={(val: string | null) => {
              if (val === "tokens" || val === "requests") {
                setMetric(val)
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

      <CardContent className="flex flex-1 flex-col justify-between">
        {slices.length === 0 ? (
          <div className="flex flex-1 flex-col items-center justify-center py-12 text-center text-muted-foreground">
            <Layers className="size-8 stroke-1 text-muted-foreground/50 mb-2" />
            <p className="text-sm font-medium">No traffic recorded</p>
            <p className="text-xs text-muted-foreground mt-0.5">{emptyMessage}</p>
          </div>
        ) : (
          <div className="space-y-4">
            {/* Visual 1: Donut with Active highest sector */}
            <ChartContainer
              config={chartConfig}
              className="mx-auto aspect-square max-h-[210px] w-full"
            >
              <PieChart>
                <ChartTooltip
                  cursor={false}
                  content={({ active, payload }) => {
                    if (!active || !payload?.length) return null
                    const item = payload[0].payload
                    return (
                      <div className="rounded-xl bg-popover px-3 py-2 text-xs text-popover-foreground shadow-lg ring-1 ring-foreground/10">
                        <div
                          className="font-semibold text-foreground truncate max-w-[220px]"
                          title={item.fullName}
                        >
                          {item.name}
                        </div>
                        <div className="flex items-center justify-between gap-4 mt-1 font-mono text-[11px]">
                          <span className="text-muted-foreground capitalize">{metric}:</span>
                          <span className="font-bold text-foreground">
                            {item.value.toLocaleString()} ({item.share.toFixed(1)}%)
                          </span>
                        </div>
                      </div>
                    )
                  }}
                />
                <Pie
                  data={slices}
                  dataKey="value"
                  nameKey="name"
                  innerRadius={55}
                  outerRadius={80}
                  stroke="none"
                  strokeWidth={0}
                  shape={({
                    index,
                    outerRadius = 0,
                    ...props
                  }: PieSectorShapeProps) =>
                    index === 0 ? (
                      <Sector {...props} outerRadius={outerRadius + 8} stroke="none" strokeWidth={0} />
                    ) : (
                      <Sector {...props} outerRadius={outerRadius} stroke="none" strokeWidth={0} />
                    )
                  }
                >
                  {slices.map((entry) => (
                    <Cell key={entry.key} fill={entry.fill} />
                  ))}
                </Pie>
              </PieChart>
            </ChartContainer>

            {/* Visual 2: Legend with count and share % */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {slices.map((slice) => (
                <div
                  key={slice.key}
                  className="flex items-center justify-between text-xs rounded-lg border border-border/50 bg-muted/20 px-2.5 py-1.5"
                >
                  <div className="flex items-center gap-2 min-w-0 pr-2">
                    <span
                      className="size-2.5 shrink-0 rounded-[2px]"
                      style={{ backgroundColor: slice.fill }}
                    />
                    <span
                      className="font-mono text-foreground truncate text-[11px]"
                      title={slice.fullName}
                    >
                      {slice.name}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5 shrink-0 font-mono text-[11px]">
                    <span className="text-foreground font-semibold">
                      {formatCompact(slice.value)}
                    </span>
                    <span className="text-muted-foreground">({slice.share.toFixed(1)}%)</span>
                  </div>
                </div>
              ))}
            </div>

            {/* Visual 3: Small BarChart - Horizontal below */}
            <div className="pt-3 border-t border-border/60">
              <div className="flex items-center justify-between text-[11px] text-muted-foreground mb-2">
                <span className="font-medium">Ranked Comparison</span>
                <span className="font-mono text-[10px] uppercase font-semibold text-foreground/80">
                  {metric}
                </span>
              </div>
              <ChartContainer
                config={chartConfig}
                className={cn(
                  "aspect-auto w-full",
                  slices.length > 4 ? "h-[145px]" : slices.length > 2 ? "h-[125px]" : "h-[95px]"
                )}
              >
                <BarChart
                  accessibilityLayer
                  data={slices}
                  layout="vertical"
                  margin={{ left: 0, right: 38, top: 0, bottom: 0 }}
                  barCategoryGap="16%"
                >
                  <XAxis
                    type="number"
                    dataKey="value"
                    hide
                    domain={[0, "dataMax"]}
                  />
                  <YAxis
                    dataKey="name"
                    type="category"
                    tickLine={false}
                    axisLine={false}
                    interval={0}
                    width={105}
                    tickMargin={4}
                    tickFormatter={(val) => {
                      const str = String(val)
                      return str.length > 16 ? `${str.slice(0, 15)}…` : str
                    }}
                    className="text-[11px] font-mono text-muted-foreground"
                  />
                  <ChartTooltip
                    cursor={{ fill: "var(--muted)/40" }}
                    content={({ active, payload }) => {
                      if (!active || !payload?.length) return null
                      const d = payload[0].payload
                      return (
                        <div className="rounded-lg bg-popover px-2.5 py-1.5 text-xs text-popover-foreground shadow ring-1 ring-foreground/10">
                          <span className="font-medium text-foreground">{d.name}</span>:{" "}
                          <span className="font-mono font-bold">
                            {d.value.toLocaleString()}
                          </span>{" "}
                          ({d.share.toFixed(1)}%)
                        </div>
                      )
                    }}
                  />
                  <Bar dataKey="value" radius={[0, 4, 4, 0]}>
                    {slices.map((entry) => (
                      <Cell key={`bar-${entry.key}`} fill={entry.fill} />
                    ))}
                    <LabelList
                      dataKey="value"
                      position="right"
                      offset={8}
                      className="fill-foreground font-mono text-[11px] font-medium"
                      formatter={(val: unknown) => formatCompact(Number(val) || 0)}
                    />
                  </Bar>
                </BarChart>
              </ChartContainer>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
