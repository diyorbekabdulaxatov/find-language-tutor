/**
 * Where to send someone after they sign in or sign up.
 *
 * `next` (a same-site absolute path) always wins — it's how RequireUser
 * bounces people back to what they were doing. Otherwise: a new account that
 * came through "Apply to teach" (`?role=teacher`) goes to the teacher
 * dashboard to create the profile; any other new account is a student and
 * goes to the teacher catalog; a returning user goes home.
 */
export type AuthMode = "login" | "signup";

export function safeNext(next: string | null | undefined): string | null {
  if (next && next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/\\")) {
    return next;
  }
  return null;
}

export function postAuthDestination(opts: {
  mode: AuthMode;
  next?: string | null;
  role?: string | null;
}): string {
  const next = safeNext(opts.next);
  if (next) return next;
  if (opts.mode === "signup") {
    return opts.role === "teacher" ? "/dashboard" : "/teachers";
  }
  return "/";
}
