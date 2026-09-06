"use client";

import { useState } from "react";
import Image from "next/image";
import { Play } from "lucide-react";

/**
 * Intro-video placeholder. The real player (an <video> or an embed from R2)
 * drops in where the "coming soon" panel is. For now, clicking the poster just
 * reveals that panel so the interaction is wired up.
 */
export function IntroVideo({
  poster,
  name,
}: {
  poster: string;
  name: string;
}) {
  const [playing, setPlaying] = useState(false);

  return (
    <div className="relative aspect-video overflow-hidden rounded-xl bg-muted ring-1 ring-foreground/10">
      {playing ? (
        <div className="grid h-full place-items-center bg-foreground/90 p-6 text-center text-sm text-background">
          Video playback isn&rsquo;t wired up yet — this is where {name}&rsquo;s
          intro will play.
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
            alt=""
            fill
            priority
            sizes="(max-width: 768px) 100vw, 620px"
            className="object-cover transition-transform duration-500 group-hover:scale-[1.03]"
          />
          <span className="absolute inset-0 grid place-items-center bg-foreground/10 transition-colors group-hover:bg-foreground/20">
            <span className="grid size-14 place-items-center rounded-full bg-background/90 text-foreground shadow-md backdrop-blur transition-colors group-hover:bg-primary group-hover:text-primary-foreground">
              <Play className="size-5 translate-x-0.5 fill-current" />
            </span>
          </span>
        </button>
      )}
    </div>
  );
}
