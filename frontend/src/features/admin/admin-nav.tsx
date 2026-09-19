"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  BarChart3,
  Banknote,
  BookOpen,
  CalendarClock,
  GraduationCap,
  MessageSquareText,
  ShieldAlert,
  ShieldCheck,
  Star,
  Users,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { PERMISSIONS, type Permission } from "./permissions";
import { useCan } from "./use-can";

const LINKS: {
  href: string;
  label: NavKey;
  icon: typeof BarChart3;
  exact?: boolean;
  perm: Permission;
}[] = [
  { href: "/admin", label: "navDashboard", icon: BarChart3, exact: true, perm: PERMISSIONS.metricsView },
  { href: "/admin/teachers", label: "navTeachers", icon: GraduationCap, perm: PERMISSIONS.teachersView },
  { href: "/admin/bookings", label: "navBookings", icon: CalendarClock, perm: PERMISSIONS.bookingsView },
  { href: "/admin/disputes", label: "navDisputes", icon: ShieldAlert, perm: PERMISSIONS.disputesResolve },
  { href: "/admin/payouts", label: "navPayouts", icon: Banknote, perm: PERMISSIONS.payoutsView },
  { href: "/admin/reviews", label: "navReviews", icon: Star, perm: PERMISSIONS.reviewsModerate },
  { href: "/admin/courses", label: "navCourses", icon: BookOpen, perm: PERMISSIONS.coursesModerate },
  { href: "/admin/course-reviews", label: "navCourseReviews", icon: MessageSquareText, perm: PERMISSIONS.reviewsModerate },
  { href: "/admin/users", label: "navUsers", icon: Users, perm: PERMISSIONS.usersView },
  { href: "/admin/roles", label: "navRoles", icon: ShieldCheck, perm: PERMISSIONS.rolesManage },
];
type NavKey =
  | "navDashboard" | "navTeachers" | "navBookings" | "navDisputes" | "navPayouts"
  | "navReviews" | "navCourses" | "navCourseReviews" | "navUsers" | "navRoles";

function useLinks() {
  const pathname = usePathname();
  const { can } = useCan();
  const links = LINKS.filter((l) => can(l.perm));
  const isActive = (href: string, exact?: boolean) =>
    exact ? pathname === href : pathname.startsWith(href);
  return { links, isActive };
}

/**
 * Udemy's instructor console keeps its sections in a dark rail down the left
 * edge; ten of them don't fit a tab row anyway. Below `lg` the rail would eat
 * the page, so there it falls back to the scrolling tab row (`AdminTabs`).
 */
export function AdminSidebar() {
  const { links, isActive } = useLinks();
  const t = useTranslations("admin");

  return (
    <nav className="sticky top-[72px] hidden h-[calc(100vh-72px)] overflow-y-auto bg-ink py-4 text-ink-foreground lg:block">
      {links.map(({ href, label, icon: Icon, exact }) => {
        const active = isActive(href, exact);
        return (
          <Link
            key={href}
            href={href}
            aria-current={active ? "page" : undefined}
            className={cn(
              "flex items-center gap-3 border-l-4 px-5 py-3 text-sm font-bold transition-colors",
              active
                ? "border-primary bg-white/10 text-white"
                : "border-transparent text-white/70 hover:bg-white/5 hover:text-white",
            )}
          >
            <Icon className="size-4 shrink-0" />
            {t(label)}
          </Link>
        );
      })}
    </nav>
  );
}

/** The same sections as a scrolling tab row, for narrow screens. */
export function AdminTabs() {
  const { links, isActive } = useLinks();
  const t = useTranslations("admin");

  return (
    <nav className="flex gap-4 overflow-x-auto border-b border-border lg:hidden">
      {links.map(({ href, label, icon: Icon, exact }) => {
        const active = isActive(href, exact);
        return (
          <Link
            key={href}
            href={href}
            aria-current={active ? "page" : undefined}
            className={cn(
              "-mb-px inline-flex shrink-0 items-center gap-1.5 border-b-2 py-3 text-sm font-bold whitespace-nowrap transition-colors",
              active
                ? "border-foreground text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground",
            )}
          >
            <Icon className="size-4" />
            {t(label)}
          </Link>
        );
      })}
    </nav>
  );
}
