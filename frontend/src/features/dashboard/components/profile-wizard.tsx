"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Check, MailWarning, Plus, Trash2, X } from "lucide-react";
import { useAuth } from "@/features/auth/auth-context";
import { resendVerification } from "@/features/auth/api";
import {
  createMyProfile,
  emptyProfileForm,
  ProfileError,
  profileToForm,
  updateMyProfile,
  type ProfileFormValues,
} from "@/features/dashboard/api";
import { MediaStep } from "@/features/dashboard/components/media-step";
import { Field, Section } from "@/features/dashboard/components/form-bits";
import type { LanguageLevel, TeacherProfile } from "@/types/teacher";
import { COUNTRY_CODES, LANGUAGE_CODES, isoLanguageName, regionName } from "@/lib/iso";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Combobox } from "@/components/ui/combobox";
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
const STEPS = ["about", "teaching", "media"] as const;
type Step = (typeof STEPS)[number];

/** whole so'm <-> minor units (tiyin, ×100) */
const toSom = (minor: number) => (minor ? Math.round(minor / 100) : 0);
const toMinor = (som: number) => Math.round(som * 100);

/**
 * Which step the first missing required value lives on, or null when the
 * form can be submitted. Mirrors the backend's required set so a save never
 * bounces on something the wizard could have caught; browser `required`
 * handles the per-step case, this handles cross-step jumps in edit mode.
 */
function firstIncompleteStep(v: ProfileFormValues): Step | null {
  const aboutOk =
    v.displayName.trim() &&
    v.headline.trim() &&
    v.city.trim() &&
    v.countryCode &&
    v.timezone &&
    v.languages.some((l) => l.role === "teaches" && l.code);
  if (!aboutOk) return "about";
  return null;
}

export function ProfileWizard({
  initial,
  onSaved,
}: {
  initial: TeacherProfile | null;
  onSaved: (p: TeacherProfile) => void;
}) {
  const router = useRouter();
  const t = useTranslations("profileEditor");
  const { user } = useAuth();
  const isCreate = initial === null;
  // The backend refuses a new profile from an unconfirmed address (403
  // `email_not_verified`); say so up front rather than after three steps.
  const needsVerification = isCreate && !!user && !user.emailVerified;
  const [values, setValues] = useState<ProfileFormValues>(() =>
    initial ? profileToForm(initial) : emptyProfileForm(user?.displayName ?? ""),
  );
  const [step, setStep] = useState<Step>("about");
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<"idle" | "saving" | "saved">("idle");
  const formRef = useRef<HTMLFormElement>(null);
  const stepIndex = STEPS.indexOf(step);

  function set<K extends keyof ProfileFormValues>(key: K, val: ProfileFormValues[K]) {
    setValues((v) => ({ ...v, [key]: val }));
    setStatus("idle");
  }

  /** Browser constraint validation for the fields currently on screen. */
  function currentStepValid(): boolean {
    if (!formRef.current?.reportValidity()) return false;
    if (step === "about" && firstIncompleteStep(values) === "about") {
      setError(t("needTeachLanguage"));
      return false;
    }
    setError(null);
    return true;
  }

  function goTo(next: Step) {
    // Moving forward checks the step you're leaving; going back never does,
    // so a half-filled step is never a trap.
    if (STEPS.indexOf(next) > stepIndex && !currentStepValid()) return;
    setError(null);
    setStep(next);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!currentStepValid()) return;
    const incomplete = firstIncompleteStep(values);
    if (incomplete) {
      setStep(incomplete);
      setError(t("fillRequired"));
      return;
    }
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
      if (isCreate) window.scrollTo({ top: 0, behavior: "smooth" });
    } catch (err) {
      setStatus("idle");
      setError(err instanceof ProfileError ? err.message : t("somethingWrong"));
    }
  }

  const isLast = stepIndex === STEPS.length - 1;

  return (
    <form ref={formRef} onSubmit={handleSubmit} className="flex flex-col gap-8">
      {isCreate && (
        <p className="border border-border bg-muted px-4 py-3 text-sm text-muted-foreground">
          {t("noProfileYet")}
        </p>
      )}
      {needsVerification && <VerifyEmailNotice email={user.email} />}

      <StepRail
        current={step}
        // A new profile is filled in order; an existing one can be edited in
        // any order, so the rail becomes navigation.
        onSelect={isCreate ? undefined : goTo}
      />

      {step === "about" && <AboutStep values={values} set={set} />}
      {step === "teaching" && <TeachingStep values={values} set={set} />}
      {step === "media" && (
        <MediaStep values={values} set={set} profileStatus={initial?.status ?? null} />
      )}

      {error && (
        <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}

      <div className="sticky bottom-0 -mx-4 flex items-center gap-3 border-t border-border bg-background px-4 py-4 sm:-mx-6 sm:px-6">
        {stepIndex > 0 && (
          <Button type="button" variant="outline" onClick={() => goTo(STEPS[stepIndex - 1])}>
            {t("back")}
          </Button>
        )}
        <span className="text-sm text-muted-foreground">
          {t("stepOf", { n: stepIndex + 1, total: STEPS.length })}
        </span>
        <span className="flex-1" />
        {status === "saved" && (
          <span className="text-sm text-muted-foreground">{t("saved")}</span>
        )}
        {!isCreate && !isLast && (
          <Button type="submit" variant="ghost" disabled={status === "saving"}>
            {status === "saving" ? t("saving") : t("saveChanges")}
          </Button>
        )}
        {isLast ? (
          <Button type="submit" disabled={status === "saving" || needsVerification}>
            {status === "saving"
              ? t("saving")
              : isCreate
                ? t("createProfile")
                : t("saveChanges")}
          </Button>
        ) : (
          <Button type="button" onClick={() => goTo(STEPS[stepIndex + 1])}>
            {t("next")}
          </Button>
        )}
      </div>
    </form>
  );
}

/* ----------------------------------------------------------------------- */
/* step rail                                                                */
/* ----------------------------------------------------------------------- */

function StepRail({ current, onSelect }: { current: Step; onSelect?: (s: Step) => void }) {
  const t = useTranslations("profileEditor");
  const currentIndex = STEPS.indexOf(current);
  return (
    <ol className="grid grid-cols-3 gap-2" aria-label={t("stepsLabel")}>
      {STEPS.map((s, i) => {
        const state = i < currentIndex ? "done" : i === currentIndex ? "current" : "todo";
        const inner = (
          <>
            <span
              className={cn(
                "grid size-7 shrink-0 place-items-center rounded-full text-sm font-bold",
                state === "current" && "bg-primary text-primary-foreground",
                state === "done" && "bg-foreground text-background",
                state === "todo" && "border border-border text-muted-foreground",
              )}
            >
              {state === "done" ? <Check className="size-4" /> : i + 1}
            </span>
            <span
              className={cn(
                "truncate text-sm font-bold",
                state === "todo" && "text-muted-foreground",
              )}
            >
              {t(`step_${s}`)}
            </span>
          </>
        );
        const cls = cn(
          "flex items-center gap-2.5 border px-3 py-2.5 text-left",
          state === "current" ? "border-foreground bg-card" : "border-border bg-card",
        );
        return (
          <li key={s} aria-current={state === "current" ? "step" : undefined}>
            {onSelect ? (
              <button type="button" onClick={() => onSelect(s)} className={cn(cls, "w-full")}>
                {inner}
              </button>
            ) : (
              <div className={cls}>{inner}</div>
            )}
          </li>
        );
      })}
    </ol>
  );
}

/* ----------------------------------------------------------------------- */
/* step 1 — about you                                                       */
/* ----------------------------------------------------------------------- */

type Setter = <K extends keyof ProfileFormValues>(key: K, val: ProfileFormValues[K]) => void;

function AboutStep({ values, set }: { values: ProfileFormValues; set: Setter }) {
  const t = useTranslations("profileEditor");
  const locale = useLocale();
  const countries = COUNTRY_CODES.map((code) => ({
    value: code,
    label: regionName(code, locale),
    keywords: regionName(code, "en"),
  }));

  return (
    <div className="flex flex-col gap-8">
      <Section title={t("basics")}>
        <Field label={t("displayName")} htmlFor="displayName">
          <Input
            id="displayName"
            value={values.displayName}
            maxLength={120}
            onChange={(e) => set("displayName", e.target.value)}
            required
          />
        </Field>
        <Field label={t("headline")} htmlFor="headline" hint={t("headlineHint")}>
          <Input
            id="headline"
            value={values.headline}
            maxLength={120}
            placeholder={t("headlinePlaceholder")}
            onChange={(e) => set("headline", e.target.value)}
            required
          />
        </Field>
        <div className="flex flex-col gap-1.5">
          <Label>{t("teacherType")}</Label>
          <div className="grid gap-3 sm:grid-cols-2">
            <KindCard
              selected={values.kind === "professional"}
              title={t("professional")}
              body={t("professionalHint")}
              onClick={() => set("kind", "professional")}
            />
            <KindCard
              selected={values.kind === "community"}
              title={t("communityTutor")}
              body={t("communityHint")}
              onClick={() => set("kind", "community")}
            />
          </div>
        </div>
      </Section>

      <Section title={t("languages")} hint={t("languagesHint")}>
        <LanguageRows rows={values.languages} onChange={(rows) => set("languages", rows)} />
      </Section>

      <Section title={t("locationTime")}>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("country")} htmlFor="country">
            <Combobox
              id="country"
              value={values.countryCode}
              options={countries}
              onChange={(code) => {
                set("countryCode", code);
                set("countryName", regionName(code, "en"));
              }}
              placeholder={t("selectCountry")}
              searchPlaceholder={t("searchCountries")}
              emptyText={t("noMatch")}
            />
          </Field>
          <Field label={t("city")} htmlFor="city">
            <Input
              id="city"
              value={values.city}
              maxLength={120}
              onChange={(e) => set("city", e.target.value)}
              required
            />
          </Field>
          <div className="sm:col-span-2">
            <Field label={t("timezone")} htmlFor="timezone" hint={t("timezoneHint")}>
              <TimezoneSelect
                id="timezone"
                value={values.timezone}
                onChange={(tz) => set("timezone", tz)}
              />
            </Field>
          </div>
        </div>
      </Section>
    </div>
  );
}

function KindCard({
  selected,
  title,
  body,
  onClick,
}: {
  selected: boolean;
  title: string;
  body: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={onClick}
      className={cn(
        "flex flex-col gap-1 rounded-md border px-4 py-3 text-left transition-colors",
        selected
          ? "border-foreground bg-accent"
          : "border-border hover:bg-muted",
      )}
    >
      <span className="font-bold">{title}</span>
      <span className="text-sm text-muted-foreground">{body}</span>
    </button>
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
  const locale = useLocale();
  // A stored code outside the curated list (an old profile, a seed row)
  // still has to show and stay selectable.
  const codes = Array.from(new Set([...LANGUAGE_CODES, ...rows.map((r) => r.code).filter(Boolean)]));
  const options = codes.map((code) => ({
    value: code,
    label: isoLanguageName(code, locale),
    hint: code,
    keywords: isoLanguageName(code, "en"),
  }));

  function update(i: number, patch: Partial<ProfileFormValues["languages"][number]>) {
    onChange(rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));
  }

  return (
    <div className="flex flex-col gap-3">
      {rows.map((row, i) => (
        <div
          key={i}
          className="grid grid-cols-[1fr_auto] items-end gap-2 sm:grid-cols-[130px_1fr_120px_auto]"
        >
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("role")}</Label>
            <Select
              value={row.role}
              onValueChange={(v) => update(i, { role: v as "teaches" | "also_speaks" })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="teaches">{t("teaches")}</SelectItem>
                <SelectItem value="also_speaks">{t("alsoSpeaks")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="col-span-2 flex flex-col gap-1 sm:col-span-1">
            <Label className="text-xs">{t("language")}</Label>
            <Combobox
              value={row.code}
              options={options}
              onChange={(code) => update(i, { code, name: isoLanguageName(code, "en") })}
              placeholder={t("selectLanguage")}
              searchPlaceholder={t("searchLanguages")}
              emptyText={t("noMatch")}
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("level")}</Label>
            <Select value={row.level} onValueChange={(v) => update(i, { level: v as LanguageLevel })}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {LEVELS.map((l) => (
                  <SelectItem key={l} value={l}>
                    {t(`level_${l}`)}
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
            {
              role: rows.some((r) => r.role === "teaches") ? "also_speaks" : "teaches",
              code: "",
              name: "",
              level: rows.length === 0 ? "native" : "c1",
            },
          ])
        }
      >
        <Plus /> {t("addLanguage")}
      </Button>
    </div>
  );
}

/* ----------------------------------------------------------------------- */
/* step 2 — teaching                                                        */
/* ----------------------------------------------------------------------- */

function TeachingStep({ values, set }: { values: ProfileFormValues; set: Setter }) {
  const t = useTranslations("profileEditor");
  return (
    <div className="flex flex-col gap-8">
      <Section title={t("pricing")} hint={t("pricingHint")}>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("pricePerHour")} htmlFor="price">
            <Input
              id="price"
              type="number"
              min={0}
              step={1000}
              inputMode="numeric"
              value={toSom(values.pricePerHourMinor) || ""}
              onChange={(e) => set("pricePerHourMinor", toMinor(Number(e.target.value)))}
              required
            />
          </Field>
          <Field label={t("trialPrice")} htmlFor="trial" hint={t("trialPriceHint")}>
            <Input
              id="trial"
              type="number"
              min={0}
              step={1000}
              inputMode="numeric"
              value={values.trialPriceMinor === null ? "" : toSom(values.trialPriceMinor)}
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

      <Section title={t("focusAreas")} hint={t("focusHint")}>
        <TagInput
          tags={values.focus}
          onChange={(tags) => set("focus", tags)}
          placeholder={t("focusPlaceholder")}
          removeLabel={t("removeTag")}
          max={20}
          maxLength={40}
        />
      </Section>

      <Section title={t("about")}>
        <Field label={t("bio")} htmlFor="about" hint={t("bioHint")}>
          <Textarea
            id="about"
            rows={5}
            maxLength={4000}
            value={values.about}
            onChange={(e) => set("about", e.target.value)}
          />
        </Field>
        <Field label={t("teachingStyle")} htmlFor="style" hint={t("teachingStyleHint")}>
          <Textarea
            id="style"
            rows={3}
            maxLength={4000}
            value={values.teachingStyle}
            onChange={(e) => set("teachingStyle", e.target.value)}
          />
        </Field>
      </Section>

      <Section title={t("experience")} hint={t("experienceHint")}>
        <ExperienceRows rows={values.experience} onChange={(rows) => set("experience", rows)} />
      </Section>
    </div>
  );
}

function Textarea(props: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      {...props}
      className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
    />
  );
}

/** Chips + a text box: Enter or comma adds, Backspace on empty removes the last. */
function TagInput({
  tags,
  onChange,
  placeholder,
  removeLabel,
  max,
  maxLength,
}: {
  tags: string[];
  onChange: (tags: string[]) => void;
  placeholder: string;
  removeLabel: string;
  max: number;
  maxLength: number;
}) {
  const [draft, setDraft] = useState("");

  function commit() {
    const tag = draft.trim().slice(0, maxLength);
    if (tag && !tags.includes(tag) && tags.length < max) onChange([...tags, tag]);
    setDraft("");
  }

  return (
    <div className="flex min-h-9 flex-wrap items-center gap-1.5 rounded-md border border-input px-2 py-1.5 focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50">
      {tags.map((tag) => (
        <span
          key={tag}
          className="inline-flex items-center gap-1 rounded-sm bg-accent px-2 py-0.5 text-sm font-bold text-accent-foreground"
        >
          {tag}
          <button
            type="button"
            aria-label={`${removeLabel}: ${tag}`}
            onClick={() => onChange(tags.filter((x) => x !== tag))}
            className="rounded-sm hover:bg-accent-foreground/10"
          >
            <X className="size-3.5" />
          </button>
        </span>
      ))}
      <input
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === ",") {
            e.preventDefault();
            commit();
          } else if (e.key === "Backspace" && draft === "" && tags.length) {
            onChange(tags.slice(0, -1));
          }
        }}
        placeholder={tags.length ? "" : placeholder}
        maxLength={maxLength}
        className="min-w-32 flex-1 bg-transparent px-1 text-sm outline-none"
      />
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
        <div key={i} className="grid grid-cols-[1fr_auto] items-end gap-2 sm:grid-cols-[1fr_1fr_150px_auto]">
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("title")}</Label>
            <Input
              value={row.title}
              maxLength={120}
              placeholder={t("titlePlaceholder")}
              onChange={(e) => update(i, { title: e.target.value })}
              required
            />
          </div>
          <div className="col-span-2 flex flex-col gap-1 sm:col-span-1">
            <Label className="text-xs">{t("organisation")}</Label>
            <Input
              value={row.org}
              maxLength={120}
              placeholder={t("orgPlaceholder")}
              onChange={(e) => update(i, { org: e.target.value })}
              required
            />
          </div>
          <div className="flex flex-col gap-1">
            <Label className="text-xs">{t("period")}</Label>
            <Input
              value={row.period}
              maxLength={120}
              onChange={(e) => update(i, { period: e.target.value })}
              placeholder={t("periodPlaceholder")}
              required
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
        onClick={() => onChange([...rows, { title: "", org: "", period: "" }])}
      >
        <Plus /> {t("addExperience")}
      </Button>
    </div>
  );
}

/** Shown above the create form while the account's email is unconfirmed. */
function VerifyEmailNotice({ email }: { email: string }) {
  const t = useTranslations("profileEditor");
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);

  async function resend() {
    setBusy(true);
    try {
      await resendVerification();
    } catch {
      // 409 already-verified / rate-limited both mean "nothing more to do".
    }
    setSent(true);
    setBusy(false);
  }

  return (
    <div
      role="status"
      className="flex items-start gap-3 rounded-md border border-star/40 bg-star/10 px-4 py-3 text-sm text-rating dark:text-star"
    >
      <MailWarning className="mt-0.5 size-4 shrink-0" />
      <div className="flex-1">
        <div className="font-bold">{t("verifyFirstTitle")}</div>
        <div className="mt-0.5 opacity-90">
          {sent ? t("verifySent") : t("verifyFirstBody", { email })}
        </div>
        {!sent && (
          <button
            type="button"
            disabled={busy}
            onClick={resend}
            className="mt-1 font-bold underline underline-offset-2 disabled:opacity-60"
          >
            {t("verifyResend")}
          </button>
        )}
      </div>
    </div>
  );
}
