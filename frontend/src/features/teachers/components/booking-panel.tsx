import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import type { TeacherProfile } from "@/types/teacher";
import { formatMoney, formatResponseTime } from "@/lib/format";

/**
 * The price + book actions. Sticky on desktop (positioned by the profile page),
 * a plain block on mobile. The buttons are inert until the booking flow exists.
 */
export function BookingPanel({ teacher }: { teacher: TeacherProfile }) {
  return (
    <div className="rounded-xl bg-card p-5 ring-1 ring-foreground/10">
      <div className="flex items-baseline justify-between">
        <span className="font-display text-2xl font-medium">
          {formatMoney(teacher.pricePerHour)}
        </span>
        <span className="text-sm text-muted-foreground">per 60-min lesson</span>
      </div>

      {teacher.trialPrice && (
        <p className="mt-1 text-sm text-muted-foreground">
          Trial lesson {formatMoney(teacher.trialPrice)} · 30 min
        </p>
      )}

      <div className="mt-4 space-y-2">
        {teacher.acceptingStudents ? (
          <>
            <Button className="h-11 w-full text-[0.95rem]">Book a lesson</Button>
            {teacher.trialPrice && (
              <Button variant="outline" className="h-11 w-full text-[0.95rem]">
                Book a trial lesson
              </Button>
            )}
          </>
        ) : (
          <>
            <Button disabled className="h-11 w-full text-[0.95rem]">
              Not taking new students
            </Button>
            <Button variant="outline" className="h-11 w-full text-[0.95rem]">
              Message {teacher.displayName.split(" ")[0]}
            </Button>
          </>
        )}
      </div>

      <Separator className="my-4" />

      <dl className="space-y-2 text-sm">
        <div className="flex justify-between">
          <dt className="text-muted-foreground">Replies in</dt>
          <dd>{formatResponseTime(teacher.responseTimeHours)}</dd>
        </div>
        <div className="flex justify-between">
          <dt className="text-muted-foreground">Lessons taught</dt>
          <dd>{teacher.lessonsCompleted.toLocaleString("en-US")}</dd>
        </div>
        <div className="flex justify-between">
          <dt className="text-muted-foreground">Active students</dt>
          <dd>{teacher.studentCount}</dd>
        </div>
      </dl>

      <p className="mt-4 text-xs text-muted-foreground">
        You&rsquo;re only charged after the lesson is confirmed. Free
        cancellation up to 12 hours before.
      </p>
    </div>
  );
}
