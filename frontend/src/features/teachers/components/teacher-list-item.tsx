import Image from "next/image";
import Link from "next/link";
import { Play } from "lucide-react";
import type { TeacherSummary } from "@/types/teacher";
import { Badge } from "@/components/ui/badge";
import { Rating } from "./rating";
import { TeacherAvatar } from "./teacher-avatar";
import { formatMoney, formatCompact, formatResponseTime } from "@/lib/format";

/**
 * One row in the teacher roster. Deliberately not a card: a wide editorial entry
 * where the teacher's own headline is the loud element, set in the display serif
 * like a quote.
 */
export function TeacherListItem({ teacher }: { teacher: TeacherSummary }) {
  const href = `/teachers/${teacher.slug}`;
  const kindLabel =
    teacher.kind === "professional" ? "Professional teacher" : "Community tutor";

  return (
    <article className="grid gap-4 border-b border-border py-8 first:pt-0 sm:grid-cols-[minmax(0,200px)_1fr] sm:gap-6">
      <Link
        href={href}
        className="group relative block aspect-video overflow-hidden rounded-lg bg-muted ring-1 ring-foreground/10"
        aria-label={`${teacher.displayName} — watch intro`}
      >
        <Image
          src={teacher.videoThumbnailUrl}
          alt=""
          fill
          sizes="(max-width: 640px) 100vw, 200px"
          className="object-cover transition-transform duration-500 group-hover:scale-105"
        />
        <span className="absolute inset-0 grid place-items-center">
          <span className="grid size-10 place-items-center rounded-full bg-background/85 text-foreground shadow-sm backdrop-blur transition-colors group-hover:bg-primary group-hover:text-primary-foreground">
            <Play className="size-4 translate-x-px fill-current" />
          </span>
        </span>
      </Link>

      <div className="min-w-0">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-center gap-2.5">
            <TeacherAvatar src={teacher.avatarUrl} name={teacher.displayName} size={36} />
            <h3 className="text-lg font-semibold leading-tight">
              <Link href={href} className="hover:underline">
                {teacher.displayName}
              </Link>
            </h3>
          </div>
          <p className="shrink-0 text-right text-sm">
            <span className="font-medium">
              {formatMoney(teacher.pricePerHour)}
            </span>
            <span className="text-muted-foreground"> / hour</span>
          </p>
        </div>

        <p className="mt-2 max-w-[48ch] font-display text-lg italic leading-snug text-foreground/90">
          &ldquo;{teacher.headline}&rdquo;
        </p>

        <div className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-1 text-sm">
          <Rating value={teacher.rating} reviewCount={teacher.reviewCount} />
          <span className="text-muted-foreground">
            {teacher.city}, {teacher.countryName}
          </span>
          <span className="text-muted-foreground">
            {formatCompact(teacher.lessonsCompleted)} lessons taught
          </span>
          <span className="text-muted-foreground">
            Replies in {formatResponseTime(teacher.responseTimeHours)}
          </span>
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-2">
          <Badge variant="secondary">{kindLabel}</Badge>
          {teacher.focus.slice(0, 3).map((tag) => (
            <Badge key={tag} variant="outline" className="font-normal">
              {tag}
            </Badge>
          ))}
          {!teacher.acceptingStudents && (
            <span className="text-xs text-muted-foreground">
              Not taking new students right now
            </span>
          )}
        </div>
      </div>
    </article>
  );
}
