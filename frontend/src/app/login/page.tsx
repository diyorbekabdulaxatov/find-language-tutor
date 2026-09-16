import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { AuthForm } from "@/features/auth/components/auth-form";
import { AuthCard } from "@/features/auth/components/auth-card";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth");
  return { title: t("loginMeta"), description: t("loginMetaDescription") };
}

export default async function LoginPage() {
  const t = await getTranslations("auth");
  return (
    <AuthCard title={t("loginTitle")}>
      <Suspense fallback={<FormSkeleton />}>
        <AuthForm mode="login" />
      </Suspense>
    </AuthCard>
  );
}

function FormSkeleton() {
  return <div className="h-64 animate-pulse rounded-xl bg-muted" />;
}
