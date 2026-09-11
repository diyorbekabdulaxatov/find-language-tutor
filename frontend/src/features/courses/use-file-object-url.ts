"use client";

import { useEffect, useState } from "react";
import { fetchFileObjectUrl } from "@/features/resources/api";

/**
 * Loads a stored file's bytes (a course's video or cover image) through the
 * authenticated endpoint into an object URL, since GET /v1/files/{id} needs
 * a bearer token no plain `<a href>` / `<video src>` / `<img src>` can
 * attach. Mirrors `features/submissions/components/resource-player.tsx`'s
 * `useFileObjectUrl`. Revokes the URL on unmount / id change.
 *
 * Callers should gate rendering the preview on `fileAssetId` being set —
 * `url`/`state` are only meaningful then; when it goes back to null the
 * previous object URL is still revoked (effect cleanup), it's just not
 * surfaced as a new "idle" state since no caller here needs that.
 */
export function useFileObjectUrl(fileAssetId: string | null | undefined) {
  const [url, setUrl] = useState<string | null>(null);
  // Lazy initializer, not an effect — "no id yet" is derivable at first
  // render, so it never needs a synchronous setState inside the effect body.
  const [state, setState] = useState<"idle" | "loading" | "ready" | "error">(() =>
    fileAssetId ? "loading" : "idle",
  );

  useEffect(() => {
    if (!fileAssetId) return;
    let alive = true;
    let objectUrl: string | null = null;
    fetchFileObjectUrl(fileAssetId)
      .then((u) => {
        if (!alive) {
          URL.revokeObjectURL(u);
          return;
        }
        objectUrl = u;
        setUrl(u);
        setState("ready");
      })
      .catch(() => {
        if (alive) setState("error");
      });
    return () => {
      alive = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [fileAssetId]);

  return { url, state };
}
