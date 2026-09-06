import Link from "next/link";

export default function TeacherNotFound() {
  return (
    <div className="mx-auto max-w-md px-4 py-24 text-center">
      <h1 className="font-display text-2xl">We couldn&rsquo;t find that teacher</h1>
      <p className="mt-2 text-muted-foreground">
        The profile may have been removed, or the link is wrong.
      </p>
      <Link
        href="/teachers"
        className="mt-6 inline-flex h-11 items-center rounded-xl bg-primary px-5 text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary/90"
      >
        Browse all teachers
      </Link>
    </div>
  );
}
