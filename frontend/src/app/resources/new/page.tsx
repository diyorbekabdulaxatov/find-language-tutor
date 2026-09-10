import type { Metadata } from "next";
import { Suspense } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { RequireUser } from "@/features/auth/require-user";
import { ResourceEditor } from "@/features/resources/components/resource-editor";
import { RESOURCE_TYPES, type ResourceType } from "@/features/resources/types";

export const metadata: Metadata = { title: "New resource" };

const VALID = new Set(RESOURCE_TYPES.map((t) => t.value));

export default async function NewResourcePage({
  searchParams,
}: PageProps<"/resources/new">) {
  const { type } = await searchParams;
  const chosen =
    typeof type === "string" && VALID.has(type as ResourceType)
      ? (type as ResourceType)
      : null;

  return (
    <Suspense>
      <RequireUser>
        <div className="mx-auto max-w-3xl px-4 py-10 sm:px-6">
          {chosen ? (
            <ResourceEditor type={chosen} />
          ) : (
            <>
              <Link
                href="/resources"
                className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
              >
                <ArrowLeft className="size-4" />
                Resources
              </Link>
              <h1 className="mt-2 font-display text-2xl">
                What are you making?
              </h1>
              <ul className="mt-6 grid gap-3 sm:grid-cols-2">
                {RESOURCE_TYPES.map((t) => (
                  <li key={t.value}>
                    <Link
                      href={`/resources/new?type=${t.value}`}
                      className="block rounded-2xl border border-border bg-card p-4 transition-colors hover:border-primary/50"
                    >
                      <span className="font-medium">{t.label}</span>
                      <p className="mt-0.5 text-sm text-muted-foreground">
                        {t.blurb}
                      </p>
                    </Link>
                  </li>
                ))}
              </ul>
            </>
          )}
        </div>
      </RequireUser>
    </Suspense>
  );
}
