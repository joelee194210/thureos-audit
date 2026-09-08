"use client";

export default function DashboardError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 p-8 text-center">
      <p className="text-sm font-medium text-destructive">
        Ocurrió un error al cargar esta página
      </p>
      <p className="text-xs text-muted-foreground">
        {error.message || "Error inesperado."}
      </p>
      <button
        onClick={reset}
        className="mt-2 rounded-md border px-3 py-1.5 text-xs font-medium hover:bg-accent"
      >
        Reintentar
      </button>
    </div>
  );
}
