"use client";

import Image from "next/image";
import { useState } from "react";

interface LogoProps {
  /** isotipo: escudo solo, para sidebar y avatares.
   *  lockup: escudo + wordmark, para login y cabeceras. */
  variant?: "isotipo" | "lockup";
  /** Altura en px. Mínimos del manual: isotipo 24, lockup 180 de ancho. */
  size?: number;
  className?: string;
}

export function Logo({ variant = "isotipo", size = 32, className }: LogoProps) {
  const [failed, setFailed] = useState(false);

  if (failed) {
    return (
      <span
        className={`font-semibold uppercase tracking-[0.18em] text-ink ${className ?? ""}`}
        style={{ fontSize: size * 0.4 }}
      >
        Thureos
      </span>
    );
  }

  if (variant === "lockup") {
    // Proporción original 2172×724 = 3:1.
    return (
      <Image
        src="/brand/lockup-horizontal.png"
        alt="Thureos Compliance"
        width={size * 3}
        height={size}
        className={className}
        onError={() => setFailed(true)}
        priority
      />
    );
  }

  // Proporción original 494×618.
  return (
    <Image
      src="/brand/isotipo.png"
      alt="Thureos"
      width={Math.round(size * 0.8)}
      height={size}
      className={className}
      onError={() => setFailed(true)}
      priority
    />
  );
}
