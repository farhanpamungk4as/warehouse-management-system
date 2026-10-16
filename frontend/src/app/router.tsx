import {
  Link,
  Navigate,
  NavLink,
  Outlet,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import { Warehouse, ChevronRight, X } from "lucide-react";
import { navigation } from "./navigation";
import { NavigationSearch } from "../components/NavigationSearch";
import {
  SidebarProvider,
  Sidebar,
  SidebarHeader,
  SidebarContent,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarFooter,
  SidebarInset,
  SidebarTrigger,
  useSidebar,
} from "../components/ui/sidebar";
import { Button } from "../components/ui/button";
import { Separator } from "../components/ui/separator";
import { ErrorState } from "../components/ui/ErrorState";
import { ItemListPage } from "../pages/ItemListPage";
import { ItemCreatePage } from "../pages/ItemCreatePage";
import { ItemEditPage } from "../pages/ItemEditPage";
import { ItemDetailPage } from "../features/items/ItemDetailPage";
import { DashboardPage } from "../features/dashboard/DashboardPage";
import { InventoryPage } from "../features/inventory/InventoryPage";
import { LocationsPage } from "../features/locations/LocationsPage";
import { ReceivingPage } from "../features/receiving/ReceivingPage";
import { TransfersPage } from "../features/transfers/TransfersPage";
function Navigation() {
  const { setOpenMobile, isMobile } = useSidebar();
  const path = useLocation().pathname;
  return (
    <Sidebar collapsible="icon">
      <SidebarHeader className="p-4">
        <Link
          className="brand-link"
          to="/dashboard"
          onClick={() => setOpenMobile(false)}
        >
          <span className="brand-icon">
            <Warehouse size={21} />
          </span>
          <span className="group-data-[collapsible=icon]:hidden">
            Warehouse
          </span>
        </Link>
        {isMobile && (
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => setOpenMobile(false)}
            aria-label="Close navigation"
          >
            <X />
          </Button>
        )}
      </SidebarHeader>
      <SidebarContent>
        {["Workspace", "Inventory", "Operations", "Warehouse"].map((group) => (
          <SidebarGroup key={group}>
            <SidebarGroupLabel>{group}</SidebarGroupLabel>
            <SidebarMenu aria-label={`${group} navigation`}>
              {navigation
                .filter((item) => item.group === group)
                .map((item) => (
                  <SidebarMenuItem key={item.to}>
                    <SidebarMenuButton
                      asChild
                      isActive={path.startsWith(item.to)}
                      tooltip={item.label}
                    >
                      <NavLink
                        to={item.to}
                        onClick={() => setOpenMobile(false)}
                      >
                        <item.icon />
                        <span>{item.label}</span>
                      </NavLink>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
            </SidebarMenu>
          </SidebarGroup>
        ))}
      </SidebarContent>
      <SidebarFooter className="p-4">
        <div className="sidebar-workspace group-data-[collapsible=icon]:hidden">
          <span>W</span>
          <div>
            <strong>Warehouse workspace</strong>
            <small>Inventory management</small>
          </div>
        </div>
      </SidebarFooter>
    </Sidebar>
  );
}
function Layout() {
  const location = useLocation();
  const nav = navigation.find((n) => location.pathname.startsWith(n.to));
  const section =
    location.pathname === "/items/new"
      ? "Create item"
      : location.pathname.endsWith("/edit")
        ? "Edit item"
        : /^\/items\/\d+$/.test(location.pathname)
          ? "Item details"
          : null;
  return (
    <SidebarProvider
      style={
        {
          "--sidebar-width": "15.5rem",
          "--sidebar-width-icon": "3.5rem",
        } as React.CSSProperties
      }
    >
      <a href="#main-content" className="skip-link">
        Skip to content
      </a>
      <Navigation />
      <SidebarInset className="min-w-0">
        <header className="enterprise-topbar">
          <SidebarTrigger aria-label="Toggle navigation" />
          <Separator orientation="vertical" className="h-4" />
          <nav aria-label="Breadcrumb" className="breadcrumb">
            <span className="header-module">{nav?.label ?? "Workspace"}</span>
            {section && (
              <>
                <ChevronRight size={14} />
                <span className="header-section">{section}</span>
              </>
            )}
          </nav>
          <NavigationSearch />
          <div className="header-actions">
            <div className="workspace-identity">
              <span className="workspace-avatar">W</span>
              <span>Workspace</span>
            </div>
          </div>
        </header>
        <main
          id="main-content"
          className="enterprise-main"
          key={location.pathname}
        >
          <Outlet />
        </main>
        <footer className="enterprise-footer">
          <span>WAREHOUSE OPERATIONS</span>
          <span>Clarity in every movement.</span>
        </footer>
      </SidebarInset>
    </SidebarProvider>
  );
}
export function AppRoutes() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Navigate replace to="/dashboard" />} />
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/items" element={<ItemListPage />} />
        <Route path="/items/new" element={<ItemCreatePage />} />
        <Route path="/items/:id" element={<ItemDetailPage />} />
        <Route path="/items/:id/edit" element={<ItemEditPage />} />
        <Route path="/inventory" element={<InventoryPage />} />
        <Route path="/receiving" element={<ReceivingPage />} />
        <Route path="/transfers" element={<TransfersPage />} />
        <Route path="/locations" element={<LocationsPage />} />
        <Route
          path="*"
          element={
            <ErrorState
              title="Page not found"
              message="Return to your workspace to continue."
            />
          }
        />
      </Route>
    </Routes>
  );
}
