import { useEffect, useRef, useState } from "react"
import {
  Play,
  Square,
  Terminal,
  Trash2,
} from "lucide-react"

import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"

interface LogEntry {
  timestamp: number
  time_str: string
  level: "info" | "warn" | "error"
  source: string
  message: string
}

export function LogsPage() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [levelFilter, setLevelFilter] = useState<string>("all")
  const [search, setSearch] = useState("")
  const [isStreaming, setIsStreaming] = useState(true)
  const [autoScroll, setAutoScroll] = useState(true)
  const logContainerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!isStreaming) return

    const eventSource = new EventSource("/api/logs")

    eventSource.onmessage = (event) => {
      try {
        const entry: LogEntry = JSON.parse(event.data)
        setLogs((prev) => {
          // Avoid duplicate entries if any
          if (prev.length > 0 && prev[prev.length - 1].timestamp === entry.timestamp && prev[prev.length - 1].message === entry.message) {
            return prev
          }
          return [...prev.slice(-499), entry]
        })
      } catch (e) {
        console.error("Failed to parse log entry:", e)
      }
    }

    eventSource.onerror = (e) => {
      console.warn("EventSource disconnected, retrying in background:", e)
    }

    return () => {
      eventSource.close()
    }
  }, [isStreaming])

  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
    }
  }, [logs, autoScroll])

  const filteredLogs = logs.filter((l) => {
    if (levelFilter !== "all" && l.level !== levelFilter) return false
    if (!search) return true
    const term = search.toLowerCase()
    return (
      l.message.toLowerCase().includes(term) ||
      l.source.toLowerCase().includes(term)
    )
  })

  return (
    <div className="flex flex-1 flex-col gap-6">
      {/* Header */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            Console Logs
          </h1>
          <p className="text-sm text-muted-foreground">
            Live streaming system daemon events, proxy rotations, and failover notifications.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant={isStreaming ? "default" : "outline"}
            size="sm"
            onClick={() => setIsStreaming(!isStreaming)}
          >
            {isStreaming ? (
              <>
                <Square className="size-3.5 mr-1.5 fill-current" />
                Pause Stream
              </>
            ) : (
              <>
                <Play className="size-3.5 mr-1.5" />
                Resume Stream
              </>
            )}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setLogs([])}
            title="Clear logs"
          >
            <Trash2 className="size-3.5 mr-1.5" />
            Clear
          </Button>
        </div>
      </div>

      {/* Toolbar filters */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-1 rounded-xl bg-muted p-1">
          {["all", "info", "warn", "error"].map((lvl) => (
            <button
              key={lvl}
              type="button"
              onClick={() => setLevelFilter(lvl)}
              className={`rounded-lg px-2.5 py-1 text-xs font-mono uppercase transition-all ${
                levelFilter === lvl
                  ? "bg-card text-foreground font-semibold shadow-xs"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {lvl}
            </button>
          ))}
        </div>

        <div className="relative flex-1 max-w-xs">
          <Input
            placeholder="Search logs..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="text-xs font-mono h-8"
          />
        </div>

        <div className="ml-auto flex items-center gap-2 text-xs font-mono text-muted-foreground">
          <label className="flex items-center gap-1.5 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={autoScroll}
              onChange={(e) => setAutoScroll(e.target.checked)}
              className="rounded"
            />
            Auto-scroll
          </label>
          <span>•</span>
          <span>{filteredLogs.length} events</span>
        </div>
      </div>

      {/* Console Viewer */}
      <Card className="flex-1 flex flex-col min-h-[480px] overflow-hidden border-border bg-black/95 text-zinc-100 font-mono text-xs dark:bg-zinc-950">
        <div className="flex items-center justify-between border-b border-zinc-800 bg-zinc-900/80 px-4 py-2 text-[11px] text-zinc-400">
          <div className="flex items-center gap-2">
            <Terminal className="size-3.5 text-emerald-400" />
            <span>aip-gateway.log</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="flex items-center gap-1">
              <span className={`size-1.5 rounded-full ${isStreaming ? "bg-emerald-500 animate-pulse" : "bg-amber-500"}`} />
              {isStreaming ? "CONNECTED" : "PAUSED"}
            </span>
          </div>
        </div>

        <div
          ref={logContainerRef}
          className="flex-1 overflow-y-auto p-4 space-y-1.5 font-mono select-text"
        >
          {filteredLogs.length > 0 ? (
            filteredLogs.map((entry, i) => {
              const levelColor =
                entry.level === "error"
                  ? "text-red-400 bg-red-950/50 border-red-800/40"
                  : entry.level === "warn"
                  ? "text-amber-400 bg-amber-950/50 border-amber-800/40"
                  : "text-emerald-400 bg-emerald-950/30 border-emerald-800/40"

              return (
                <div
                  key={i}
                  className="flex items-start gap-2.5 leading-relaxed hover:bg-zinc-900/60 p-1 rounded-md transition-colors"
                >
                  <span className="text-zinc-500 shrink-0 select-none">
                    {entry.time_str || new Date(entry.timestamp).toLocaleTimeString()}
                  </span>
                  <span
                    className={`px-1.5 py-0.2 rounded text-[10px] uppercase font-semibold border shrink-0 select-none ${levelColor}`}
                  >
                    {entry.level}
                  </span>
                  <span className="text-purple-400/90 shrink-0 font-medium select-none">
                    [{entry.source}]
                  </span>
                  <span className="text-zinc-200 break-all">{entry.message}</span>
                </div>
              )
            })
          ) : (
            <div className="flex flex-col items-center justify-center h-full text-zinc-500 py-16">
              <Terminal className="size-8 stroke-1 text-zinc-600 mb-2" />
              <p>No console log events match your filter</p>
            </div>
          )}
        </div>
      </Card>
    </div>
  )
}
