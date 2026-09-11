"use client";

/**
 * The right-hand side of the site header. Renders the log-in / sign-up links
 * when signed out, and an account dropdown when signed in. While the initial
 * silent refresh is in flight it renders a neutral placeholder so the header
 * doesn't flicker from "logged out" to "logged in" on every reload.
 */

import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  BookOpen,
  CalendarDays,
  LayoutDashboard,
  LogOut,
  PenLine,
  Shield,
  User as UserIcon,
} from "lucide-react";
import { useAuth } from "@/features/auth/auth-context";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

function initials(name: string): string {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((p) => p[0]?.toUpperCase() ?? "")
      .join("") || "?"
  );
}

export function AccountMenu() {
  const { status, user, logout } = useAuth();
  const router = useRouter();

  if (status === "loading") {
    return <div className="size-8 rounded-full bg-muted" aria-hidden />;
  }

  if (status === "unauthenticated" || !user) {
    return (
      <>
        <Link
          href="/login"
          className="hidden rounded-lg px-3 py-2 text-sm font-medium text-foreground transition-colors hover:bg-secondary sm:inline-flex"
        >
          Log in
        </Link>
        <Link
          href="/signup"
          className="inline-flex h-9 items-center rounded-lg bg-primary px-4 text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary/90"
        >
          Get started
        </Link>
      </>
    );
  }

  async function handleLogout() {
    await logout();
    router.push("/");
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="Account menu"
        className="rounded-full outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <Avatar>
          <AvatarFallback>{initials(user.displayName)}</AvatarFallback>
        </Avatar>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-56">
        <DropdownMenuLabel className="flex flex-col gap-0.5">
          <span className="font-medium">{user.displayName}</span>
          <span className="text-xs font-normal text-muted-foreground">
            {user.email}
          </span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link href="/bookings">
            <CalendarDays />
            My bookings
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/dashboard">
            <LayoutDashboard />
            Teacher dashboard
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/resources">
            <BookOpen />
            Teaching resources
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/grading">
            <PenLine />
            Homework to grade
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/account">
            <UserIcon />
            Account settings
          </Link>
        </DropdownMenuItem>
        {user.permissions.length > 0 && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link href="/admin">
                <Shield />
                Admin
              </Link>
            </DropdownMenuItem>
          </>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={handleLogout}>
          <LogOut />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
