import { ModeToggle } from "@/components/mode-toggle"
import { Separator } from "@/components/ui/separator"
import { SidebarTrigger } from "@/components/ui/sidebar"
import { useLocation } from "react-router-dom"

const routeTitles: Record<string, string> = {
  "/": "Overview",
  "/integration": "Integrations",
  "/proxies": "Proxies",
  "/usage": "Usage & Audit Logs",
  "/logs": "Console Logs",
}

export function SiteHeader() {
  const location = useLocation()
  const title = routeTitles[location.pathname] || "Dashboard"

  return (
    <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center justify-between gap-2 border-b border-border bg-background transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-12">
      <div className="flex w-full items-center gap-1 px-4 lg:gap-2 lg:px-6">
        <SidebarTrigger className="-ml-1" />
        <Separator
          orientation="vertical"
          className="mx-2 data-vertical:h-4 data-vertical:self-auto"
        />
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold tracking-tight text-foreground">
            {title}
          </span>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <ModeToggle />
        </div>
      </div>
    </header>
  )
}
