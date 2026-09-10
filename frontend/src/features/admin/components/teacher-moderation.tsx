"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, BadgeCheck, ExternalLink } from "lucide-react";
import { formatMoney } from "@/lib/format";
import {
  AdminError,
  approveTeacher,
  getTeacher,
  rejectTeacher,
  setTeacherVerified,
  suspendTeacher,
  type AdminTeacherDetail,
} from "@/features/admin/api";
import { Button } from "@/components/ui/button";
import { TeacherStatusBadge } from "./teacher-status-badge";

type Busy = "approve" | "reject" | "suspend" | "verify" | null;

export function TeacherModeration({ slug }: { slug: string }) {
  const [data, setData] = useState<AdminTeacherDetail | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [errMsg, setErrMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState<Busy>(null);
  const [noteMode, setNoteMode] = useState<"reject" | "suspend" | null>(null);
  const [note, setNote] = useState("");

  async function reload() {
    setData(await getTeacher(slug));
  }

  useEffect(() => {
    let alive = true;
    getTeacher(slug)
      .then((d) => {
        if (!alive) return;
        setData(d);
        setState("ready");
      })
      .catch((err) => {
        if (!alive) return;
        setErrMsg(
          err instanceof AdminError ? err.message : "Could not load that teacher.",
        );
        setState("error");
      });
    return () => {
      alive = false;
    };
  }, [slug]);

  async function run(action: Busy, fn: () => Promise<unknown>) {
    setBusy(action);
    setErrMsg(null);
    try {
      await fn();
      await reload();
      setNoteMode(null);
      setNote("");
    } catch (err) {
      setErrMsg(
        err instanceof AdminError ? err.message : "That action failed. Try again.",
      );
    } finally {
      setBusy(null);
    }
  }

  if (state === "loading") {
    return <div className="h-96 animate-pulse rounded-2xl bg-muted" />;
  }
  if (state === "error" || !data) {
    return (
      <div>
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errMsg}
        </p>
        <Back />
      </div>
    );
  }

  const canApprove =
    data.status === "pending" ||
    data.status === "rejected" ||
    data.status === "suspended";
  const canReject = data.status === "pending";
  const canSuspend = data.status === "approved";

  return (
    <div className="flex flex-col gap-6">
      <Back />

      <div className="rounded-2xl border border-border bg-card p-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="inline-flex items-center gap-2 font-display text-2xl">
            {data.displayName}
            {data.verified && <BadgeCheck className="size-5 text-primary" />}
          </h2>
          <div className="flex items-center gap-3">
            <TeacherStatusBadge status={data.status} />
            {data.status === "approved" && (
              <Link
                href={`/teachers/${data.slug}`}
                target="_blank"
                className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
              >
                View live <ExternalLink className="size-3.5" />
              </Link>
            )}
          </div>
        </div>

        <p className="mt-1 text-sm text-muted-foreground">{data.headline}</p>

        <dl className="mt-4 grid gap-2 text-sm sm:grid-cols-2">
          <Row label="Owner">
            <Link
              href={`/admin/users/${data.owner.id}`}
              className="text-primary hover:underline"
            >
              {data.owner.displayName}
            </Link>{" "}
            <span className="text-muted-foreground">({data.owner.email})</span>
          </Row>
          <Row label="Location">
            {data.city}, {data.countryName} · {data.timezone}
          </Row>
          <Row label="Price / hour">{formatMoney(data.pricePerHour)}</Row>
          <Row label="Slug">
            <span className="font-mono text-xs">{data.slug}</span>
          </Row>
        </dl>

        {data.moderationNote && (
          <p className="mt-4 rounded-lg bg-muted px-3 py-2 text-sm">
            <span className="font-medium">Note to teacher:</span>{" "}
            {data.moderationNote}
          </p>
        )}
      </div>

      <div className="rounded-2xl border border-border bg-card p-6">
        <h3 className="font-display text-lg">About</h3>
        <p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-foreground/90">
          {data.about || "—"}
        </p>
        <h3 className="mt-4 font-display text-lg">How I teach</h3>
        <p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-foreground/90">
          {data.teachingStyle || "—"}
        </p>
      </div>

      {errMsg && (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errMsg}
        </p>
      )}

      {noteMode ? (
        <div className="flex flex-col gap-3 rounded-2xl border border-border bg-card p-6">
          <label htmlFor="note" className="text-sm font-medium">
            Reason ({noteMode}) — shown to the teacher
          </label>
          <textarea
            id="note"
            rows={3}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          />
          <div className="flex gap-2">
            <Button
              variant="destructive"
              disabled={busy !== null || note.trim() === ""}
              onClick={() =>
                run(
                  noteMode,
                  noteMode === "reject"
                    ? () => rejectTeacher(slug, note.trim())
                    : () => suspendTeacher(slug, note.trim()),
                )
              }
            >
              {busy ? "Working…" : `Confirm ${noteMode}`}
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                setNoteMode(null);
                setNote("");
              }}
            >
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <div className="flex flex-wrap gap-3">
          {canApprove && (
            <Button
              disabled={busy !== null}
              onClick={() => run("approve", () => approveTeacher(slug))}
            >
              {busy === "approve" ? "Approving…" : "Approve"}
            </Button>
          )}
          {canReject && (
            <Button
              variant="destructive"
              disabled={busy !== null}
              onClick={() => setNoteMode("reject")}
            >
              Reject
            </Button>
          )}
          {canSuspend && (
            <Button
              variant="destructive"
              disabled={busy !== null}
              onClick={() => setNoteMode("suspend")}
            >
              Suspend
            </Button>
          )}
          <Button
            variant="outline"
            disabled={busy !== null}
            onClick={() =>
              run("verify", () => setTeacherVerified(slug, !data.verified))
            }
          >
            {busy === "verify"
              ? "Working…"
              : data.verified
                ? "Remove verified badge"
                : "Mark verified"}
          </Button>
        </div>
      )}
    </div>
  );
}

function Back() {
  return (
    <Link
      href="/admin/teachers"
      className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground hover:text-foreground"
    >
      <ArrowLeft className="size-4" /> All teachers
    </Link>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="font-medium">{children}</dd>
    </div>
  );
}
