"use client";

import { useRef, useState } from "react";
import { Paperclip, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ResourceError, uploadFile } from "@/features/resources/api";

/**
 * Upload one file → returns its file-asset id. Shows the current file with a
 * remove button once set. `accept` narrows the picker (e.g. "audio/*").
 */
export function FileField({
  value,
  filename,
  accept,
  label,
  onChange,
}: {
  value: string | undefined;
  filename?: string;
  accept?: string;
  label: string;
  onChange: (id: string | undefined, filename?: string) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function pick(file: File) {
    setError(null);
    setBusy(true);
    try {
      const up = await uploadFile(file);
      onChange(up.id, up.filename);
    } catch (err) {
      setError(err instanceof ResourceError ? err.message : "Upload failed.");
    }
    setBusy(false);
  }

  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-sm font-medium">{label}</span>
      {value ? (
        <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/40 px-3 py-2 text-sm">
          <Paperclip className="size-4 shrink-0 text-muted-foreground" />
          <span className="truncate">{filename ?? "Uploaded file"}</span>
          <button
            type="button"
            aria-label="Remove file"
            className="ml-auto rounded p-0.5 hover:bg-muted"
            onClick={() => onChange(undefined, undefined)}
          >
            <X className="size-4" />
          </button>
        </div>
      ) : (
        <div>
          <input
            ref={inputRef}
            type="file"
            accept={accept}
            className="hidden"
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) void pick(f);
              e.target.value = "";
            }}
          />
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={busy}
            onClick={() => inputRef.current?.click()}
          >
            {busy ? "Uploading…" : "Choose a file"}
          </Button>
        </div>
      )}
      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
