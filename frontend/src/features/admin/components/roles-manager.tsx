"use client";

import { useEffect, useState } from "react";
import { ChevronDown, Lock, Plus, Trash2 } from "lucide-react";
import {
  AdminError,
  createRole,
  deleteRole,
  getPermissionCatalog,
  listRoles,
  updateRole,
  type PermissionInfo,
  type Role,
} from "@/features/admin/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export function RolesManager() {
  const [roles, setRoles] = useState<Role[] | null>(null);
  const [catalog, setCatalog] = useState<PermissionInfo[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [creating, setCreating] = useState(false);

  async function reload() {
    const [r, c] = await Promise.all([listRoles(), getPermissionCatalog()]);
    setRoles(r);
    setCatalog(c);
    setState("ready");
  }

  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const [r, c] = await Promise.all([listRoles(), getPermissionCatalog()]);
        if (!alive) return;
        setRoles(r);
        setCatalog(c);
        setState("ready");
      } catch {
        if (alive) setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, []);

  if (state === "loading") {
    return <div className="h-72 animate-pulse rounded-2xl bg-muted" />;
  }
  if (state === "error" || !roles) {
    return (
      <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
        Could not load roles.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          A role bundles permissions. Assign roles to users on their detail page.
        </p>
        {!creating && (
          <Button size="sm" onClick={() => setCreating(true)}>
            <Plus className="size-4" /> New role
          </Button>
        )}
      </div>

      {creating && (
        <RoleCreateForm
          catalog={catalog}
          onDone={async (saved) => {
            setCreating(false);
            if (saved) await reload();
          }}
        />
      )}

      <ul className="flex flex-col gap-3">
        {roles.map((role) => (
          <RoleCard
            key={role.id}
            role={role}
            catalog={catalog}
            onChanged={reload}
          />
        ))}
      </ul>
    </div>
  );
}

/* --------------------------------- card --------------------------------- */

function RoleCard({
  role,
  catalog,
  onChanged,
}: {
  role: Role;
  catalog: PermissionInfo[];
  onChanged: () => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const [description, setDescription] = useState(role.description);
  const [perms, setPerms] = useState<string[]>(role.permissions);
  const [busy, setBusy] = useState<"save" | "delete" | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const dirty =
    description !== role.description ||
    perms.length !== role.permissions.length ||
    perms.some((p) => !role.permissions.includes(p));

  async function save() {
    setBusy("save");
    setErr(null);
    try {
      await updateRole(role.id, {
        description,
        permissions: role.isSystem ? undefined : perms,
      });
      await onChanged();
      setOpen(false);
    } catch (e) {
      setErr(e instanceof AdminError ? e.message : "Could not save the role.");
    } finally {
      setBusy(null);
    }
  }

  async function remove() {
    setBusy("delete");
    setErr(null);
    try {
      await deleteRole(role.id);
      await onChanged();
    } catch (e) {
      setErr(e instanceof AdminError ? e.message : "Could not delete the role.");
      setBusy(null);
    }
  }

  return (
    <li className="rounded-2xl border border-border bg-card">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center gap-3 px-5 py-4 text-left"
      >
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <span className="font-medium">{role.name}</span>
            {role.isSystem && (
              <span className="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                <Lock className="size-3" /> system
              </span>
            )}
          </div>
          <p className="mt-0.5 text-sm text-muted-foreground">
            {role.description || "No description."}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            {role.permissions.length} permission
            {role.permissions.length === 1 ? "" : "s"} · {role.userCount} user
            {role.userCount === 1 ? "" : "s"}
          </p>
        </div>
        <ChevronDown
          className={cn(
            "size-4 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-180",
          )}
        />
      </button>

      {open && (
        <div className="border-t border-border px-5 py-4">
          <label className="text-xs font-medium text-muted-foreground">
            Description
          </label>
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="mt-1"
          />

          <p className="mt-4 text-xs font-medium text-muted-foreground">
            Permissions{role.isSystem && " (locked for a system role)"}
          </p>
          <PermissionChecklist
            catalog={catalog}
            selected={perms}
            disabled={role.isSystem}
            onToggle={(key) =>
              setPerms((cur) =>
                cur.includes(key)
                  ? cur.filter((k) => k !== key)
                  : [...cur, key],
              )
            }
          />

          {err && (
            <p className="mt-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {err}
            </p>
          )}

          <div className="mt-4 flex items-center gap-2">
            <Button size="sm" disabled={!dirty || busy !== null} onClick={save}>
              {busy === "save" ? "Saving…" : "Save changes"}
            </Button>
            {!role.isSystem && (
              <Button
                size="sm"
                variant="ghost"
                className="text-destructive hover:text-destructive"
                disabled={busy !== null || role.userCount > 0}
                title={
                  role.userCount > 0
                    ? "Unassign this role from every user first"
                    : undefined
                }
                onClick={remove}
              >
                <Trash2 className="size-4" />
                {busy === "delete" ? "Deleting…" : "Delete"}
              </Button>
            )}
          </div>
        </div>
      )}
    </li>
  );
}

/* ------------------------------- create --------------------------------- */

function RoleCreateForm({
  catalog,
  onDone,
}: {
  catalog: PermissionInfo[];
  onDone: (saved: boolean) => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [perms, setPerms] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit() {
    setBusy(true);
    setErr(null);
    try {
      await createRole({ name: name.trim(), description: description.trim(), permissions: perms });
      onDone(true);
    } catch (e) {
      setErr(e instanceof AdminError ? e.message : "Could not create the role.");
      setBusy(false);
    }
  }

  return (
    <div className="rounded-2xl border border-border bg-card p-5">
      <h3 className="font-display text-lg">New role</h3>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <div>
          <label className="text-xs font-medium text-muted-foreground">Name</label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. moderator"
            className="mt-1"
          />
        </div>
        <div>
          <label className="text-xs font-medium text-muted-foreground">
            Description
          </label>
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="mt-1"
          />
        </div>
      </div>

      <p className="mt-4 text-xs font-medium text-muted-foreground">Permissions</p>
      <PermissionChecklist
        catalog={catalog}
        selected={perms}
        onToggle={(key) =>
          setPerms((cur) =>
            cur.includes(key) ? cur.filter((k) => k !== key) : [...cur, key],
          )
        }
      />

      {err && (
        <p className="mt-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {err}
        </p>
      )}

      <div className="mt-4 flex gap-2">
        <Button
          size="sm"
          disabled={name.trim() === "" || busy}
          onClick={submit}
        >
          {busy ? "Creating…" : "Create role"}
        </Button>
        <Button size="sm" variant="outline" onClick={() => onDone(false)}>
          Cancel
        </Button>
      </div>
    </div>
  );
}

/* ----------------------------- checklist -------------------------------- */

function PermissionChecklist({
  catalog,
  selected,
  disabled = false,
  onToggle,
}: {
  catalog: PermissionInfo[];
  selected: string[];
  disabled?: boolean;
  onToggle: (key: string) => void;
}) {
  return (
    <div className="mt-2 grid gap-1.5 sm:grid-cols-2">
      {catalog.map((p) => {
        const on = selected.includes(p.key);
        return (
          <label
            key={p.key}
            className={cn(
              "flex items-start gap-2 rounded-lg border border-border px-3 py-2 text-sm",
              disabled ? "opacity-60" : "cursor-pointer hover:bg-muted/50",
              on && "border-primary/40 bg-primary/5",
            )}
          >
            <input
              type="checkbox"
              checked={on}
              disabled={disabled}
              onChange={() => onToggle(p.key)}
              className="mt-0.5 accent-primary"
            />
            <span>
              <span className="font-mono text-xs">{p.key}</span>
              <span className="block text-xs text-muted-foreground">
                {p.description}
              </span>
            </span>
          </label>
        );
      })}
    </div>
  );
}
