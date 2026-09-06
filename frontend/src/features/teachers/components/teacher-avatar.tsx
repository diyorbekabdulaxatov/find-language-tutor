import Image from "next/image";
import { cn } from "@/lib/utils";

/** Bump the placeholder-avatar service's size param, e.g. .../240?img=1 -> .../480?img=1 */
export function photoUrl(src: string, size: number): string {
  return src.replace(/\/(\d+)\?/, `/${size}?`);
}

/**
 * Round teacher photo. Plain next/image rather than the radix Avatar component —
 * we don't need the fallback/loading state machinery here, and this keeps the
 * component usable from server components without a client boundary.
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
  return (
    <Image
      src={photoUrl(src, Math.max(size * 2, 96))}
      alt={name}
      width={size}
      height={size}
      className={cn(
        "shrink-0 rounded-full object-cover ring-1 ring-border",
        className,
      )}
      style={{ width: size, height: size }}
    />
  );
}
