import { Skeleton } from "@/components/ui/skeleton";

/**
 * Shown while the server component streams. Next renders this instantly from the
 * nearest `loading.tsx` while `TeachersPage` awaits its data.
 */
export default function TeachersLoading() {
  return (
    <div className="mx-auto max-w-6xl px-4 py-10 sm:px-6 lg:py-14">
      <Skeleton className="h-10 w-64" />
      <Skeleton className="mt-3 h-5 w-full max-w-lg" />

      <Skeleton className="mt-8 h-32 w-full rounded-2xl" />
      <Skeleton className="mt-6 h-5 w-40" />

      <div className="mt-4 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="overflow-hidden rounded-2xl ring-1 ring-border">
            <Skeleton className="aspect-[5/4] w-full rounded-none" />
            <div className="space-y-3 p-4">
              <Skeleton className="h-5 w-32" />
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
