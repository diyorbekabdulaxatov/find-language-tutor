"use client";

import { useEffect, useState } from "react";
import { Video } from "lucide-react";
import { Button } from "@/components/ui/button";

const JOIN_OPENS_MS = 10 * 60_000; // join button unlocks 10 min before start

function countdownLabel(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const d = Math.floor(total / 86_400);
  const h = Math.floor((total % 86_400) / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

/**
 * The video-call panel on a confirmed booking. Shows a live countdown, unlocks
 * the join button 10 minutes before the start, and closes it once the lesson
 * ends. `meetingUrl` is empty until the teacher sets a link (the backend only
 * sends it to participants on a confirmed booking).
 */
export function LessonJoinCard({
  startAt,
  endAt,
  meetingUrl,
  isTeacher,
}: {
  startAt: string;
  endAt: string;
  meetingUrl: string;
  isTeacher: boolean;
}) {
  const [now, setNow] = useState(() => Date.now());

  const start = new Date(startAt).getTime();
  const end = new Date(endAt).getTime();
  const untilStart = start - now;
  const ended = now >= end;
  const joinOpen = now >= start - JOIN_OPENS_MS && !ended;

  useEffect(() => {
    if (ended) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [ended]);

  return (
    <div className="mt-5 rounded-xl bg-primary/8 p-4">
      <div className="flex items-center gap-2 text-sm font-medium">
        <Video className="size-4" /> Video call
      </div>

      {ended ? (
        <p className="mt-1 text-sm text-muted-foreground">
          This lesson has ended.
        </p>
      ) : (
        <>
          <p className="mt-1 text-sm text-muted-foreground">
            {joinOpen
              ? meetingUrl
                ? "You can join now."
                : isTeacher
                  ? "Add a meeting link below so your student can join."
                  : "Waiting for the teacher to share the meeting link."
              : `Starts in ${countdownLabel(untilStart)}.`}
          </p>

          {meetingUrl ? (
            <Button
              asChild={joinOpen}
              disabled={!joinOpen}
              className="mt-3"
              size="lg"
            >
              {joinOpen ? (
                <a href={meetingUrl} target="_blank" rel="noopener noreferrer">
                  Join lesson
                </a>
              ) : (
                <span>Join opens 10 min before</span>
              )}
            </Button>
          ) : null}
        </>
      )}
    </div>
  );
}
