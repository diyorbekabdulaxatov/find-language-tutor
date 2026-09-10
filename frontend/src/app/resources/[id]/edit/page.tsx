import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { EditResource } from "@/features/resources/components/edit-resource";

export const metadata: Metadata = { title: "Edit resource" };

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
