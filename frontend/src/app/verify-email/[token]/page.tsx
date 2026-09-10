import type { Metadata } from "next";
import { AuthCard } from "@/features/auth/components/auth-card";
import { VerifyEmailPanel } from "@/features/auth/components/verify-email-panel";

export const metadata: Metadata = {
  title: "Confirm your email",
};

export default async function VerifyEmailPage({
  params,
}: PageProps<"/verify-email/[token]">) {
  const { token } = await params;
  return (
    <AuthCard title="Confirm your email">
      <VerifyEmailPanel token={token} />
    </AuthCard>
  );
}
