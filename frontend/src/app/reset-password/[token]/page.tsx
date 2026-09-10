import type { Metadata } from "next";
import { AuthCard } from "@/features/auth/components/auth-card";
import { ResetPasswordForm } from "@/features/auth/components/reset-password-form";

export const metadata: Metadata = {
  title: "Choose a new password",
};

export default async function ResetPasswordPage({
  params,
}: PageProps<"/reset-password/[token]">) {
  const { token } = await params;
  return (
    <AuthCard title="Choose a new password">
      <ResetPasswordForm token={token} />
    </AuthCard>
  );
}
