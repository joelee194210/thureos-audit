"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { AlertCircle, ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AuthShell } from "@/components/auth/auth-shell";
import { useAuthStore } from "@/stores/auth-store";

export default function LoginPage() {
  const router = useRouter();
  const { login, isLoading } = useAuthStore();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    try {
      await login(email, password);
      router.push("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al iniciar sesión");
    }
  }

  return (
    <AuthShell
      eyebrow="Acceso al sistema"
      title="Iniciar sesión"
      description="Ingresa tus credenciales para acceder a la plataforma de monitoreo transaccional."
      footer={
        <div className="mt-6 flex items-center gap-2 text-xs text-muted-foreground">
          <ShieldCheck className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span>Conexión cifrada</span>
        </div>
      }
    >
      <form onSubmit={handleSubmit} className="space-y-5">
        {error && (
          <div
            role="alert"
            className="flex items-center gap-2 rounded-md border border-destructive/25 bg-destructive/8 px-3 py-2.5 text-sm text-destructive"
          >
            <AlertCircle className="h-4 w-4 shrink-0" aria-hidden="true" />
            {error}
          </div>
        )}

        <div className="space-y-2">
          <Label
            htmlFor="email"
            className="text-xs font-semibold uppercase tracking-wide text-muted-foreground"
          >
            Correo electrónico
          </Label>
          <Input
            id="email"
            type="email"
            placeholder="tu@email.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            required
            autoFocus
            className="h-11"
          />
        </div>

        <div className="space-y-2">
          <Label
            htmlFor="password"
            className="text-xs font-semibold uppercase tracking-wide text-muted-foreground"
          >
            Contraseña
          </Label>
          <Input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
            className="h-11"
          />
        </div>

        <Button
          type="submit"
          className="h-11 w-full text-[13.5px] font-semibold tracking-wide"
          disabled={isLoading}
        >
          {isLoading ? "Ingresando..." : "Iniciar sesión"}
        </Button>

        <p className="text-center text-sm text-muted-foreground">
          ¿No tienes cuenta?{" "}
          <Link href="/register" className="text-primary hover:underline">
            Regístrate
          </Link>
        </p>
      </form>
    </AuthShell>
  );
}
