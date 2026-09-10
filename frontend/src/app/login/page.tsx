import type { Metadata } from "next";
import { Suspense } from "react";
import { AuthForm } from "@/features/auth/components/auth-form";
import { AuthCard } from "@/features/auth/components/auth-card";

export const metadata: Metadata = {
  title: "Log in",
  description: "Sign in to your FindTutor account.",
};

export default function LoginPage() {
  return (
    <AuthCard title="Welcome back">
      <Suspense fallback={<FormSkeleton />}>
        <AuthForm mode="login" />
      </Suspense>
    </AuthCard>
  );
}

function FormSkeleton() {
  return <div className="h-64 animate-pulse rounded-xl bg-muted" />;
}
