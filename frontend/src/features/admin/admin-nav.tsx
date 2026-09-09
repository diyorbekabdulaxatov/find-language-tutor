"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { BarChart3, GraduationCap, ShieldCheck, Users } from "lucide-react";
import { cn } from "@/lib/utils";
import { PERMISSIONS, type Permission } from "./permissions";
import { useCan } from "./use-can";

const LINKS: {
  href: string;
  label: string;
  icon: typeof BarChart3;
  exact?: boolean;
  perm: Permission;
}[] = [
  { href: "/admin", label: "Dashboard", icon: BarChart3, exact: true, perm: PERMISSIONS.metricsView },
  { href: "/admin/teachers", label: "Teachers", icon: GraduationCap, perm: PERMISSIONS.teachersView },
  { href: "/admin/users", label: "Users", icon: Users, perm: PERMISSIONS.usersView },
  { href: "/admin/roles", label: "Roles", icon: ShieldCheck, perm: PERMISSIONS.rolesManage },
];

export function AdminNav() {
  const pathname = usePathname();
  const { can } = useCan();
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
            {label}
          </Link>
        );
      })}
    </nav>
  );
}
