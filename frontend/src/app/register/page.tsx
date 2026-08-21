"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AuthShell } from "@/components/auth/auth-shell";
import { useAuthStore } from "@/stores/auth-store";

export default function RegisterPage() {
  const router = useRouter();
  const { register, isLoading } = useAuthStore();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    try {
      await register(name, email, password);
      router.push("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al registrar");
    }
  }

  return (
    <AuthShell
      eyebrow="Nueva cuenta"
      title="Crear cuenta"
      description="Regístrate para acceder a la plataforma de monitoreo transaccional."
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
            htmlFor="name"
            className="text-xs font-semibold uppercase tracking-wide text-muted-foreground"
          >
            Nombre
          </Label>
          <Input
            id="name"
            placeholder="Tu nombre"
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoComplete="name"
            required
            autoFocus
            className="h-11"
          />
        </div>

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
            autoComplete="new-password"
            required
            minLength={12}
            className="h-11"
          />
          <p className="text-xs text-muted-foreground">
            Mínimo 12 caracteres, con mayúscula, minúscula, número y carácter especial
          </p>
        </div>

        <Button
          type="submit"
          className="h-11 w-full text-[13.5px] font-semibold tracking-wide"
          disabled={isLoading}
        >
          {isLoading ? "Registrando..." : "Crear cuenta"}
        </Button>

        <p className="text-center text-sm text-muted-foreground">
          ¿Ya tienes cuenta?{" "}
          <Link href="/login" className="text-primary hover:underline">
            Iniciar sesión
          </Link>
        </p>
      </form>
    </AuthShell>
  );
}
