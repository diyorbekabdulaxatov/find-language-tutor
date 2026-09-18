"use client";

import { useTransition } from "react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Check, Languages } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { LOCALE_LABELS, LOCALES, type Locale } from "@/i18n/config";
import { setLocale } from "@/i18n/actions";
import { useAuth } from "@/features/auth/auth-context";
import { updateProfile } from "@/features/auth/api";

export function LocaleSwitcher({ withLabel = false }: { withLabel?: boolean }) {
  const locale = useLocale();
  const t = useTranslations("common");
  const router = useRouter();
  const { status, setUser } = useAuth();
  const [pending, startTransition] = useTransition();

  function choose(next: Locale) {
    if (next === locale) return;
    startTransition(async () => {
      await setLocale(next);
      // A signed-in person also wants their email in this language. Best
      // effort: the cookie already switched the UI, so a failure here is not
      // worth interrupting them over.
      if (status === "authenticated") {
        try {
          setUser(await updateProfile({ locale: next }));
        } catch {
          /* keep the UI switch */
        }
      }
      router.refresh();
    });
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size={withLabel ? "default" : "icon"}
          aria-label={t("changeLanguage")}
          title={t("changeLanguage")}
          disabled={pending}
        >
          <Languages />
          {withLabel && LOCALE_LABELS[locale as Locale]}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-40">
        {LOCALES.map((l) => (
          <DropdownMenuItem key={l} onClick={() => choose(l)}>
            <span className="flex-1">{LOCALE_LABELS[l]}</span>
            {l === locale && <Check className="size-4 text-primary" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
