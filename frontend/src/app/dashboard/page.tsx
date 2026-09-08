import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { DashboardShell } from "@/features/dashboard/components/dashboard-shell";

export const metadata: Metadata = {
  title: "Teacher dashboard",
};

export default function DashboardPage() {
  return (
    <Suspense>
      <RequireUser>
        <DashboardShell />
      </RequireUser>
    </Suspense>
  );
}
