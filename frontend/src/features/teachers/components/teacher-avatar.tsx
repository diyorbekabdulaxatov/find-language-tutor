import Image from "next/image";
import { cn } from "@/lib/utils";

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
      src={src}
      alt={name}
      width={size}
      height={size}
      className={cn(
        "shrink-0 rounded-full object-cover ring-1 ring-foreground/10",
        className,
      )}
      style={{ width: size, height: size }}
    />
  );
}
