"use client";

/**
 * Teacher dashboard: profile editor + weekly availability, behind tabs.
 * Loads the caller's own teacher profile once (`GET /v1/teachers/me`); the
 * availability tab needs a profile to exist first (it keys off the slug).
 */

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/features/auth/auth-context";
import { getMyProfile } from "@/features/dashboard/api";
import { ProfileWizard } from "@/features/dashboard/components/profile-wizard";
import { AvailabilityEditor } from "@/features/availability/components/availability-editor";
import { EarningsPanel } from "@/features/dashboard/components/earnings-panel";
import type { TeacherProfile } from "@/types/teacher";
import { Button } from "@/components/ui/button";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";

export function DashboardShell() {
  const { user } = useAuth();
  const [profile, setProfile] = useState<TeacherProfile | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const t = useTranslations("dashboard");

  useEffect(() => {
    let alive = true;
    getMyProfile()
      .then((p) => {
        if (!alive) return;
        setProfile(p);
        setState("ready");
      })
      .catch(() => alive && setState("error"));
    return () => {
      alive = false;
    };
  }, []);

  const handleSaved = useCallback((p: TeacherProfile) => setProfile(p), []);

  if (!user) return null;

  return (
    <div className="mx-auto max-w-3xl px-4 py-12 sm:px-6">
      <header className="mb-8">
        <h1 className="font-display text-3xl">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("signedInAs", { name: user.displayName })}</p>
      </header>

      {state === "loading" && (
        <div className="h-96 animate-pulse rounded-2xl bg-muted" />
      )}

      {state === "error" && (
        <p className="rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {t("couldNotLoad")}
        </p>
      )}

      {state === "ready" && profile && <ModerationBanner profile={profile} />}

      {/* A brand-new profile is `pending` and can't be used yet — show only the
          review notice above and a way back, not the whole editor. */}
      {state === "ready" && profile?.status === "pending" && (
        <Button asChild variant="outline">
          <Link href="/">{t("backHome")}</Link>
        </Button>
      )}

      {state === "ready" && profile?.status !== "pending" && (
        <Tabs defaultValue="profile">
          <TabsList>
            <TabsTrigger value="profile">{t("tabProfile")}</TabsTrigger>
            <TabsTrigger value="availability">{t("tabAvailability")}</TabsTrigger>
            <TabsTrigger value="earnings">{t("tabEarnings")}</TabsTrigger>
          </TabsList>

          <TabsContent value="profile" className="mt-6">
            <ProfileWizard initial={profile} onSaved={handleSaved} />
          </TabsContent>

          <TabsContent value="availability" className="mt-6">
            {profile ? (
              <AvailabilityEditor slug={profile.slug} />
            ) : (
              <p className="rounded-xl bg-accent/60 px-4 py-3 text-sm text-accent-foreground">
                {t("needProfileAvailability")}
              </p>
            )}
          </TabsContent>

          <TabsContent value="earnings" className="mt-6">
            {profile ? (
              <EarningsPanel />
            ) : (
              <p className="rounded-xl bg-accent/60 px-4 py-3 text-sm text-accent-foreground">
                {t("needProfileEarnings")}
              </p>
            )}
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}

function ModerationBanner({ profile }: { profile: TeacherProfile }) {
  const t = useTranslations("dashboard");
  if (profile.status === "approved") return null;

  const copy: Record<string, { title: string; body: string; tone: string }> = {
    pending: {
      title: t("pendingTitle"),
      body: t("pendingBody"),
      tone: "bg-star/10 text-star",
    },
    rejected: {
      title: t("rejectedTitle"),
      // The moderator's note says what to fix; the hint says saving resubmits.
      body: profile.moderationNote
        ? `${profile.moderationNote} — ${t("rejectedBody")}`
        : t("rejectedBody"),
      tone: "bg-destructive/10 text-destructive",
    },
    suspended: {
      title: t("suspendedTitle"),
      body: profile.moderationNote || t("suspendedBody"),
      tone: "bg-destructive/10 text-destructive",
    },
  };
  const c = copy[profile.status];
  if (!c) return null;

  return (
    <div className={`mb-6 rounded-xl px-4 py-3 text-sm ${c.tone}`}>
      <div className="font-semibold">{c.title}</div>
      <div className="mt-0.5 opacity-90">{c.body}</div>
    </div>
  );
}
