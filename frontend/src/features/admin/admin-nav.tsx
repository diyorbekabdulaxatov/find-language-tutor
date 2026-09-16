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
  { href: "/admin/users", label: "navUsers", icon: Users, perm: PERMISSIONS.usersView },
  { href: "/admin/roles", label: "navRoles", icon: ShieldCheck, perm: PERMISSIONS.rolesManage },
];
type NavKey =
  | "navDashboard" | "navTeachers" | "navBookings" | "navDisputes" | "navPayouts"
  | "navReviews" | "navCourses" | "navUsers" | "navRoles";

export function AdminNav() {
  const pathname = usePathname();
  const { can } = useCan();
  const t = useTranslations("admin");
  const links = LINKS.filter((l) => can(l.perm));

  return (
    <nav className="flex gap-1 border-b border-border">
      {links.map(({ href, label, icon: Icon, exact }) => {
        const active = exact ? pathname === href : pathname.startsWith(href);
        return (
          <Link
            key={href}
            href={href}
            className={cn(
              "inline-flex items-center gap-2 border-b-2 px-3 py-2.5 text-sm font-medium transition-colors",
              active
                ? "border-primary text-foreground"
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
