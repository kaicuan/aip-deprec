import { BrowserRouter, Route, Routes } from "react-router-dom"
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { AppSidebar } from "@/components/app-sidebar"
import { SiteHeader } from "@/components/site-header"
import { ThemeProvider } from "@/components/theme-provider"

import { OverviewPage } from "@/pages/Overview"
import { IntegrationPage } from "@/pages/Integration"
import { ProxiesPage } from "@/pages/Proxies"
import { UsagePage } from "@/pages/Usage"
import { LogsPage } from "@/pages/Logs"

export function App() {
  return (
    <ThemeProvider defaultTheme="system" storageKey="vite-ui-theme">
      <BrowserRouter>
        <SidebarProvider>
          <AppSidebar variant="inset" />
          <SidebarInset className="overflow-clip">
            <SiteHeader />
            <main className="flex flex-1 flex-col">
              <div className="@container/main flex flex-1 flex-col gap-2">
                <div className="flex flex-col gap-4 p-4 md:gap-6 md:p-6">
                  <Routes>
                    <Route path="/" element={<OverviewPage />} />
                    <Route path="/integration" element={<IntegrationPage />} />
                    <Route path="/proxies" element={<ProxiesPage />} />
                    <Route path="/usage" element={<UsagePage />} />
                    <Route path="/logs" element={<LogsPage />} />
                    <Route path="*" element={<OverviewPage />} />
                  </Routes>
                </div>
              </div>
            </main>
          </SidebarInset>
        </SidebarProvider>
      </BrowserRouter>
    </ThemeProvider>
  )
}

export default App
