import { Link, useLocation, Outlet } from "react-router";
import { useGetIdentity, useLogout } from "@refinedev/core";
import { useTheme } from "next-themes";
import { Workflow, LayoutList, Clock, LogOut, EllipsisVertical, Sun, Moon } from "lucide-react";
import { cn } from "@/lib/utils";
import { Switch } from "@/components/ui/switch";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const navItems = [
  { label: "Workflows", path: "/workflows", icon: Workflow },
  { label: "Queues", path: "/queues", icon: LayoutList },
  { label: "Scheduled", path: "/scheduled", icon: Clock },
];

export function Layout() {
  const location = useLocation();
  const { data: identity } = useGetIdentity<{ email?: string }>();
  const { mutate: logout } = useLogout();
  const { theme, setTheme } = useTheme();

  return (
    <div className="flex h-screen">
      <aside className="flex h-screen w-56 flex-col border-r bg-background">
        <div className="flex h-14 items-center border-b px-4">
          <Workflow className="mr-2 h-5 w-5" />
          <span className="text-lg font-semibold">PocketFlow</span>
        </div>
        <nav className="flex-1 space-y-1 p-3">
          {navItems.map((item) => {
            const isActive = location.pathname.startsWith(item.path);
            return (
              <Link
                key={item.path}
                to={item.path}
                draggable={false}
                className={cn(
                  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-accent text-accent-foreground"
                    : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                )}
              >
                <item.icon className="h-4 w-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>
        <div className="flex items-center justify-between border-t px-4 py-3">
          <div className="flex items-center gap-2 text-muted-foreground">
            <Sun className="h-4 w-4" />
            <Switch
              checked={theme === "dark"}
              onCheckedChange={(checked) => setTheme(checked ? "dark" : "light")}
            />
            <Moon className="h-4 w-4" />
          </div>
        </div>
        <div className="flex items-center gap-3 border-t px-4 py-3">
          <Avatar className="h-8 w-8">
            <AvatarFallback className="text-xs">
              {identity?.email?.charAt(0).toUpperCase() ?? "?"}
            </AvatarFallback>
          </Avatar>
          <span className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
            {identity?.email ?? ""}
          </span>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button className="rounded p-1 hover:bg-accent">
                <EllipsisVertical className="h-4 w-4 text-muted-foreground" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent side="top" align="end">
              <DropdownMenuItem
                onClick={() => logout({ redirectPath: "/login" })}
                className="text-destructive focus:text-destructive"
              >
                <LogOut className="mr-2 h-4 w-4" />
                Logout
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </aside>
      <main className="flex-1 overflow-y-auto p-6">
        <Outlet />
      </main>
    </div>
  );
}
