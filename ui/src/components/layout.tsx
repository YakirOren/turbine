import type { CSSProperties } from "react";
import { Link, useLocation, Outlet } from "react-router";
import { useGetIdentity, useLogout } from "@refinedev/core";
import { useTheme } from "next-themes";
import {
  LayoutList,
  Clock,
  LogOut,
  Sun,
  Moon,
  Workflow,
  Database,
  Settings,
  Webhook,
  Bell,
  BookOpen,
  ArrowUpRight,
} from "lucide-react";
import { TurbineLogo } from "@/components/turbine-logo";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
} from "@/components/ui/sidebar";

interface NavItem {
  label: string;
  path: string;
  icon: React.ComponentType<{ className?: string }>;
  children?: NavItem[];
}

const navItems: NavItem[] = [
  { label: "Workflows", path: "/workflows", icon: Workflow },
  { label: "Queues", path: "/queues", icon: LayoutList },
  { label: "Scheduled", path: "/scheduled", icon: Clock },
  { label: "KV Store", path: "/kv", icon: Database },
  {
    label: "Settings",
    path: "/settings",
    icon: Settings,
    children: [
      { label: "Webhooks", path: "/settings/webhooks", icon: Webhook },
      { label: "Notifications", path: "/settings/notifications", icon: Bell },
    ],
  },
];

export function Layout() {
  const location = useLocation();
  const { data: identity } = useGetIdentity<{ email?: string }>();
  const { mutate: logout } = useLogout();
  const { resolvedTheme, setTheme } = useTheme();

  return (
    <SidebarProvider
      style={{ "--sidebar-width-icon": "3.5rem" } as CSSProperties}
    >
      <Sidebar collapsible="icon" variant="sidebar">
        <SidebarHeader className="h-14 border-b p-0">
          <Link
            to="/workflows"
            draggable={false}
            className="group/logo flex h-full items-center gap-2 px-4 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0"
          >
            <TurbineLogo className="h-7 w-7 shrink-0 transition-transform duration-700 ease-in-out group-hover/logo:rotate-[360deg]" />
            <span className="text-lg font-semibold group-data-[collapsible=icon]:hidden">
              Turbine
            </span>
          </Link>
        </SidebarHeader>

        <SidebarContent>
          <SidebarGroup>
            <SidebarMenu>
              {navItems.map((item) => {
                const isActive = location.pathname.startsWith(item.path);
                return (
                  <SidebarMenuItem key={item.path}>
                    <SidebarMenuButton
                      asChild
                      isActive={isActive}
                      tooltip={item.label}
                      className="group-data-[collapsible=icon]:size-10! group-data-[collapsible=icon]:[&>svg]:size-5"
                    >
                      <Link
                        to={item.children ? item.children[0].path : item.path}
                        draggable={false}
                      >
                        <item.icon />
                        <span>{item.label}</span>
                      </Link>
                    </SidebarMenuButton>
                    {item.children && isActive && (
                      <SidebarMenuSub>
                        {item.children.map((child) => {
                          const childActive = location.pathname.startsWith(
                            child.path,
                          );
                          return (
                            <SidebarMenuSubItem key={child.path}>
                              <SidebarMenuSubButton
                                asChild
                                isActive={childActive}
                              >
                                <Link to={child.path} draggable={false}>
                                  <child.icon />
                                  <span>{child.label}</span>
                                </Link>
                              </SidebarMenuSubButton>
                            </SidebarMenuSubItem>
                          );
                        })}
                      </SidebarMenuSub>
                    )}
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroup>
          <SidebarGroup className="mt-auto">
            <SidebarMenu>
              <SidebarMenuItem>
                <SidebarMenuButton
                  asChild
                  tooltip="Documentation"
                  className="group-data-[collapsible=icon]:size-10! group-data-[collapsible=icon]:[&>svg]:size-5"
                >
                  <a
                    href="https://turbine.yakir.io/"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    <BookOpen />
                    <span className="group-data-[collapsible=icon]:hidden">Documentation</span>
                    <ArrowUpRight className="ml-auto h-3.5 w-3.5 text-muted-foreground group-data-[collapsible=icon]:hidden" />
                  </a>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarGroup>
        </SidebarContent>

        <SidebarFooter className="gap-0 border-t">
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                tooltip={resolvedTheme === "dark" ? "Light mode" : "Dark mode"}
                onClick={() =>
                  setTheme(resolvedTheme === "dark" ? "light" : "dark")
                }
                className="h-10 [&>svg]:size-5 group-data-[collapsible=icon]:size-10!"
              >
                {resolvedTheme === "dark" ? <Sun /> : <Moon />}
                <span className="group-data-[collapsible=icon]:hidden">
                  {resolvedTheme === "dark" ? "Light mode" : "Dark mode"}
                </span>
              </SidebarMenuButton>
            </SidebarMenuItem>

            <SidebarMenuItem>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <SidebarMenuButton
                    tooltip={identity?.email ?? "Account"}
                    className="h-10 group-data-[collapsible=icon]:size-10!"
                  >
                    <Avatar className="size-5 rounded-sm">
                      <AvatarFallback className="rounded-sm text-[10px]">
                        {identity?.email?.charAt(0).toUpperCase() ?? "?"}
                      </AvatarFallback>
                    </Avatar>
                    <span className="truncate group-data-[collapsible=icon]:hidden">
                      {identity?.email ?? ""}
                    </span>
                  </SidebarMenuButton>
                </DropdownMenuTrigger>
                <DropdownMenuContent side="right" align="end" className="min-w-56">
                  {identity?.email && (
                    <>
                      <DropdownMenuLabel className="flex flex-col gap-0.5 font-normal">
                        <span className="text-xs text-muted-foreground">Signed in as</span>
                        <span className="truncate text-sm">{identity.email}</span>
                      </DropdownMenuLabel>
                      <DropdownMenuSeparator />
                    </>
                  )}
                  <DropdownMenuItem
                    onClick={() => logout({ redirectPath: "/login" })}
                    className="text-destructive focus:text-destructive"
                  >
                    <LogOut className="mr-2 h-4 w-4" />
                    Logout
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>

        <SidebarRail />
      </Sidebar>

      <SidebarInset>
        <header className="flex h-14 items-center gap-2 border-b px-3 md:hidden">
          <SidebarTrigger />
          <Link
            to="/workflows"
            draggable={false}
            className="flex items-center gap-2"
          >
            <TurbineLogo className="h-6 w-6 shrink-0" />
            <span className="text-base font-semibold">Turbine</span>
          </Link>
        </header>
        <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
