"use client";

import { useEffect, useState } from "react";
import { X } from "lucide-react";
import {
  AdminError,
  assignRole,
  listRoles,
  unassignRole,
  type Role,
} from "@/features/admin/api";
import { PERMISSIONS } from "@/features/admin/permissions";
import { useCan } from "@/features/admin/use-can";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

type RoleRef = { id: string; name: string };

/**
 * The roles a user holds, with add / remove controls for anyone who has
 * `users.manage_roles`. The backend enforces the real permission; this only
 * decides whether to render the controls.
 */
export function UserRolesCard({
  userId,
  roles,
  onChanged,
}: {
  userId: string;
  roles: RoleRef[];
  onChanged: () => void;
}) {
  const { can } = useCan();
  const manage = can(PERMISSIONS.usersManageRoles);

  const [allRoles, setAllRoles] = useState<Role[] | null>(null);
  const [toAdd, setToAdd] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!manage) return;
    let alive = true;
    async function load() {
      try {
        const r = await listRoles();
        if (alive) setAllRoles(r);
      } catch {
        if (alive) setAllRoles([]);
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [manage]);

  const held = new Set(roles.map((r) => r.id));
  const addable = (allRoles ?? []).filter((r) => !held.has(r.id));

  async function run(key: string, fn: () => Promise<void>) {
    setBusy(key);
    setErr(null);
    try {
      await fn();
      onChanged();
    } catch (e) {
      setErr(
        e instanceof AdminError ? e.message : "That change failed. Try again.",
      );
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="rounded-2xl border border-border bg-card p-6">
      <h3 className="font-display text-lg">Roles</h3>

      {roles.length === 0 ? (
        <p className="mt-2 text-sm text-muted-foreground">
          No roles — an ordinary user.
        </p>
      ) : (
        <ul className="mt-3 flex flex-wrap gap-2">
          {roles.map((r) => (
            <li
              key={r.id}
              className="inline-flex items-center gap-1.5 rounded-full bg-primary/15 py-1 pl-3 pr-1.5 text-xs font-semibold text-primary"
            >
              {r.name}
              {manage && (
                <button
                  type="button"
                  aria-label={`Remove ${r.name}`}
                  disabled={busy !== null}
                  onClick={() =>
                    run(`rm:${r.id}`, () => unassignRole(userId, r.id))
                  }
                  className="rounded-full p-0.5 hover:bg-primary/20 disabled:opacity-50"
                >
                  <X className="size-3" />
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      {manage && addable.length > 0 && (
        <div className="mt-4 flex flex-wrap items-center gap-2">
          <Select value={toAdd} onValueChange={setToAdd}>
            <SelectTrigger className="w-56">
              <SelectValue placeholder="Add a role…" />
            </SelectTrigger>
            <SelectContent>
              {addable.map((r) => (
                <SelectItem key={r.id} value={r.id}>
                  {r.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            size="sm"
            disabled={!toAdd || busy !== null}
            onClick={() =>
              run("add", async () => {
                await assignRole(userId, toAdd);
                setToAdd("");
              })
            }
          >
            {busy === "add" ? "Adding…" : "Add"}
          </Button>
        </div>
      )}

      {err && (
        <p className="mt-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {err}
        </p>
      )}
    </div>
  );
}
