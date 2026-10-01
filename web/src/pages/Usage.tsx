import { useEffect, useState } from "react"
import {
  ArrowLeft,
  ArrowRight,
  Check,
  ChevronRight,
  Copy,
  RefreshCw,
  Search,
  X,
} from "lucide-react"

import { Card, CardContent } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { getUsageLogs, getSingleUsage, type RequestLogItem } from "@/lib/api"

export function UsagePage() {
  const [logs, setLogs] = useState<RequestLogItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState("")
  const [offset, setOffset] = useState(0)
  const limit = 20

  // Inspection Drawer
  const [selectedLog, setSelectedLog] = useState<RequestLogItem | null>(null)
  const [isInspectOpen, setIsInspectOpen] = useState(false)
  const [copiedReq, setCopiedReq] = useState(false)
  const [copiedRes, setCopiedRes] = useState(false)

  const loadLogs = async (currentOffset = offset) => {
    try {
      setLoading(true)
      const data = await getUsageLogs(limit, currentOffset)
      setLogs(data.logs || [])
      setTotal(data.total || 0)
    } catch (err) {
      console.error("Failed to load logs:", err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadLogs(offset)
  }, [offset])

  const filteredLogs = logs.filter((l) => {
    if (!search) return true
    const term = search.toLowerCase()
    return (
      l.model.toLowerCase().includes(term) ||
      l.proxy_url.toLowerCase().includes(term) ||
      String(l.status_code).includes(term)
    )
  })

  const handleRowClick = async (log: RequestLogItem) => {
    setSelectedLog(log)
    setIsInspectOpen(true)
    if (!log.request_body && !log.response_body) {
      try {
        const full = await getSingleUsage(log.id)
        setSelectedLog(full)
      } catch (err) {
        console.error("Failed to load full request log:", err)
      }
    }
  }

  const handleCopyReq = () => {
    if (!selectedLog) return
    navigator.clipboard.writeText(selectedLog.request_body)
    setCopiedReq(true)
    setTimeout(() => setCopiedReq(false), 2000)
  }

  const handleCopyRes = () => {
    if (!selectedLog) return
    navigator.clipboard.writeText(selectedLog.response_body || selectedLog.error_message)
    setCopiedRes(true)
    setTimeout(() => setCopiedRes(false), 2000)
  }

  const formatJSON = (str: string) => {
    try {
      const parsed = JSON.parse(str)
      return JSON.stringify(parsed, null, 2)
    } catch {
      return str
    }
  }

  return (
    <div className="flex flex-1 flex-col gap-6">
      {/* Header */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            Usage & Request Audit Logs
          </h1>
          <p className="text-sm text-muted-foreground">
            Granular token telemetry, cache hit metrics, latency, and full request/response inspection.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => loadLogs(offset)}
            disabled={loading}
          >
            <RefreshCw className={`size-3.5 mr-1.5 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </div>
      </div>

      {/* Filter and Search Bar */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="size-4 absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Filter by model, status, proxy..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-8 text-xs font-mono"
          />
        </div>
        <span className="text-xs text-muted-foreground ml-auto font-mono">
          Showing {filteredLogs.length} of {total} logs
        </span>
      </div>

      {/* Table */}
      <Card>
        <CardContent className="p-0">
          <div className="rounded-xl overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[160px]">Timestamp</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Latency</TableHead>
                  <TableHead>Prompt</TableHead>
                  <TableHead>Completion</TableHead>
                  <TableHead>Cache Read</TableHead>
                  <TableHead>Proxy Egress</TableHead>
                  <TableHead className="w-[50px]"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredLogs.length > 0 ? (
                  filteredLogs.map((log) => {
                    const date = new Date(log.timestamp * 1000)
                    const isSuccess = log.status_code >= 200 && log.status_code < 300
                    const is429 = log.status_code === 429

                    return (
                      <TableRow
                        key={log.id}
                        className="cursor-pointer hover:bg-muted/70 transition-colors"
                        onClick={() => handleRowClick(log)}
                      >
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {date.toLocaleDateString([], { month: "short", day: "numeric" })}{" "}
                          {date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })}
                        </TableCell>
                        <TableCell className="font-mono text-xs font-medium text-foreground">
                          <code className="bg-muted px-1.5 py-0.5 rounded">
                            {log.model || "—"}
                          </code>
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={isSuccess ? "default" : is429 ? "secondary" : "destructive"}
                            className="font-mono text-[11px]"
                          >
                            {log.status_code}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {log.duration_ms}ms
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {log.prompt_tokens.toLocaleString()}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {log.completion_tokens.toLocaleString()}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {log.cache_read_tokens > 0 ? (
                            <Badge variant="outline" className="border-emerald-500/30 text-emerald-600 dark:text-emerald-400 font-mono text-[10px]">
                              {log.cache_read_tokens.toLocaleString()}
                            </Badge>
                          ) : (
                            "—"
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground max-w-[180px] truncate">
                          {log.proxy_url.replace(/:\/\/.*@/, "://***@")}
                        </TableCell>
                        <TableCell className="text-right">
                          <ChevronRight className="size-4 text-muted-foreground" />
                        </TableCell>
                      </TableRow>
                    )
                  })
                ) : (
                  <TableRow>
                    <TableCell colSpan={9} className="h-32 text-center text-muted-foreground">
                      No logs found.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Pagination */}
      <div className="flex items-center justify-between">
        <Button
          variant="outline"
          size="sm"
          onClick={() => setOffset(Math.max(0, offset - limit))}
          disabled={offset === 0}
        >
          <ArrowLeft className="size-3.5 mr-1" />
          Previous
        </Button>
        <span className="text-xs font-mono text-muted-foreground">
          Page {Math.floor(offset / limit) + 1} of {Math.max(1, Math.ceil(total / limit))}
        </span>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setOffset(offset + limit)}
          disabled={offset + limit >= total}
        >
          Next
          <ArrowRight className="size-3.5 ml-1" />
        </Button>
      </div>

      {/* Interactive Row Inspection Drawer */}
      <Drawer
        open={isInspectOpen}
        onOpenChange={setIsInspectOpen}
        swipeDirection="right"
      >
        <DrawerContent className="data-[swipe-axis=x]:sm:max-w-xl data-[swipe-axis=x]:sm:[--drawer-content-width:36rem]">
          {selectedLog && (
            <div className="flex h-full flex-col">
              <DrawerHeader className="border-b border-border pb-4">
                <div className="flex items-center justify-between">
                  <DrawerTitle className="font-mono text-base flex items-center gap-2">
                    Request #{selectedLog.id}
                  </DrawerTitle>
                  <div className="flex items-center gap-2">
                    <Badge
                      variant={
                        selectedLog.status_code >= 200 && selectedLog.status_code < 300
                          ? "default"
                          : selectedLog.status_code === 429
                          ? "secondary"
                          : "destructive"
                      }
                      className="font-mono text-xs"
                    >
                      HTTP {selectedLog.status_code}
                    </Badge>
                    <DrawerClose
                      render={
                        <Button
                          variant="ghost"
                          size="icon-sm"
                        />
                      }
                    >
                      <X className="size-4" />
                      <span className="sr-only">Close</span>
                    </DrawerClose>
                  </div>
                </div>
                <DrawerDescription className="font-mono text-xs">
                  {new Date(selectedLog.timestamp * 1000).toLocaleString()}
                </DrawerDescription>
              </DrawerHeader>

              <div className="flex-1 overflow-y-auto p-4 space-y-6">
                {/* Summary telemetry stats */}
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 rounded-xl border border-border bg-muted/40 p-3 font-mono text-xs">
                  <div>
                    <span className="text-muted-foreground block text-[10px] uppercase">Model</span>
                    <span className="font-semibold text-foreground truncate block">{selectedLog.model || "—"}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block text-[10px] uppercase">Duration</span>
                    <span className="font-semibold text-foreground">{selectedLog.duration_ms}ms</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block text-[10px] uppercase">Tokens</span>
                    <span className="font-semibold text-foreground">
                      {selectedLog.prompt_tokens + selectedLog.completion_tokens}
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block text-[10px] uppercase">Stream</span>
                    <span className="font-semibold text-foreground">{selectedLog.is_stream ? "Yes" : "No"}</span>
                  </div>
                </div>

                {/* Proxy Node and Session Affinity */}
                <div className="space-y-2 text-xs font-mono">
                  <div>
                    <span className="text-muted-foreground block text-[10px] uppercase mb-0.5">Proxy Egress Node</span>
                    <div className="rounded-lg bg-card p-2 border border-border break-all">
                      {selectedLog.proxy_url}
                    </div>
                  </div>
                  {selectedLog.session_id && (
                    <div>
                      <span className="text-muted-foreground block text-[10px] uppercase mb-0.5">Session Affinity ID</span>
                      <div className="rounded-lg bg-card p-2 border border-border break-all">
                        {selectedLog.session_id}
                      </div>
                    </div>
                  )}
                </div>

                {/* Request Payload */}
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold uppercase font-mono text-muted-foreground">
                      Request Payload
                    </span>
                    <Button variant="outline" size="xs" onClick={handleCopyReq}>
                      {copiedReq ? <Check className="size-3 text-emerald-500 mr-1" /> : <Copy className="size-3 mr-1" />}
                      {copiedReq ? "Copied" : "Copy"}
                    </Button>
                  </div>
                  <pre className="overflow-x-auto rounded-xl border border-border bg-card p-3 text-xs font-mono text-foreground max-h-56">
                    {selectedLog.request_body ? formatJSON(selectedLog.request_body) : "// No body recorded"}
                  </pre>
                </div>

                {/* Response Body or Error Message */}
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold uppercase font-mono text-muted-foreground">
                      {selectedLog.error_message ? "Response Error Details" : "Response Preview"}
                    </span>
                    <Button variant="outline" size="xs" onClick={handleCopyRes}>
                      {copiedRes ? <Check className="size-3 text-emerald-500 mr-1" /> : <Copy className="size-3 mr-1" />}
                      {copiedRes ? "Copied" : "Copy"}
                    </Button>
                  </div>
                  <pre className="overflow-x-auto rounded-xl border border-border bg-card p-3 text-xs font-mono text-foreground max-h-56">
                    {selectedLog.response_body
                      ? formatJSON(selectedLog.response_body)
                      : selectedLog.error_message || "// Empty response"}
                  </pre>
                </div>
              </div>
            </div>
          )}
        </DrawerContent>
      </Drawer>
    </div>
  )
}
