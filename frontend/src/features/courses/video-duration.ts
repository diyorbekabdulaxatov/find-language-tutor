/**
 * Reads a video file's duration in the browser, before it is uploaded.
 *
 * The backend deliberately does not probe the uploaded bytes: a lecture's
 * length is display metadata that never gates access, pricing or payouts, so
 * the browser — which already has to decode the file to show a preview — is
 * the cheapest honest source for it. See the 000023 migration's comment for
 * the full reasoning.
 *
 * Resolves to 0 ("unknown") rather than rejecting for anything the browser
 * can't decode: a WebM with no duration in its header, a codec it doesn't
 * support, a stalled metadata load. 0 is a legal value the API accepts and
 * every surface renders as "no duration shown", so a failure here must never
 * block the upload it is decorating.
 */
export function readVideoDuration(file: File, timeoutMs = 5000): Promise<number> {
  return new Promise((resolve) => {
    let url: string | null = null;
    let settled = false;

    const video = document.createElement("video");
    video.preload = "metadata";
    // Muted + no autoplay: we only ever load metadata, never play.
    video.muted = true;

    const finish = (seconds: number) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      video.removeAttribute("src");
      video.load();
      if (url) URL.revokeObjectURL(url);
      resolve(Number.isFinite(seconds) && seconds > 0 ? Math.round(seconds) : 0);
    };

    const timer = setTimeout(() => finish(0), timeoutMs);

    video.onloadedmetadata = () => finish(video.duration);
    video.onerror = () => finish(0);

    try {
      url = URL.createObjectURL(file);
      video.src = url;
    } catch {
      finish(0);
    }
  });
}
