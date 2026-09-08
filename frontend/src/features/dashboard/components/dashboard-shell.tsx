"use client";

/**
 * Teacher dashboard shell. Two panes — profile and availability — behind tabs.
 *
 * The editors themselves are wired against `GET/POST /v1/teachers/me` and
 * `PUT /v1/teachers/{slug}/availability`. Until the profile-write endpoints
 * merge, this renders the layout plus a "coming together" notice so the route
 * and navigation are real.
 */

import { useAuth } from "@/features/auth/auth-context";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";

export function DashboardShell() {
  const { user } = useAuth();
  if (!user) return null;

  return (
    <div className="mx-auto max-w-4xl px-4 py-12 sm:px-6">
      <header className="mb-8">
        <h1 className="font-display text-3xl">Teacher dashboard</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Signed in as {user.displayName}.
        </p>
      </header>

      <Tabs defaultValue="profile">
        <TabsList>
          <TabsTrigger value="profile">Profile</TabsTrigger>
          <TabsTrigger value="availability">Availability</TabsTrigger>
        </TabsList>

        <TabsContent value="profile" className="mt-6">
          <Placeholder label="Profile editor" />
        </TabsContent>

        <TabsContent value="availability" className="mt-6">
          <Placeholder label="Weekly availability editor" />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function Placeholder({ label }: { label: string }) {
  return (
    <div className="rounded-2xl border border-dashed border-border bg-card/50 p-10 text-center">
      <p className="font-medium">{label}</p>
      <p className="mt-1 text-sm text-muted-foreground">
        Lands with the teacher-profile write endpoints (in progress).
      </p>
    </div>
  );
}
