import { Logo } from "@/components/brand/logo";

interface AuthShellProps {
  eyebrow: string;
  title: string;
  description: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
}

// Grabado de seguridad (guilloché): hairlines + anillos concéntricos + pozos
// de luz azul eléctrico. CSS puro, sin assets. Los colores vienen de los
// tokens Thureos (--thu-navy-*, --thu-blue-*): el panel institucional se
// mantiene en navy fijo con ambos temas, como una firma de marca, no un
// área que responda a claro/oscuro.
const guillocheStyle: React.CSSProperties = {
  backgroundColor: "var(--thu-navy-900)",
  backgroundImage: [
    "repeating-radial-gradient(circle at 82% 16%, rgba(255,255,255,0.05) 0 1px, transparent 1px 13px)",
    "repeating-radial-gradient(circle at 14% 92%, rgba(255,255,255,0.035) 0 1px, transparent 1px 16px)",
    "repeating-linear-gradient(118deg, rgba(255,255,255,0.035) 0 1px, transparent 1px 8px)",
    "radial-gradient(120% 85% at 84% 8%, color-mix(in srgb, var(--thu-blue-300) 20%, transparent), transparent 56%)",
    "radial-gradient(95% 75% at 8% 100%, color-mix(in srgb, var(--thu-blue-500) 16%, transparent), transparent 60%)",
  ].join(", "),
};

/**
 * Layout compartido de /login y /register: panel de formulario a la
 * izquierda + panel institucional de marca a la derecha (solo desktop).
 * Firma visual común con thureos-main, el otro producto de la familia.
 */
export function AuthShell({
  eyebrow,
  title,
  description,
  children,
  footer,
}: AuthShellProps) {
  return (
    <div className="grid min-h-screen md:grid-cols-2 lg:grid-cols-[1.32fr_1fr]">
      <div className="relative flex flex-col bg-background">
        <div className="h-[3px] bg-[linear-gradient(to_right,var(--thu-navy-700),var(--color-primary),var(--thu-blue-500))]" />

        <div className="flex flex-1 flex-col px-6 py-8 sm:px-10 md:px-14 lg:px-16">
          <div className="flex items-center gap-3">
            <Logo variant="lockup" size={40} />
          </div>

          <div className="flex flex-1 items-center">
            <div className="w-full max-w-[400px]">
              <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">
                {eyebrow}
              </p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-foreground">
                {title}
              </h1>
              <p className="mt-2 text-sm text-muted-foreground">
                {description}
              </p>

              <div className="mt-8">{children}</div>

              {footer}
            </div>
          </div>

          <footer className="mt-8 border-t border-border pt-5">
            <p className="text-[11px] text-muted-foreground/70">
              © 2026 Thureos Compliance
            </p>
          </footer>
        </div>
      </div>

      <aside
        style={guillocheStyle}
        className="relative hidden overflow-hidden md:flex md:flex-col md:justify-center"
      >
        <div className="absolute inset-y-0 left-0 w-px bg-[linear-gradient(to_bottom,transparent,color-mix(in_srgb,var(--thu-blue-300)_40%,transparent),transparent)]" />

        <div className="relative px-10 lg:px-14">
          <p
            className="text-[11px] font-semibold uppercase tracking-[0.28em]"
            style={{ color: "var(--thu-navy-300)" }}
          >
            Plataforma de cumplimiento
          </p>
          <h2 className="mt-5 max-w-sm text-[1.9rem] font-semibold leading-[1.08] tracking-tight text-white lg:text-[2.4rem] lg:leading-[1.05] xl:text-[2.6rem]">
            Thureos Compliance
          </h2>
          <p className="mt-5 max-w-sm text-[15px] leading-relaxed text-white/65">
            Ingesta de datos, motor de reglas dinámicas, monitoreo transaccional
            y dashboards configurables — en una sola plataforma.
          </p>
        </div>
      </aside>
    </div>
  );
}
