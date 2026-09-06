import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function TeacherNotFound() {
  return (
    <div className="mx-auto max-w-md px-4 py-24 text-center">
      <h1 className="font-display text-2xl font-medium">
        We couldn&rsquo;t find that teacher
      </h1>
      <p className="mt-2 text-muted-foreground">
        The profile may have been removed, or the link is wrong.
      </p>
      <Button asChild className="mt-6 h-10 px-4">
        <Link href="/teachers">Browse all teachers</Link>
      </Button>
    </div>
  );
}
