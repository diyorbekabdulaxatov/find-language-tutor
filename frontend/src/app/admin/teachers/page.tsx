import type { Metadata } from "next";
import { Suspense } from "react";
import { TeachersTable } from "@/features/admin/components/teachers-table";

export const metadata: Metadata = { title: "Teachers" };

export default function AdminTeachersPage() {
  return (
    <Suspense>
      <TeachersTable />
    </Suspense>
  );
}
