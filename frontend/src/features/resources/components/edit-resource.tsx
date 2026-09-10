"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { getResource, ResourceError } from "@/features/resources/api";
import type { Resource } from "@/features/resources/types";
import { ResourceEditor } from "./resource-editor";

export function EditResource({ id }: { id: string }) {
  const [resource, setResource] = useState<Resource | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [msg, setMsg] = useState("Could not load that resource.");

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
        {msg}{" "}
        <Link href="/resources" className="font-medium underline">
          Back to resources
        </Link>
      </div>
    );
  }
  return <ResourceEditor type={resource.type} initial={resource} />;
}
