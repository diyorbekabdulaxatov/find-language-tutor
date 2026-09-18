import Image from "next/image";
import { cn } from "@/lib/utils";

/** Bump the placeholder-avatar service's size param, e.g. .../240?img=1 -> .../480?img=1 */
export function photoUrl(src: string, size: number): string {
  return src.replace(/\/(\d+)\?/, `/${size}?`);
}

/** "Nodira Karimova" -> "NK"; one letter for a single name. */
export function initials(name: string): string {
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");
}

/**
 * A teacher's photo filling its container (`fill` — the parent sets the
 * size / aspect and is `relative`). A profile with no photo yet gets an
 * initials tile in the brand tint instead of a broken image; plain
 * next/image so it works from server components.
 */
export function TeacherPhoto({
  src,
  name,
  size,
  sizes,
  priority,
  className,
}: {
  src: string;
  name: string;
  /** width hint for the placeholder service's size param */
  size: number;
  sizes: string;
  priority?: boolean;
  className?: string;
}) {
  if (!src) {
    return (
      <span
        aria-label={name}
        role="img"
        className={cn(
          "absolute inset-0 grid place-items-center bg-gradient-to-br from-accent to-secondary font-display text-5xl text-accent-foreground",
          className,
        )}
      >
        {initials(name)}
      </span>
    );
  }
  return (
    <Image
      src={photoUrl(src, size)}
      alt={name}
      fill
      sizes={sizes}
      priority={priority}
      className={cn("object-cover", className)}
    />
  );
}

/**
 * Round teacher photo at a fixed pixel size — for list rows and headers.
 */
export function TeacherAvatar({
  src,
  name,
  size = 40,
  className,
}: {
  src: string;
  name: string;
  size?: number;
  className?: string;
}) {
  const ring = "shrink-0 rounded-full ring-1 ring-border";
  if (!src) {
    return (
      <span
        aria-label={name}
        role="img"
        className={cn(
          ring,
          "grid place-items-center bg-accent font-display text-accent-foreground",
          className,
        )}
        style={{ width: size, height: size, fontSize: size * 0.4 }}
      >
        {initials(name)}
      </span>
    );
  }
  return (
    <Image
      src={photoUrl(src, Math.max(size * 2, 96))}
      alt={name}
      width={size}
      height={size}
      className={cn(ring, "object-cover", className)}
      style={{ width: size, height: size }}
    />
  );
}
