import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { LocaleSwitcher } from "@/components/layout/locale-switcher";

/**
 * Udemy's footer: a near-black band, link columns in white, the language
 * control on the right, and a bottom row with the wordmark and the copyright.
 */
export async function SiteFooter() {
  const t = await getTranslations("footer");

  const columns: { heading: string; links: { label: string; href: string }[] }[] = [
    {
      heading: t("learn"),
      links: [
        { label: t("findTeacher"), href: "/teachers" },
        { label: t("findCourse"), href: "/courses/catalog" },
        { label: t("howLessonsWork"), href: "/#how-it-works" },
        { label: t("pricing"), href: "/#pricing" },
      ],
    },
    {
      heading: t("teach"),
      links: [
        { label: t("becomeTeacher"), href: "/#teach" },
        { label: t("payouts"), href: "/#payouts" },
      ],
    },
    {
      heading: t("company"),
      links: [
        { label: t("about"), href: "/about" },
        { label: t("contact"), href: "/contact" },
      ],
    },
  ];

  return (
    <footer className="mt-12 bg-ink text-ink-foreground dark:border-t dark:border-border">
      <div className="mx-auto max-w-[1340px] px-4 sm:px-6">
        <div className="flex flex-col gap-10 border-b border-white/15 py-12 lg:flex-row lg:justify-between">
          <div className="grid gap-8 sm:grid-cols-3 lg:w-2/3">
            {columns.map((col) => (
              <nav key={col.heading} className="text-sm">
                <p className="font-bold">{col.heading}</p>
                <ul className="mt-3 space-y-2">
                  {col.links.map((link) => (
                    <li key={link.href}>
                      <Link
                        href={link.href}
                        className="text-white/85 underline-offset-2 hover:underline"
                      >
                        {link.label}
                      </Link>
                    </li>
                  ))}
                </ul>
              </nav>
            ))}
          </div>
          <div className="[&_button]:border-white [&_button]:bg-transparent [&_button]:text-white [&_button]:hover:bg-white/10">
            <LocaleSwitcher withLabel />
          </div>
        </div>

        <div className="flex flex-col gap-4 py-8 sm:flex-row sm:items-center sm:justify-between">
          <Link href="/" className="font-display text-2xl text-white" aria-label="FindTutor">
            FindTutor
          </Link>
          <p className="max-w-md text-xs text-white/75">{t("tagline")}</p>
          <p className="text-xs text-white/75">
            {t("copyright", { year: new Date().getFullYear() })}
          </p>
        </div>
      </div>
    </footer>
  );
}
