import type { Metadata } from "next";
import { UserDetail } from "@/features/admin/components/user-detail";

export const metadata: Metadata = { title: "User" };

export default async function AdminUserPage({
  params,
}: PageProps<"/admin/users/[id]">) {
  const { id } = await params;
  return <UserDetail id={id} />;
}
