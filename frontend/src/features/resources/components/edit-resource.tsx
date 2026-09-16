"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { getResource, ResourceError } from "@/features/resources/api";
import type { Resource } from "@/features/resources/types";
import { ResourceEditor } from "./resource-editor";

export function EditResource({ id }: { id: string }) {
  const [resource, setResource] = useState<Resource | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [msg, setMsg] = useState<string | null>(null);
  const t = useTranslations("resources");

  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const r = await getResource(id);
        if (!alive) return;
        setResource(r);
        setState("ready");
      } catch (err) {
        if (!alive) return;
        if (err instanceof ResourceError) setMsg(err.message);
        setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [id]);

  if (state === "loading") {
    return <div className="h-96 animate-pulse rounded-2xl bg-muted" />;
  }
  if (state === "error" || !resource) {
    return (
      <div className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {msg ?? t("couldNotLoadOne")}{" "}
        <Link href="/resources" className="font-medium underline">
          {t("backToResources")}
        </Link>
      </div>
    );
  }
  return <ResourceEditor type={resource.type} initial={resource} />;
}
