"use client";

/**
 * The right-hand side of the site header. Renders the log-in / sign-up links
 * when signed out, and an account dropdown when signed in. While the initial
 * silent refresh is in flight it renders a neutral placeholder so the header
 * doesn't flicker from "logged out" to "logged in" on every reload.
 */

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  BookOpen,
  CalendarDays,
  GraduationCap,
  LayoutDashboard,
  LogOut,
  PenLine,
  Shield,
  Sparkles,
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
  const t = useTranslations("nav");

  if (status === "loading") {
    return <div className="size-9 rounded-full bg-muted" aria-hidden />;
  }

  if (status === "unauthenticated" || !user) {
    return (
      <>
        <Link
          href="/login"
          className="hidden h-10 items-center rounded-md border border-foreground bg-background px-3 text-sm font-bold text-foreground transition-colors hover:bg-accent sm:inline-flex dark:border-foreground/60"
        >
          {t("login")}
        </Link>
        <Link
          href="/signup"
          className="inline-flex h-10 items-center rounded-md bg-ink px-3 text-sm font-bold text-ink-foreground transition-colors hover:bg-ink/85"
        >
          {t("signUp")}
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
      <Link
        href="/learn"
        className="hidden rounded-md px-3 py-2 text-sm text-foreground transition-colors hover:text-link lg:inline-flex"
      >
        {t("myLearning")}
      </Link>
      <DropdownMenuTrigger
        aria-label={t("accountMenu")}
        className="rounded-full outline-none focus-visible:ring-3 focus-visible:ring-ring/40"
      >
        <Avatar className="size-9">
          <AvatarFallback className="bg-ink text-sm font-bold text-ink-foreground">
            {initials(user.displayName)}
          </AvatarFallback>
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
            {t("myBookings")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/learn">
            <Sparkles />
            {t("myLearning")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/dashboard">
            <LayoutDashboard />
            {t("teacherDashboard")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/resources">
            <BookOpen />
            {t("teachingResources")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/grading">
            <PenLine />
            {t("homeworkToGrade")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/courses">
            <GraduationCap />
            {t("myCourses")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/account">
            <UserIcon />
            {t("accountSettings")}
          </Link>
        </DropdownMenuItem>
        {user.permissions.length > 0 && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link href="/admin">
                <Shield />
                {t("admin")}
              </Link>
            </DropdownMenuItem>
          </>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={handleLogout}>
          <LogOut />
          {t("signOut")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
