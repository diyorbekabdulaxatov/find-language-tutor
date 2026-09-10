import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { AccountSettings } from "@/features/auth/components/account-settings";

export const metadata: Metadata = {
  title: "Account settings",
};

export default function AccountPage() {
  return (
    <Suspense>
      <RequireUser>
        <AccountSettings />
      </RequireUser>
    </Suspense>
  );
}
