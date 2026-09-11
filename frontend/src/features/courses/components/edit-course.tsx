"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { CourseError, getCourse } from "@/features/courses/api";
import type { CourseDetail } from "@/features/courses/types";
import { CourseEditor } from "./course-editor";

export function EditCourse({ id }: { id: string }) {
  const [course, setCourse] = useState<CourseDetail | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [msg, setMsg] = useState("Could not load that course.");

  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const c = await getCourse(id);
        if (!alive) return;
        setCourse(c);
        setState("ready");
      } catch (err) {
        if (!alive) return;
        if (err instanceof CourseError) setMsg(err.message);
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
  if (state === "error" || !course) {
    return (
      <div className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
        {msg}{" "}
        <Link href="/courses" className="font-medium underline">
          Back to courses
        </Link>
      </div>
    );
  }
  return <CourseEditor initial={course} />;
}
