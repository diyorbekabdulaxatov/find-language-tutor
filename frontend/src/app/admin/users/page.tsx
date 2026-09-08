import type { Metadata } from "next";
import { UsersTable } from "@/features/admin/components/users-table";

export const metadata: Metadata = { title: "Users" };

export default function AdminUsersPage() {
  return <UsersTable />;
}
