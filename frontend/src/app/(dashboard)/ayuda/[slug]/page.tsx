"use client";

import { useEffect } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import Image from "next/image";
import { ArrowLeft, CircleHelp } from "lucide-react";
import { Header } from "@/components/layout/header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { articuloPorSlug } from "@/lib/help/articles";
import { useTabStore } from "@/stores/tab-store";

export default function AyudaFichaPage() {
  // El patrón del proyecto para una ruta dinámica: useParams en un
  // componente cliente, igual que monitors/[id] y red-flags/[id].
  const { slug } = useParams<{ slug: string }>();
  const ficha = articuloPorSlug(slug);
  const registerEntityLabel = useTabStore((s) => s.registerEntityLabel);

  // Sin esto, labelForPath convierte /ayuda/monitores en una pestaña
  // llamada «Monitores», indistinguible de la pantalla real.
  useEffect(() => {
    if (ficha) registerEntityLabel(ficha.slug, `Ayuda: ${ficha.title}`);
  }, [ficha, registerEntityLabel]);

  if (!ficha) {
    return (
      <>
        <Header title="Ayuda" />
        <div className="p-6">
          <Card>
            <CardContent className="flex flex-col items-center gap-3 p-10 text-center">
              <CircleHelp className="h-8 w-8 text-muted-foreground" />
              <p className="font-medium">No existe una ficha para «{slug}»</p>
              <p className="text-sm text-muted-foreground">
                Puede que el enlace esté mal escrito o que la pantalla todavía
                no esté documentada.
              </p>
              <Button asChild variant="outline" className="mt-2">
                <Link href="/ayuda">Volver a la ayuda</Link>
              </Button>
            </CardContent>
          </Card>
        </div>
      </>
    );
  }

  const Icono = ficha.icon;

  return (
    <>
      <Header title="Ayuda" />
      <div className="p-6 space-y-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <nav className="mb-2 text-sm text-muted-foreground">
              <Link href="/ayuda" className="hover:text-foreground">
                Ayuda
              </Link>
              <span className="mx-1">/</span>
              <span className="text-foreground">{ficha.title}</span>
            </nav>
            <h1 className="flex items-center gap-2 text-2xl font-bold">
              <Icono className="h-6 w-6 text-primary" />
              {ficha.title}
            </h1>
            <p className="mt-1 max-w-2xl text-muted-foreground">
              {ficha.summary}
            </p>
          </div>
          <Button asChild variant="outline">
            <Link href="/ayuda">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Volver a la ayuda
            </Link>
          </Button>
        </div>

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div className="space-y-6 lg:col-span-2">
            {ficha.sections.map((seccion, i) => (
              <Card key={i}>
                <CardHeader>
                  <CardTitle className="text-base">{seccion.heading}</CardTitle>
                </CardHeader>
                <CardContent>
                  {"body" in seccion ? (
                    <p className="leading-relaxed text-muted-foreground">
                      {seccion.body}
                    </p>
                  ) : (
                    <ol className="list-inside list-decimal space-y-2 text-muted-foreground">
                      {seccion.items.map((item, j) => (
                        <li key={j}>{item}</li>
                      ))}
                    </ol>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>

          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm uppercase tracking-wide text-muted-foreground">
                  Quién puede usarlo
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-sm leading-relaxed">{ficha.quienPuede}</p>
              </CardContent>
            </Card>

            {ficha.images.map((img) => (
              <Card key={img} className="overflow-hidden">
                <Image
                  src={`/ayuda/${img}`}
                  alt={`Captura de ${ficha.title}`}
                  width={1440}
                  height={900}
                  className="h-auto w-full"
                />
              </Card>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
