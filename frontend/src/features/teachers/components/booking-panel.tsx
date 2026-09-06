import { CalendarCheck, Clock, ShieldCheck } from "lucide-react";
import type { TeacherProfile } from "@/types/teacher";
import { formatMoney, formatResponseTime } from "@/lib/format";

/**
 * The price + book actions. Sticky on desktop (positioned by the profile page),
 * a plain block on mobile. The buttons are inert until the booking flow exists.
 */
export function BookingPanel({ teacher }: { teacher: TeacherProfile }) {
  const firstName = teacher.displayName.split(" ")[0];

  return (
    <div className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-card">
      <div className="flex items-end gap-1.5">
        <span className="font-display text-3xl text-foreground">
          {formatMoney(teacher.pricePerHour)}
        </span>
        <span className="pb-1 text-sm text-muted-foreground">/ 60 min</span>
      </div>

      {teacher.trialPrice && (
        <p className="mt-1.5 inline-flex items-center gap-1.5 rounded-full bg-coral/10 px-2.5 py-1 text-xs font-semibold text-coral">
          Trial lesson {formatMoney(teacher.trialPrice)} · 30 min
        </p>
      )}

      <div className="mt-5 space-y-2.5">
        {teacher.acceptingStudents ? (
          <>
            <button className="h-12 w-full rounded-xl bg-primary text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary/90">
              Book a lesson
            </button>
            {teacher.trialPrice && (
              <button className="h-12 w-full rounded-xl bg-coral/12 text-sm font-semibold text-coral transition-colors hover:bg-coral/20">
                Book a trial lesson
              </button>
            )}
          </>
        ) : (
          <>
            <button
              disabled
              className="h-12 w-full cursor-not-allowed rounded-xl bg-secondary text-sm font-semibold text-muted-foreground"
            >
              Not taking new students
            </button>
            <button className="h-12 w-full rounded-xl bg-secondary text-sm font-semibold text-foreground transition-colors hover:bg-accent">
              Message {firstName}
            </button>
          </>
        )}
      </div>

      <dl className="mt-6 space-y-3 border-t border-border pt-5 text-sm">
        <Row icon={Clock} label="Replies in">
          {formatResponseTime(teacher.responseTimeHours)}
        </Row>
        <Row icon={CalendarCheck} label="Lessons taught">
          {teacher.lessonsCompleted.toLocaleString("en-US")}
        </Row>
        <Row icon={ShieldCheck} label="Active students">
          {teacher.studentCount}
        </Row>
      </dl>

      <p className="mt-5 rounded-xl bg-primary/8 p-3 text-xs text-muted-foreground">
        You&rsquo;re only charged after the lesson is confirmed. Free cancellation
        up to 12 hours before.
      </p>
    </div>
  );
}

function Row({
  icon: Icon,
  label,
  children,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between">
      <dt className="inline-flex items-center gap-2 text-muted-foreground">
        <Icon className="size-4" />
        {label}
      </dt>
      <dd className="font-medium text-foreground">{children}</dd>
    </div>
  );
}
