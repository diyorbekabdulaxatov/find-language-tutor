import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { AuthCard } from "@/features/auth/components/auth-card";
import { ResetPasswordForm } from "@/features/auth/components/reset-password-form";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth");
  return { title: t("resetTitle") };
}

export default async function ResetPasswordPage({
  params,
}: PageProps<"/reset-password/[token]">) {
  const [{ token }, t] = await Promise.all([params, getTranslations("auth")]);
  return (
    <AuthCard title={t("resetTitle")}>
      <ResetPasswordForm token={token} />
    </AuthCard>
  );
}
