"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Play } from "lucide-react";
import { cn } from "@/lib/utils";
import { TeacherPhoto } from "@/features/teachers/components/teacher-avatar";

/**
 * The teacher's photo with a play affordance. Clicking swaps in the uploaded
 * intro video (autoplaying, with native controls); a profile with no video
 * yet explains that in place. The photo doubles as the poster, so a teacher
 * with no photo gets the initials tile here too.
 *
 * `className` controls the aspect ratio / size from the caller.
 */
export function IntroVideo({
  poster,
  videoUrl,
  name,
  className,
}: {
  poster: string;
  videoUrl: string;
  name: string;
  className?: string;
}) {
  const [playing, setPlaying] = useState(false);
  const t = useTranslations("profile");

  return (
    <div
      className={cn(
        "relative overflow-hidden border border-border bg-muted",
        className,
      )}
    >
      {playing && videoUrl ? (
        <video
          src={videoUrl}
          poster={poster || undefined}
          controls
          autoPlay
          playsInline
          className="absolute inset-0 size-full bg-black object-contain"
        />
      ) : playing ? (
        <div className="grid h-full place-items-center bg-foreground p-6 text-center text-sm text-background">
          {t("introNotWired", { name })}
        </div>
      ) : (
        <button
          type="button"
          onClick={() => setPlaying(true)}
          className="group absolute inset-0"
          aria-label={t("playIntro", { name })}
        >
          <TeacherPhoto
            src={poster}
            name={name}
            size={640}
            sizes="(max-width: 768px) 100vw, 320px"
            priority
            className="transition-transform duration-500 group-hover:scale-[1.04]"
          />
          <span className="absolute inset-0 grid place-items-center bg-black/15 transition-colors group-hover:bg-black/25">
            <span className="grid size-14 place-items-center rounded-full bg-white text-ink transition-colors group-hover:bg-primary group-hover:text-primary-foreground">
              <Play className="size-6 translate-x-0.5 fill-current" />
            </span>
          </span>
          <span className="absolute bottom-3 left-3 rounded-full bg-black/55 px-2.5 py-1 text-xs font-medium text-white backdrop-blur">
            {t("watchIntro")}
          </span>
        </button>
      )}
    </div>
  );
}
