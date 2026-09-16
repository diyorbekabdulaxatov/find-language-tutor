import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { EditResource } from "@/features/resources/components/edit-resource";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("resources");
  return { title: t("metaEdit") };
}

export default async function EditResourcePage({
  params,
}: PageProps<"/resources/[id]/edit">) {
  const { id } = await params;
  return (
    <Suspense>
      <RequireUser>
        <div className="mx-auto max-w-3xl px-4 py-10 sm:px-6">
          <EditResource id={id} />
        </div>
      </RequireUser>
    </Suspense>
  );
}
