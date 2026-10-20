import { useEffect, useState } from "react"
import {
  AlertCircle,
  CheckCircle2,
  Clock,
  Network,
  Play,
  Plus,
  RefreshCw,
  Trash2,
  X,
  XCircle,
} from "lucide-react"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
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
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs"
import {
  Field,
  FieldDescription,
  FieldLabel,
} from "@/components/ui/field"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Textarea } from "@/components/ui/textarea"
import { Input } from "@/components/ui/input"
import {
  addProxies,
  deleteProxies,
  getProxies,
  testProxies,
  type ProxyItem,
} from "@/lib/api"

export function ProxiesPage() {
  const [proxies, setProxies] = useState<ProxyItem[]>([])
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [testingIds, setTestingIds] = useState<Record<number, boolean>>({})
  const [testingAll, setTestingAll] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [isAddOpen, setIsAddOpen] = useState(false)
  const [nowSec, setNowSec] = useState(() => Math.floor(Date.now() / 1000))

  // Form states for Add Drawer
  const [addMode, setAddMode] = useState<"bulk" | "single">("bulk")
  const [bulkInput, setBulkInput] = useState("")
  const [defaultProtocol, setDefaultProtocol] = useState("http")
  const [singleHost, setSingleHost] = useState("")
  const [singlePort, setSinglePort] = useState("")
  const [singleUser, setSingleUser] = useState("")
  const [singlePass, setSinglePass] = useState("")
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const loadProxies = async () => {
    try {
      const data = await getProxies()
      setProxies(data)
    } catch (err) {
      console.error("Failed to load proxies:", err)
    }
  }

  useEffect(() => {
    loadProxies()
  }, [])

  // Timer tick for live 429 countdown
  useEffect(() => {
    const interval = setInterval(() => {
      setNowSec(Math.floor(Date.now() / 1000))
    }, 1000)
    return () => clearInterval(interval)
  }, [])

  // Toggle selection
  const handleToggleSelect = (id: number) => {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((i) => i !== id) : [...prev, id]
    )
  }

  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(proxies.map((p) => p.id))
    } else {
      setSelectedIds([])
    }
  }

  const isAllSelected = proxies.length > 0 && selectedIds.length === proxies.length

  // Test selected or single
  const handleTest = async (ids?: number[]) => {
    const targetIds = ids || selectedIds
    if (targetIds.length === 0) return

    const idMap: Record<number, boolean> = {}
    targetIds.forEach((id) => (idMap[id] = true))
    setTestingIds((prev) => ({ ...prev, ...idMap }))

    try {
      await testProxies(targetIds)
      await loadProxies()
    } catch (err) {
      console.error("Test failed:", err)
    } finally {
      setTestingIds((prev) => {
        const next = { ...prev }
        targetIds.forEach((id) => delete next[id])
        return next
      })
    }
  }

  const handleTestAll = async () => {
    try {
      setTestingAll(true)
      await testProxies([])
      await loadProxies()
    } catch (err) {
      console.error("Test all failed:", err)
    } finally {
      setTestingAll(false)
    }
  }

  // Delete selected or single
  const handleDelete = async (ids: number[]) => {
    if (ids.length === 0) return
    try {
      setDeleting(true)
      await deleteProxies(ids)
      setSelectedIds((prev) => prev.filter((id) => !ids.includes(id)))
      await loadProxies()
    } catch (err) {
      console.error("Delete failed:", err)
    } finally {
      setDeleting(false)
    }
  }

  // Add Submit
  const handleAddSubmit = async () => {
    setFormError(null)
    setSubmitting(true)

    try {
      let inputString = ""
      if (addMode === "bulk") {
        inputString = bulkInput.trim()
        if (!inputString) {
          setFormError("Please enter at least one proxy URL or IP:Port")
          setSubmitting(false)
          return
        }
      } else {
        if (!singleHost || !singlePort) {
          setFormError("Host and Port are required")
          setSubmitting(false)
          return
        }
        if (singleUser) {
          inputString = `${defaultProtocol}://${singleUser}:${singlePass}@${singleHost}:${singlePort}`
        } else {
          inputString = `${defaultProtocol}://${singleHost}:${singlePort}`
        }
      }

      const res = await addProxies(inputString, defaultProtocol)
      if (res.errors && res.errors.length > 0 && res.added_count === 0) {
        setFormError(res.errors[0])
        setSubmitting(false)
        return
      }

      setBulkInput("")
      setSingleHost("")
      setSinglePort("")
      setSingleUser("")
      setSinglePass("")
      setIsAddOpen(false)
      await loadProxies()
    } catch (err: any) {
      setFormError(err.message || "Failed to add proxy")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex flex-1 flex-col gap-6">
      {/* Header */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            Proxy Pool Management
          </h1>
          <p className="text-sm text-muted-foreground">
            SOCKS5, HTTP, and Direct proxies with automated 429 tracking and failover.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleTestAll}
            disabled={testingAll || proxies.length === 0}
          >
            <Play className={`size-3.5 mr-1.5 ${testingAll ? "animate-spin" : ""}`} />
            {testingAll ? "Testing All..." : "Test All"}
          </Button>
          <Button
            variant="default"
            size="sm"
            onClick={() => setIsAddOpen(true)}
          >
            <Plus className="size-3.5 mr-1.5" />
            Add Proxy
          </Button>
        </div>
      </div>

      {/* Batch Action Toolbar */}
      {selectedIds.length > 0 && (
        <div className="flex items-center justify-between rounded-xl border border-border bg-card px-4 py-2.5 shadow-xs">
          <span className="text-xs font-medium text-foreground">
            {selectedIds.length} {selectedIds.length === 1 ? "proxy" : "proxies"} selected
          </span>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="xs"
              onClick={() => handleTest()}
              disabled={Object.keys(testingIds).length > 0}
            >
              <RefreshCw className="size-3 mr-1" />
              Test Selected
            </Button>
            <Button
              variant="destructive"
              size="xs"
              onClick={() => handleDelete(selectedIds)}
              disabled={deleting}
            >
              <Trash2 className="size-3 mr-1" />
              Delete Selected
            </Button>
          </div>
        </div>
      )}

      {/* Proxies Table */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-semibold flex items-center gap-2">
            <Network className="size-4 text-primary" />
            Configured Nodes ({proxies.length})
          </CardTitle>
          <CardDescription>
            Probe checks latency against Cloudflare CDN edge without calling OpenCode upstream.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="rounded-xl border border-border overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[40px]">
                    <Checkbox
                      checked={isAllSelected}
                      onCheckedChange={(checked) => handleSelectAll(!!checked)}
                      aria-label="Select all"
                    />
                  </TableHead>
                  <TableHead className="w-[100px]">Protocol</TableHead>
                  <TableHead>Host / Endpoint</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Latency</TableHead>
                  <TableHead>Egress IP</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {proxies.length > 0 ? (
                  proxies.map((p) => {
                    const isSelected = selectedIds.includes(p.id)
                    const isTesting = !!testingIds[p.id]

                    // Calculate live countdown
                    const remaining =
                      p.status === "rate_limited" && p.cooldown_until > 0
                        ? Math.max(0, p.cooldown_until - nowSec)
                        : 0

                    const formatCountdown = (secs: number) => {
                      const m = Math.floor(secs / 60)
                      const s = secs % 60
                      return `${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`
                    }

                    return (
                      <TableRow key={p.id} data-state={isSelected ? "selected" : undefined}>
                        <TableCell>
                          <Checkbox
                            checked={isSelected}
                            onCheckedChange={() => handleToggleSelect(p.id)}
                            aria-label={`Select proxy ${p.id}`}
                          />
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="font-mono text-[10px] uppercase">
                            {p.protocol}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {p.protocol === "direct" ? (
                            <span className="text-muted-foreground italic">direct (local connection)</span>
                          ) : p.protocol === "relay" ? (
                            <div className="flex items-center gap-1.5">
                              <span className="font-medium text-foreground">{p.host}</span>
                              <span className="text-[10px] text-muted-foreground bg-muted px-1 rounded">
                                relay edge
                              </span>
                            </div>
                          ) : (
                            <div className="flex items-center gap-1.5">
                              <span className="font-medium text-foreground">{p.host}</span>
                              <span className="text-muted-foreground">:{p.port}</span>
                              {p.username && (
                                <span className="text-[10px] text-muted-foreground/70 bg-muted px-1 rounded">
                                  auth
                                </span>
                              )}
                            </div>
                          )}
                        </TableCell>
                        <TableCell>
                          {p.status === "online" || (p.status === "rate_limited" && remaining === 0) ? (
                            <Badge variant="default" className="bg-emerald-600 hover:bg-emerald-600 text-white font-mono text-[11px] gap-1">
                              <CheckCircle2 className="size-3" />
                              online
                            </Badge>
                          ) : p.status === "rate_limited" ? (
                            <Badge variant="secondary" className="bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30 font-mono text-[11px] gap-1">
                              <Clock className="size-3 animate-spin" />
                              429 ({formatCountdown(remaining)})
                            </Badge>
                          ) : (
                            <Badge variant="destructive" className="font-mono text-[11px] gap-1">
                              <XCircle className="size-3" />
                              error
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {isTesting ? (
                            <span className="animate-pulse">probing...</span>
                          ) : p.latency_ms > 0 ? (
                            `${p.latency_ms}ms`
                          ) : (
                            "—"
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {p.egress_ip || "—"}
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-1">
                            <Button
                              variant="ghost"
                              size="icon-xs"
                              onClick={() => handleTest([p.id])}
                              disabled={isTesting}
                              title="Test probe"
                              aria-label="Test proxy"
                            >
                              <Play className={`size-3.5 ${isTesting ? "animate-spin" : ""}`} />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon-xs"
                              onClick={() => handleDelete([p.id])}
                              className="text-muted-foreground hover:text-destructive"
                              title="Delete proxy"
                              aria-label="Delete proxy"
                            >
                              <Trash2 className="size-3.5" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })
                ) : (
                  <TableRow>
                    <TableCell colSpan={7} className="h-32 text-center text-muted-foreground">
                      No proxies configured. Click &quot;Add Proxy&quot; to populate your pool.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Add Proxy Drawer */}
      <Drawer
        open={isAddOpen}
        onOpenChange={(open) => {
          setIsAddOpen(open)
          if (!open) setFormError(null)
        }}
        swipeDirection="right"
      >
        <DrawerContent className="data-[swipe-axis=x]:sm:max-w-md data-[swipe-axis=x]:sm:[--drawer-content-width:28rem]">
          <div className="flex h-full flex-col">
            <DrawerHeader className="border-b border-border pb-4">
              <div className="flex items-center justify-between">
                <DrawerTitle>Add Proxies to Pool</DrawerTitle>
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
              <DrawerDescription>
                Supports standard URLs, vendor format (ip:port:user:pass), and bulk lists.
              </DrawerDescription>
            </DrawerHeader>

            <div className="flex-1 overflow-y-auto p-4 space-y-4">
              <Tabs
                value={addMode}
                onValueChange={(val) => setAddMode(val as "bulk" | "single")}
              >
                <TabsList className="grid w-full grid-cols-2">
                  <TabsTrigger value="bulk">Bulk Import</TabsTrigger>
                  <TabsTrigger value="single">Single Proxy</TabsTrigger>
                </TabsList>

                {/* Protocol selector */}
                <Field className="mt-4">
                  <FieldLabel>Default Protocol</FieldLabel>
                  <div className="flex gap-2">
                    {["http", "socks5", "relay"].map((proto) => (
                      <Button
                        key={proto}
                        type="button"
                        variant={defaultProtocol === proto ? "default" : "outline"}
                        size="xs"
                        onClick={() => setDefaultProtocol(proto)}
                        className="capitalize font-mono text-xs"
                      >
                        {proto}
                      </Button>
                    ))}
                  </div>
                </Field>

                <TabsContent value="bulk" className="mt-4">
                  <Field data-invalid={addMode === "bulk" && !!formError}>
                    <FieldLabel>Proxy List (One per line)</FieldLabel>
                    <Textarea
                      placeholder={`# Supported formats:\n31.59.20.176:6754:user:pass\n45.38.107.97:6014\nhttp://user:pass@host:port\nsocks5://user:pass@host:port\nhttps://your-relay.workers.dev\nhttps://your-relay.vercel.app`}
                      rows={8}
                      value={bulkInput}
                      onChange={(e) => {
                        setBulkInput(e.target.value)
                        if (formError) setFormError(null)
                      }}
                      className="font-mono text-xs leading-relaxed"
                    />
                    <FieldDescription>
                      Auto-normalizes vendor raw format into valid proxy credentials.
                    </FieldDescription>
                  </Field>
                </TabsContent>

                <TabsContent value="single" className="mt-4 space-y-3">
                  <div className="grid grid-cols-3 gap-2">
                    <Field className="col-span-2" data-invalid={addMode === "single" && !!formError && !singleHost}>
                      <FieldLabel>Host / IP</FieldLabel>
                      <Input
                        placeholder="192.168.1.1 or proxy.io"
                        value={singleHost}
                        onChange={(e) => {
                          setSingleHost(e.target.value)
                          if (formError) setFormError(null)
                        }}
                        className="font-mono text-xs"
                      />
                    </Field>
                    <Field data-invalid={addMode === "single" && !!formError && !singlePort}>
                      <FieldLabel>Port</FieldLabel>
                      <Input
                        placeholder="8080"
                        value={singlePort}
                        onChange={(e) => {
                          setSinglePort(e.target.value)
                          if (formError) setFormError(null)
                        }}
                        className="font-mono text-xs"
                      />
                    </Field>
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <Field>
                      <FieldLabel>Username (Optional)</FieldLabel>
                      <Input
                        placeholder="user"
                        value={singleUser}
                        onChange={(e) => setSingleUser(e.target.value)}
                        className="font-mono text-xs"
                      />
                    </Field>
                    <Field>
                      <FieldLabel>Password (Optional)</FieldLabel>
                      <Input
                        type="password"
                        placeholder="pass"
                        value={singlePass}
                        onChange={(e) => setSinglePass(e.target.value)}
                        className="font-mono text-xs"
                      />
                    </Field>
                  </div>
                </TabsContent>
              </Tabs>

              {formError && (
                <div className="rounded-lg bg-destructive/10 p-2.5 text-xs text-destructive flex items-center gap-2">
                  <AlertCircle className="size-4 shrink-0" />
                  <span>{formError}</span>
                </div>
              )}
            </div>

            <DrawerFooter className="border-t border-border p-4 flex-row justify-end gap-2">
              <DrawerClose
                render={
                  <Button
                    variant="outline"
                    size="sm"
                    type="button"
                  />
                }
              >
                Cancel
              </DrawerClose>
              <Button
                variant="default"
                size="sm"
                type="button"
                onClick={handleAddSubmit}
                disabled={submitting}
              >
                {submitting ? "Adding..." : "Add to Pool"}
              </Button>
            </DrawerFooter>
          </div>
        </DrawerContent>
      </Drawer>
    </div>
  )
}
