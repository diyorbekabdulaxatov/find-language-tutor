import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { RequireUser } from "@/features/auth/require-user";
import { ResourceEditor } from "@/features/resources/components/resource-editor";
import { RESOURCE_TYPES, type ResourceType } from "@/features/resources/types";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("resources");
  return { title: t("metaNew") };
}

const VALID = new Set<string>(RESOURCE_TYPES);

export default async function NewResourcePage({
  searchParams,
}: PageProps<"/resources/new">) {
  const [{ type }, t, tType] = await Promise.all([
    searchParams,
    getTranslations("resources"),
    getTranslations("resourceTypes"),
  ]);
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
                className="inline-flex items-center gap-1.5 text-sm font-bold text-link hover:underline"
              >
                <ArrowLeft className="size-4" />
                {t("back")}
              </Link>
              <h1 className="mt-2 font-display text-2xl">{t("whatMaking")}</h1>
              <ul className="mt-6 grid gap-3 sm:grid-cols-2">
                {RESOURCE_TYPES.map((type) => (
                  <li key={type}>
                    <Link
                      href={`/resources/new?type=${type}`}
                      className="block border border-border bg-card p-4 transition-colors hover:bg-accent"
                    >
                      <span className="font-bold">{tType(`long.${type}`)}</span>
                      <p className="mt-0.5 text-sm text-muted-foreground">{tType(`blurb.${type}`)}</p>
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
