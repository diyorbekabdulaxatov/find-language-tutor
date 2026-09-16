"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Plus, Trash2 } from "lucide-react";
import type { LanguageLevel } from "@/types/teacher";
import {
  createMyProfile,
  emptyProfileForm,
  ProfileError,
  profileToForm,
  updateMyProfile,
  type ProfileFormValues,
} from "@/features/dashboard/api";
import type { TeacherProfile } from "@/types/teacher";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { TimezoneSelect } from "@/components/ui/timezone-select";

const LEVELS: LanguageLevel[] = ["native", "c2", "c1", "b2", "b1", "a2", "a1"];

/** whole so'm <-> minor units (tiyin, ×100) */
const toSom = (minor: number) => (minor ? Math.round(minor / 100) : 0);
const toMinor = (som: number) => Math.round(som * 100);

export function ProfileEditor({
  initial,
  onSaved,
}: {
  initial: TeacherProfile | null;
  onSaved: (p: TeacherProfile) => void;
}) {
  const router = useRouter();
  const t = useTranslations("profileEditor");
  const isCreate = initial === null;
  const [values, setValues] = useState<ProfileFormValues>(
    initial ? profileToForm(initial) : emptyProfileForm(),
  );
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<"idle" | "saving" | "saved">("idle");

  function set<K extends keyof ProfileFormValues>(
    key: K,
    val: ProfileFormValues[K],
  ) {
    setValues((v) => ({ ...v, [key]: val }));
    setStatus("idle");
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setStatus("saving");
    try {
      const saved = isCreate
        ? await createMyProfile(values)
        : await updateMyProfile(initial.slug, values);
      setValues(profileToForm(saved));
      setStatus("saved");
      onSaved(saved);

      // Only an approved profile has a public page — send the teacher there to
      // see the live result. A new profile is always `pending` and 404s on the
      // public route, so stay put and confirm it in place.
      if (saved.status === "approved") {
        router.push(`/teachers/${saved.slug}`);
        router.refresh();
        return;
      }
      router.refresh();
      // Not public yet — bring the "awaiting review" banner (top of the
      // dashboard) into view instead of leaving the teacher at the save bar.
      if (isCreate) {
        window.scrollTo({ top: 0, behavior: "smooth" });
      }
    } catch (err) {
      setStatus("idle");
      setError(err instanceof ProfileError ? err.message : t("somethingWrong"));
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-8">
      {isCreate && (
        <p className="rounded-xl bg-accent/60 px-4 py-3 text-sm text-accent-foreground">
          {t("noProfileYet")}
        </p>
      )}

      <Section title={t("basics")}>
        <Field label={t("displayName")} htmlFor="displayName">
          <Input
            id="displayName"
            value={values.displayName}
            onChange={(e) => set("displayName", e.target.value)}
            required
          />
        </Field>
        <Field label={t("headline")} htmlFor="headline" hint={t("headlineHint")}>
          <Input
            id="headline"
            value={values.headline}
            onChange={(e) => set("headline", e.target.value)}
            required
          />
        </Field>
        <Field label={t("teacherType")} htmlFor="kind">
          <Select
            value={values.kind}
            onValueChange={(v) => set("kind", v as ProfileFormValues["kind"])}
          >
            <SelectTrigger id="kind">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="professional">{t("professional")}</SelectItem>
              <SelectItem value="community">{t("communityTutor")}</SelectItem>
            </SelectContent>
          </Select>
        </Field>
      </Section>

      <Section title={t("locationTime")}>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("city")} htmlFor="city">
            <Input
              id="city"
              value={values.city}
              onChange={(e) => set("city", e.target.value)}
              required
            />
          </Field>
          <Field label={t("country")} htmlFor="countryName">
            <Input
              id="countryName"
              value={values.countryName}
              onChange={(e) => set("countryName", e.target.value)}
              required
            />
          </Field>
          <Field label={t("countryCode")} htmlFor="countryCode" hint={t("countryCodeHint")}>
            <Input
              id="countryCode"
              value={values.countryCode}
              maxLength={2}
              onChange={(e) =>
                set("countryCode", e.target.value.toUpperCase())
              }
              required
            />
          </Field>
          <Field label={t("timezone")} htmlFor="timezone" hint={t("timezoneHint")}>
            <TimezoneSelect
              id="timezone"
              value={values.timezone}
              onChange={(tz) => set("timezone", tz)}
            />
          </Field>
        </div>
      </Section>

      <Section title={t("pricing")} hint={t("pricingHint")}>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("pricePerHour")} htmlFor="price">
            <Input
              id="price"
              type="number"
              min={0}
              step={1000}
              value={toSom(values.pricePerHourMinor) || ""}
              onChange={(e) =>
                set("pricePerHourMinor", toMinor(Number(e.target.value)))
              }
              required
            />
          </Field>
          <Field label={t("trialPrice")} htmlFor="trial" hint={t("trialPriceHint")}>
            <Input
              id="trial"
              type="number"
              min={0}
              step={1000}
              value={
                values.trialPriceMinor === null
                  ? ""
                  : toSom(values.trialPriceMinor)
              }
              onChange={(e) =>
                set(
                  "trialPriceMinor",
                  e.target.value === "" ? null : toMinor(Number(e.target.value)),
                )
              }
            />
          </Field>
        </div>
      </Section>

      <Section title={t("languages")}>
        <LanguageRows
          rows={values.languages}
          onChange={(rows) => set("languages", rows)}
        />
      </Section>

      <Section title={t("focusAreas")} hint={t("focusHint")}>
        <Input
          value={values.focus.join(", ")}
          onChange={(e) =>
            set(
              "focus",
              e.target.value
                .split(",")
                .map((s) => s.trim())
                .filter(Boolean),
            )
          }
        />
      </Section>

      <Section title={t("about")}>
        <Field label={t("bio")} htmlFor="about">
          <textarea
            id="about"
            rows={4}
            className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
            value={values.about}
            onChange={(e) => set("about", e.target.value)}
          />
        </Field>
        <Field label={t("teachingStyle")} htmlFor="style">
          <textarea
            id="style"
            rows={3}
            className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
            value={values.teachingStyle}
            onChange={(e) => set("teachingStyle", e.target.value)}
          />
        </Field>
      </Section>

      <Section title={t("media")} hint={t("mediaHint")}>
        <Field label={t("avatarUrl")} htmlFor="avatar">
          <Input
            id="avatar"
            value={values.avatarUrl}
            onChange={(e) => set("avatarUrl", e.target.value)}
          />
        </Field>
        <Field label={t("introVideoUrl")} htmlFor="video">
          <Input
            id="video"
            value={values.introVideoUrl}
            onChange={(e) => set("introVideoUrl", e.target.value)}
          />
        </Field>
        <Field label={t("videoThumbnailUrl")} htmlFor="thumb">
          <Input
            id="thumb"
            value={values.videoThumbnailUrl}
            onChange={(e) => set("videoThumbnailUrl", e.target.value)}
          />
        </Field>
      </Section>

      <Section title={t("videoRoom")} hint={t("videoRoomHint")}>
        <Field label={t("meetingLink")} htmlFor="meeting">
          <Input
            id="meeting"
            type="url"
            placeholder="https://meet.example.com/your-room"
            value={values.meetingUrl}
            onChange={(e) => set("meetingUrl", e.target.value)}
          />
        </Field>
      </Section>

      <Section title={t("experience")}>
        <ExperienceRows
          rows={values.experience}
          onChange={(rows) => set("experience", rows)}
        />
      </Section>

      {error && (
        <p
          role="alert"
          className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive"
        >
          {error}
        </p>
      )}

      <div className="sticky bottom-0 -mx-4 flex items-center gap-3 border-t border-border bg-background px-4 py-4 shadow-[0_-10px_20px_-12px_rgba(23,23,51,0.18)] sm:-mx-6 sm:px-6">
        <Button type="submit" disabled={status === "saving"}>
          {status === "saving" ? t("saving") : isCreate ? t("createProfile") : t("saveChanges")}
        </Button>
        {status === "saved" && (
          <span className="text-sm text-muted-foreground">{t("saved")}</span>
        )}
      </div>
    </form>
  );
}

function Section({
  title,
  hint,
  children,
}: {
  title: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="flex flex-col gap-4">
      <div>
        <h2 className="font-display text-lg">{title}</h2>
        {hint && <p className="text-sm text-muted-foreground">{hint}</p>}
      </div>
      {children}
    </section>
  );
}

function Field({
  label,
  htmlFor,
  hint,
  children,
}: {
  label: string;
  htmlFor: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  );
}

function LanguageRows({
  rows,
  onChange,
}: {
  rows: ProfileFormValues["languages"];
  onChange: (rows: ProfileFormValues["languages"]) => void;
}) {
  const t = useTranslations("profileEditor");
  const tLang = useTranslations("languages");
  function update(i: number, patch: Partial<ProfileFormValues["languages"][number]>) {
    onChange(rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));
  }
  return (
    <div className="flex flex-col gap-3">
      {rows.map((row, i) => (
        <div key={i} className="flex flex-wrap items-end gap-2">
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("role")}</Label>
            <Select
              value={row.role}
              onValueChange={(v) =>
                update(i, { role: v as "teaches" | "also_speaks" })
              }
            >
              <SelectTrigger className="w-[130px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="teaches">{t("teaches")}</SelectItem>
                <SelectItem value="also_speaks">{t("alsoSpeaks")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("code")}</Label>
            <Input
              className="w-16"
              value={row.code}
              onChange={(e) => update(i, { code: e.target.value })}
              placeholder="en"
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("name")}</Label>
            <Input
              className="w-32"
              value={row.name}
              onChange={(e) => update(i, { name: e.target.value })}
              placeholder={tLang("en")}
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("level")}</Label>
            <Select
              value={row.level}
              onValueChange={(v) => update(i, { level: v as LanguageLevel })}
            >
              <SelectTrigger className="w-24">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {LEVELS.map((l) => (
                  <SelectItem key={l} value={l}>
                    {l}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => onChange(rows.filter((_, idx) => idx !== i))}
            aria-label={t("removeLanguage")}
          >
            <Trash2 />
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="self-start"
        onClick={() =>
          onChange([
            ...rows,
            { role: "teaches", code: "", name: "", level: "native" },
          ])
        }
      >
        <Plus /> {t("addLanguage")}
      </Button>
    </div>
  );
}

function ExperienceRows({
  rows,
  onChange,
}: {
  rows: ProfileFormValues["experience"];
  onChange: (rows: ProfileFormValues["experience"]) => void;
}) {
  const t = useTranslations("profileEditor");
  function update(i: number, patch: Partial<ProfileFormValues["experience"][number]>) {
    onChange(rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));
  }
  return (
    <div className="flex flex-col gap-3">
      {rows.map((row, i) => (
        <div key={i} className="flex flex-wrap items-end gap-2">
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("title")}</Label>
            <Input
              className="w-44"
              value={row.title}
              onChange={(e) => update(i, { title: e.target.value })}
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("organisation")}</Label>
            <Input
              className="w-44"
              value={row.org}
              onChange={(e) => update(i, { org: e.target.value })}
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("period")}</Label>
            <Input
              className="w-40"
              value={row.period}
              onChange={(e) => update(i, { period: e.target.value })}
              placeholder={t("periodPlaceholder")}
            />
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => onChange(rows.filter((_, idx) => idx !== i))}
            aria-label={t("removeExperience")}
          >
            <Trash2 />
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="self-start"
        onClick={() =>
          onChange([...rows, { title: "", org: "", period: "" }])
        }
      >
        <Plus /> {t("addExperience")}
      </Button>
    </div>
  );
}
