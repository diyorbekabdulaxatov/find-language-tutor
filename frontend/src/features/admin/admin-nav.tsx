"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { BarChart3, GraduationCap, Users } from "lucide-react";
import { cn } from "@/lib/utils";

const LINKS = [
  { href: "/admin", label: "Dashboard", icon: BarChart3, exact: true },
  { href: "/admin/teachers", label: "Teachers", icon: GraduationCap },
  { href: "/admin/users", label: "Users", icon: Users },
];

export function AdminNav() {
  const pathname = usePathname();
  return (
    <nav className="flex gap-1 border-b border-border">
      {LINKS.map(({ href, label, icon: Icon, exact }) => {
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
