import Link from "next/link";
import { GraduationCap } from "lucide-react";

/** The centered panel that wraps the login / signup forms. */
export function AuthCard({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="mx-auto flex min-h-[calc(100vh-8rem)] w-full max-w-md flex-col justify-center px-4 py-12 sm:px-6">
      <Link href="/" className="mb-6 flex items-center justify-center gap-2">
        <span className="grid size-8 place-items-center rounded-lg bg-primary text-primary-foreground">
          <GraduationCap className="size-5" />
        </span>
        <span className="font-display text-xl tracking-tight">findtutor</span>
      </Link>

      <div className="rounded-2xl border border-border bg-card p-6 shadow-card sm:p-8">
        <h1 className="mb-6 text-center font-display text-2xl">{title}</h1>
        {children}
      </div>
    </div>
  );
}
