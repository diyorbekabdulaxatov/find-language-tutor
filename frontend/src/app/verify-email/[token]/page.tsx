import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { AuthCard } from "@/features/auth/components/auth-card";
import { VerifyEmailPanel } from "@/features/auth/components/verify-email-panel";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth");
  return { title: t("verifyTitle") };
}

export default async function VerifyEmailPage({
  params,
}: PageProps<"/verify-email/[token]">) {
  const [{ token }, t] = await Promise.all([params, getTranslations("auth")]);
  return (
    <AuthCard title={t("verifyTitle")}>
      <VerifyEmailPanel token={token} />
    </AuthCard>
  );
}
