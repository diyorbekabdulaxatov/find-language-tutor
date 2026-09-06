import { Skeleton } from "@/components/ui/skeleton";

/**
 * Shown while the server component streams. Next renders this instantly from the
 * nearest `loading.tsx` while `TeachersPage` awaits its data.
 */
export default function TeachersLoading() {
  return (
    <div className="mx-auto max-w-6xl px-4 py-10 sm:px-6 lg:py-14">
      <Skeleton className="h-9 w-56" />
      <Skeleton className="mt-3 h-5 w-full max-w-lg" />

      <div className="mt-10 grid gap-10 lg:grid-cols-[240px_1fr]">
        <div className="hidden space-y-6 lg:block">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </div>

        <div className="space-y-8">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="grid gap-4 sm:grid-cols-[200px_1fr] sm:gap-6">
              <Skeleton className="aspect-video w-full rounded-lg" />
              <div className="space-y-3">
                <Skeleton className="h-5 w-48" />
                <Skeleton className="h-6 w-full max-w-md" />
                <Skeleton className="h-4 w-64" />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
