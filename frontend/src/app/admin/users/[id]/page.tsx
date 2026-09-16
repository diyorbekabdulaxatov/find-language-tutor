import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { UserDetail } from "@/features/admin/components/user-detail";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaUser") };
}

export default async function AdminUserPage({
  params,
}: PageProps<"/admin/users/[id]">) {
  const { id } = await params;
  return <UserDetail id={id} />;
}
