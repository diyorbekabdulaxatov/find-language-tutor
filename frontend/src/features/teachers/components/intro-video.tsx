"use client";

import { useState } from "react";
import Image from "next/image";
import { Play } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * Intro-video placeholder. The real player (a <video> or an embed from R2) drops
 * in where the "coming soon" panel is. For now, clicking the poster reveals that
 * panel so the interaction is wired up.
 *
 * `className` controls the aspect ratio / size from the caller.
 */
export function IntroVideo({
  poster,
  name,
  className,
}: {
  poster: string;
  name: string;
  className?: string;
}) {
  const [playing, setPlaying] = useState(false);

  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-2xl bg-muted ring-1 ring-border",
        className,
      )}
    >
      {playing ? (
        <div className="grid h-full place-items-center bg-foreground p-6 text-center text-sm text-background">
          Video playback isn&rsquo;t wired up yet — {name}&rsquo;s intro will play
          here.
        </div>
      ) : (
        <button
          type="button"
          onClick={() => setPlaying(true)}
          className="group absolute inset-0"
          aria-label={`Play ${name}'s intro video`}
        >
          <Image
            src={poster}
            alt={name}
            fill
            priority
            sizes="(max-width: 768px) 100vw, 320px"
            className="object-cover transition-transform duration-500 group-hover:scale-[1.04]"
          />
          <span className="absolute inset-0 grid place-items-center bg-black/15 transition-colors group-hover:bg-black/25">
            <span className="grid size-14 place-items-center rounded-full bg-card/95 text-foreground shadow-lift transition-colors group-hover:bg-primary group-hover:text-primary-foreground">
              <Play className="size-6 translate-x-0.5 fill-current" />
            </span>
          </span>
          <span className="absolute bottom-3 left-3 rounded-full bg-black/55 px-2.5 py-1 text-xs font-medium text-white backdrop-blur">
            Watch 1-min intro
          </span>
        </button>
      )}
    </div>
  );
}
