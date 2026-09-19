import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { AuthForm } from "@/features/auth/components/auth-form";
import { AuthCard } from "@/features/auth/components/auth-card";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth");
  return { title: t("signupMeta"), description: t("signupMetaDescription") };
}

export default async function SignupPage() {
  const t = await getTranslations("auth");
  return (
    <AuthCard title={t("signupTitle")}>
      <Suspense fallback={<FormSkeleton />}>
        <AuthForm mode="signup" />
      </Suspense>
    </AuthCard>
  );
}

function FormSkeleton() {
  return <div className="h-80 animate-pulse bg-muted" />;
}
