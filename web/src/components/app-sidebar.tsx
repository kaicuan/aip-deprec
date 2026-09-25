import { Link, useLocation } from "react-router-dom"
import {
  Activity,
  Cpu,
  Layers,
  Network,
  Radio,
  Terminal,
} from "lucide-react"

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"

const navItems = [
  {
    title: "Overview",
    url: "/",
    icon: Layers,
  },
  {
    title: "Integration & Models",
    url: "/integration",
    icon: Cpu,
  },
  {
    title: "Proxies",
    url: "/proxies",
    icon: Network,
  },
  {
    title: "Usage",
    url: "/usage",
    icon: Activity,
  },
  {
    title: "Console Logs",
    url: "/logs",
    icon: Terminal,
  },
]

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const location = useLocation()

  return (
    <Sidebar collapsible="offcanvas" {...props}>
      <SidebarHeader className="px-4 py-3">
        <div className="flex items-center gap-2.5">
          <div className="flex size-7 items-center justify-center rounded-lg bg-primary text-primary-foreground font-bold shadow-xs">
            <Radio className="size-4 animate-pulse" />
          </div>
          <div className="grid flex-1 text-left text-sm leading-tight">
            <span className="truncate font-semibold tracking-tight text-sidebar-foreground">
              AIP Dashboard
            </span>
            <span className="truncate text-xs text-sidebar-foreground/60 font-mono">
              OpenCode Free Proxy
            </span>
          </div>
        </div>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel className="text-xs font-medium text-sidebar-foreground/50 px-2 py-1">
            Platform
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {navItems.map((item) => {
                const isActive = location.pathname === item.url
                return (
                  <SidebarMenuItem key={item.title}>
                    <SidebarMenuButton
                      isActive={isActive}
                      tooltip={item.title}
                      render={<Link to={item.url} />}
                    >
                      <item.icon className="size-4" />
                      <span>{item.title}</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                )
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter className="border-t border-sidebar-border p-3">
        <div className="flex items-center justify-between text-xs text-sidebar-foreground/60 px-1 font-mono">
          <span className="flex items-center gap-1.5">
            <span className="size-1.5 rounded-full bg-emerald-500" />
            Gateway Live
          </span>
          <span>v1.0.0</span>
        </div>
      </SidebarFooter>
    </Sidebar>
  )
}
