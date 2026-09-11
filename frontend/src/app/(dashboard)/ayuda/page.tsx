"use client";

import Link from "next/link";
import { CircleHelp } from "lucide-react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { HELP_ARTICLES } from "@/lib/help/articles";
import { HELP_GROUPS } from "@/lib/help/types";

export default function AyudaPage() {
  return (
    <>
      <Header title="Ayuda" />
      <div className="p-6 space-y-8">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold">
            <CircleHelp className="h-6 w-6 text-primary" />
            Centro de Ayuda
          </h1>
          <p className="mt-1 text-muted-foreground">
            Para qué sirve cada pantalla del sistema y cómo hacer las tareas
            más comunes.
          </p>
        </div>

        {HELP_GROUPS.map((grupo) => {
          const fichas = HELP_ARTICLES.filter((a) => a.group === grupo);
          if (fichas.length === 0) return null;
          return (
            <section key={grupo} className="space-y-3">
              <h2 className="text-lg font-semibold">{grupo}</h2>
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
                {fichas.map((ficha) => {
                  const Icono = ficha.icon;
                  return (
                    <Link key={ficha.slug} href={`/ayuda/${ficha.slug}`}>
                      <Card className="h-full transition-colors hover:border-primary/50">
                        <CardContent className="flex items-start gap-3 p-5">
                          <div className="shrink-0 rounded-lg bg-primary/10 p-2">
                            <Icono className="h-5 w-5 text-primary" />
                          </div>
                          <div>
                            <h3 className="font-semibold">{ficha.title}</h3>
                            <p className="mt-1 text-sm text-muted-foreground">
                              {ficha.summary}
                            </p>
                          </div>
                        </CardContent>
                      </Card>
                    </Link>
                  );
                })}
              </div>
            </section>
          );
        })}
      </div>
    </>
  );
}
