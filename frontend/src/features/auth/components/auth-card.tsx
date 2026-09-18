import Link from "next/link";

/** The centered panel that wraps the login / signup forms. */
export function AuthCard({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="mx-auto flex min-h-[calc(100vh-8rem)] w-full max-w-sm flex-col justify-center px-4 py-12 sm:px-6">
      <Link href="/" className="mb-6 flex justify-center">
        <span className="font-display text-2xl tracking-tight text-primary">FindTutor</span>
      </Link>

      <h1 className="mb-6 text-center font-display text-2xl">{title}</h1>
      {children}
    </div>
  );
}
