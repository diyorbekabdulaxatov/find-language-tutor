import type { SpokenLanguage } from "@/types/teacher";

/** Turn ["English", "Portuguese"] into "English and Portuguese". */
function joinNames(names: string[]): string {
  if (names.length <= 1) return names.join("");
  if (names.length === 2) return `${names[0]} and ${names[1]}`;
  return `${names.slice(0, -1).join(", ")}, and ${names[names.length - 1]}`;
}

export function LanguageLine({
  teaches,
  alsoSpeaks,
  className,
}: {
  teaches: SpokenLanguage[];
  alsoSpeaks: SpokenLanguage[];
  className?: string;
}) {
  const taught = joinNames(teaches.map((l) => l.name));
  const also = joinNames(alsoSpeaks.map((l) => l.name));

  return (
    <p className={className}>
      Teaches {taught}.
      {also && <span className="text-muted-foreground"> Also speaks {also}.</span>}
    </p>
  );
}
