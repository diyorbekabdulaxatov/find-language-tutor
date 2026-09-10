import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireAdmin } from "@/features/admin/require-admin";
import { AdminNav } from "@/features/admin/admin-nav";

export const metadata: Metadata = {
  title: { default: "Admin", template: "%s · Admin · FindTutor" },
};

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <Suspense>
      <RequireAdmin>
        <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
          <h1 className="font-display text-3xl">Admin</h1>
          <div className="mt-4">
            <AdminNav />
          </div>
          <div className="mt-6">{children}</div>
        </div>
      </RequireAdmin>
    </Suspense>
  );
}
