import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { AccountSettings } from "@/features/auth/components/account-settings";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("auth");
  return { title: t("settingsTitle") };
}

export default function AccountPage() {
  return (
    <Suspense>
      <RequireUser>
        <AccountSettings />
      </RequireUser>
    </Suspense>
  );
}
