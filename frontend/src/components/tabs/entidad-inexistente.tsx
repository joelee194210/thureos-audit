"use client";

import { usePathname, useRouter } from "next/navigation";
import { FileQuestion } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Header } from "@/components/layout/header";
import { useTabStore } from "@/stores/tab-store";

/**
 * Lo que ve una pestaña cuyo contenido ya no existe.
 *
 * Las pestañas se guardan en localStorage, así que sobreviven al cierre de
 * sesión. Una que apunta a algo borrado se restaura en cada ingreso y vuelve
 * a pedirlo: antes eso eran dos toasts de error por carga, para siempre, sin
 * decir qué pasaba ni cómo salir.
 *
 * No se cierra sola a propósito. Un 404 puede ser transitorio —el backend
 * reiniciando a mitad de un despliegue— y cerrar la pestaña por las dudas le
 * come al usuario algo que quería tener abierto. Se le explica y decide él.
 */
export function EntidadInexistente({
  titulo,
  que,
  tabId,
}: {
  /** El título de la barra superior: "Monitor", "Caso", "Dashboard". */
  titulo: string;
  /** Cómo se nombra en la frase: "El monitor", "El caso", "El dashboard". */
  que: string;
  tabId: string;
}) {
  const closeTab = useTabStore((s) => s.closeTab);
  const router = useRouter();
  const pathname = usePathname();

  /**
   * Cerrar la pestaña y además navegar: sin lo segundo, la pestaña
   * desaparece de la barra pero la URL sigue siendo la de la entidad
   * borrada, así que uno se queda mirando esta misma tarjeta. Es el mismo
   * gesto que hace la ✕ de la barra de pestañas.
   */
  function cerrar() {
    closeTab(tabId);
    const siguiente = useTabStore.getState().activeTabId;
    router.push(siguiente && siguiente !== pathname ? siguiente : "/");
  }

  return (
    <>
      <Header title={titulo} />
      <div className="p-6">
        <Card>
          <CardContent className="flex flex-col items-center gap-3 p-10 text-center">
            <FileQuestion className="h-8 w-8 text-muted-foreground" />
            <p className="font-medium">{que} ya no existe</p>
            <p className="max-w-md text-sm text-muted-foreground">
              Puede que se haya eliminado desde otra sesión. Esta pestaña quedó
              guardada de antes y por eso se volvió a abrir.
            </p>
            <Button variant="outline" className="mt-2" onClick={cerrar}>
              Cerrar esta pestaña
            </Button>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
