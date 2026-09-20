"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Plus, Sparkles, Trash2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import type { LessonType } from "@/features/teachers/api";
import {
  LESSON_DURATIONS,
  createLessonType,
  getOwnLessonTypes,
  setLessonTypeArchived,
  updateLessonType,
  type LessonDuration,
  type LessonTypeDraft,
} from "@/features/dashboard/api";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";

/**
 * The teacher's own lesson list — what a student sees as bookable offerings.
 * A row is edited in place: title, blurb, and a price per length (leave a
 * length blank and it isn't offered).
 */
export function LessonTypesPanel() {
  const [types, setTypes] = useState<LessonType[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [editing, setEditing] = useState<string | "new" | null>(null);
  const t = useTranslations("lessonEditor");

  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const list = await getOwnLessonTypes();
        if (alive) {
          setTypes(list);
          setState("ready");
        }
      } catch {
        if (alive) setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, []);

  function replace(lt: LessonType) {
    setTypes((cur) => {
      const without = cur.filter((x) => x.id !== lt.id);
      return [...without, lt].sort(
        (a, b) => Number(b.isTrial) - Number(a.isTrial) || a.position - b.position,
      );
    });
  }

  if (state === "loading") return <div className="h-64 animate-pulse bg-muted" />;
  if (state === "error") {
    return (
      <p className="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
        {t("couldNotLoad")}
      </p>
    );
  }

  const live = types.filter((lt) => !lt.archived);
  const archived = types.filter((lt) => lt.archived);

  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm text-muted-foreground">{t("intro")}</p>

      <ul className="flex flex-col gap-3">
        {live.map((lt) =>
          editing === lt.id ? (
            <li key={lt.id}>
              <LessonForm
                initial={lt}
                onCancel={() => setEditing(null)}
                onSaved={(saved) => {
                  replace(saved);
                  setEditing(null);
                }}
              />
            </li>
          ) : (
            <li key={lt.id}>
              <LessonRow
                lessonType={lt}
                onEdit={() => setEditing(lt.id)}
                onArchived={replace}
              />
            </li>
          ),
        )}
      </ul>

      {editing === "new" ? (
        <LessonForm
          hasTrial={live.some((lt) => lt.isTrial)}
          onCancel={() => setEditing(null)}
          onSaved={(saved) => {
            replace(saved);
            setEditing(null);
          }}
        />
      ) : (
        <Button variant="outline" className="self-start" onClick={() => setEditing("new")}>
          <Plus /> {t("addLesson")}
        </Button>
      )}

      {archived.length > 0 && (
        <section>
          <h3 className="text-sm font-bold text-muted-foreground">{t("archivedTitle")}</h3>
          <ul className="mt-2 flex flex-col gap-2">
            {archived.map((lt) => (
              <li
                key={lt.id}
                className="flex items-center justify-between gap-3 border border-border px-4 py-2.5 text-sm"
              >
                <span className="text-muted-foreground">{lt.title}</span>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={async () => replace(await setLessonTypeArchived(lt.id, false))}
                >
                  {t("restore")}
                </Button>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

function LessonRow({
  lessonType: lt,
  onEdit,
  onArchived,
}: {
  lessonType: LessonType;
  onEdit: () => void;
  onArchived: (lt: LessonType) => void;
}) {
  const t = useTranslations("lessonEditor");
  const locale = useLocale();
  const [busy, setBusy] = useState(false);

  return (
    <div className="border border-border bg-card p-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <span className="inline-flex items-center gap-2 font-bold">
          {lt.title}
          {lt.isTrial && (
            <span className="inline-flex items-center gap-1 rounded-sm bg-accent px-1.5 py-0.5 text-xs font-bold text-accent-foreground">
              <Sparkles className="size-3" />
              {t("trial")}
            </span>
          )}
        </span>
        <span className="text-sm text-muted-foreground">
          {lt.prices
            .map((p) => `${p.durationMinutes}m · ${formatMoney(p.price, locale)}`)
            .join("   ")}
        </span>
      </div>
      {lt.description && (
        <p className="mt-1 text-sm text-muted-foreground">{lt.description}</p>
      )}
      <div className="mt-3 flex gap-2 border-t border-border pt-3">
        <Button size="sm" variant="outline" onClick={onEdit}>
          {t("edit")}
        </Button>
        <Button
          size="sm"
          variant="ghost"
          className="ml-auto text-muted-foreground"
          disabled={busy}
          onClick={async () => {
            setBusy(true);
            try {
              onArchived(await setLessonTypeArchived(lt.id, true));
            } finally {
              setBusy(false);
            }
          }}
        >
          <Trash2 /> {t("archive")}
        </Button>
      </div>
    </div>
  );
}

/** Create or edit one offering. Prices are entered in whole so'm. */
function LessonForm({
  initial,
  hasTrial,
  onCancel,
  onSaved,
}: {
  initial?: LessonType;
  hasTrial?: boolean;
  onCancel: () => void;
  onSaved: (lt: LessonType) => void;
}) {
  const t = useTranslations("lessonEditor");
  const [title, setTitle] = useState(initial?.title ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [isTrial, setIsTrial] = useState(initial?.isTrial ?? false);
  const [prices, setPrices] = useState<Record<number, string>>(() => {
    const out: Record<number, string> = {};
    for (const p of initial?.prices ?? []) {
      out[p.durationMinutes] = String(Math.round(p.price.amountMinor / 100));
    }
    return out;
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    const draft: LessonTypeDraft = {
      title: title.trim(),
      description: description.trim(),
      isTrial,
      position: initial?.position ?? 0,
      prices: LESSON_DURATIONS.flatMap((d) => {
        const raw = prices[d]?.trim();
        if (!raw) return [];
        const som = Number(raw);
        if (!Number.isFinite(som) || som < 0) return [];
        return [{ durationMinutes: d as LessonDuration, priceMinor: Math.round(som * 100) }];
      }),
    };
    setSaving(true);
    setError(null);
    try {
      onSaved(initial ? await updateLessonType(initial.id, draft) : await createLessonType(draft));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("couldNotSave"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="border border-foreground bg-card p-4">
      <div className="flex flex-col gap-3">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="lesson-title">{t("titleLabel")}</Label>
          <Input
            id="lesson-title"
            value={title}
            maxLength={80}
            placeholder={t("titlePlaceholder")}
            onChange={(e) => setTitle(e.target.value)}
          />
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="lesson-description">{t("descriptionLabel")}</Label>
          <textarea
            id="lesson-description"
            rows={2}
            value={description}
            placeholder={t("descriptionPlaceholder")}
            onChange={(e) => setDescription(e.target.value)}
            className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          />
        </div>

        <fieldset className="flex flex-col gap-2">
          <legend className="mb-1 text-sm font-bold">{t("pricesLabel")}</legend>
          <p className="text-xs text-muted-foreground">{t("pricesHint")}</p>
          <div className="mt-1 grid gap-2 sm:grid-cols-5">
            {LESSON_DURATIONS.map((d) => (
              <label key={d} className="flex flex-col gap-1 text-xs">
                <span className="font-bold">{t("minutes", { count: d })}</span>
                <Input
                  inputMode="numeric"
                  value={prices[d] ?? ""}
                  placeholder="—"
                  onChange={(e) =>
                    setPrices((cur) => ({ ...cur, [d]: e.target.value }))
                  }
                />
              </label>
            ))}
          </div>
        </fieldset>

        {!initial && (
          <label
            className={cn(
              "flex items-center gap-2 text-sm",
              hasTrial && "cursor-not-allowed opacity-60",
            )}
          >
            <input
              type="checkbox"
              checked={isTrial}
              disabled={hasTrial}
              onChange={(e) => setIsTrial(e.target.checked)}
              className="accent-primary"
            />
            {hasTrial ? t("trialTaken") : t("markAsTrial")}
          </label>
        )}

        {error && (
          <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}

        <div className="flex gap-2">
          <Button onClick={save} disabled={saving || title.trim() === ""}>
            {saving ? t("saving") : t("save")}
          </Button>
          <Button variant="outline" onClick={onCancel}>
            {t("cancel")}
          </Button>
        </div>
      </div>
    </div>
  );
}
