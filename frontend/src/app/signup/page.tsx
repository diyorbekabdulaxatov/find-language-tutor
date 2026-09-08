import type { Metadata } from "next";
import { Suspense } from "react";
import { AuthForm } from "@/features/auth/components/auth-form";
import { AuthCard } from "@/features/auth/components/auth-card";

export const metadata: Metadata = {
  title: "Sign up",
  description: "Create a findtutor account to book lessons or teach.",
};

export default function SignupPage() {
  return (
    <AuthCard title="Create your account">
      <Suspense fallback={<FormSkeleton />}>
        <AuthForm mode="signup" />
      </Suspense>
    </AuthCard>
  );
}

function FormSkeleton() {
  return <div className="h-80 animate-pulse rounded-xl bg-muted" />;
}
