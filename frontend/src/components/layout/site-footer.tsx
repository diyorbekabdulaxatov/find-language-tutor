import Link from "next/link";

const COLUMNS: { heading: string; links: { label: string; href: string }[] }[] = [
  {
    heading: "Learn",
    links: [
      { label: "Find a teacher", href: "/teachers" },
      { label: "How lessons work", href: "/#how-it-works" },
      { label: "Pricing", href: "/#pricing" },
    ],
  },
  {
    heading: "Teach",
    links: [
      { label: "Become a teacher", href: "/#teach" },
      { label: "Payouts", href: "/#payouts" },
    ],
  },
  {
    heading: "Company",
    links: [
      { label: "About", href: "/about" },
      { label: "Contact", href: "/contact" },
    ],
  },
];

export function SiteFooter() {
  return (
    <footer className="border-t border-border">
      <div className="mx-auto grid max-w-6xl gap-10 px-4 py-12 sm:grid-cols-[1.5fr_repeat(3,1fr)] sm:px-6">
        <div>
          <p className="font-display text-lg font-medium">findtutor</p>
          <p className="mt-2 max-w-xs text-sm text-muted-foreground">
            One-on-one language lessons with teachers you choose, on a schedule
            that works across timezones.
          </p>
        </div>

        {COLUMNS.map((col) => (
          <nav key={col.heading} className="text-sm">
            <p className="font-medium text-foreground">{col.heading}</p>
            <ul className="mt-3 space-y-2">
              {col.links.map((link) => (
                <li key={link.href}>
                  <Link
                    href={link.href}
                    className="text-muted-foreground transition-colors hover:text-foreground"
                  >
                    {link.label}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        ))}
      </div>

      <div className="mx-auto max-w-6xl px-4 pb-10 text-xs text-muted-foreground sm:px-6">
        © {new Date().getFullYear()} findtutor. A portfolio project.
      </div>
    </footer>
  );
}
