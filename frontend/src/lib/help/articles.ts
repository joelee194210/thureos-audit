import { Monitor, ShieldCheck } from "lucide-react";
import type { HelpArticle } from "./types";

export const HELP_ARTICLES: HelpArticle[] = [
  {
    slug: "monitores",
    title: "Monitores",
    icon: Monitor,
    group: "Monitoreo",
    route: "/monitors",
    quienPuede:
      "Ver los monitores, cualquier usuario con sesión iniciada. Crearlos, editarlos y cargarles datos, los roles compliance y admin.",
    summary:
      "Cada monitor es una fuente de datos: define de dónde vienen las transacciones, qué columnas traen y cómo se interpretan.",
    images: ["monitores.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Un monitor es el contenedor de una fuente de datos. Al cargar el primer archivo, el sistema detecta las columnas y arma el esquema: qué campos hay y de qué tipo es cada uno. A partir de ahí todo lo demás —las reglas, los tableros, el analista de IA— trabaja sobre ese esquema.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada puede ver la lista y el detalle de un monitor. Crear uno nuevo, editar su esquema o cargarle datos requiere el rol compliance o admin. Un viewer ve los monitores pero no los modifica.",
      },
      {
        heading: "Cómo crear un monitor",
        items: [
          "Entrá a Monitores y presioná «Nuevo monitor».",
          "Ponele nombre y descripción, y elegí el tipo de fuente: CSV, TXT, Excel, JSON o API.",
          "Si es un archivo delimitado, indicá el separador (coma, punto y coma, pipe o tabulador) y si la primera fila es el encabezado.",
          "Guardá, entrá al monitor y cargá el primer archivo. El esquema se detecta solo.",
        ],
      },
      {
        heading: "Las cuatro pestañas de un monitor",
        items: [
          "Cargar datos: subís archivos. Antes de ingerir, el sistema compara la estructura con el esquema guardado y avisa si no coinciden.",
          "Datos: los registros ya ingeridos, con búsqueda.",
          "Esquema: el tipo de cada campo, los decimales implícitos y el timestamp derivado. Es la pestaña que más conviene entender — mirá la ficha «El esquema de un monitor».",
          "Reglas: las reglas que evalúan este monitor, con acceso directo a ejecutarlas.",
        ],
      },
      {
        heading: "Por qué una carga puede ser rechazada",
        body:
          "Si el archivo trae columnas distintas de las del esquema guardado, la carga se rechaza entera y se informa qué campos faltan o sobran. Es deliberado: aceptar un archivo con la estructura cambiada es la forma más silenciosa de arruinar un histórico. Si el cambio es legítimo, primero ajustá el esquema.",
      },
    ],
  },
  {
    slug: "reglas",
    title: "Reglas",
    icon: ShieldCheck,
    group: "Monitoreo",
    route: "/rules",
    quienPuede:
      "Ver las reglas, cualquier usuario con sesión iniciada. Crearlas, editarlas y ejecutarlas, los roles compliance y admin.",
    summary:
      "Las condiciones que se evalúan sobre los datos de un monitor. Cuando una se cumple, se genera una bandera roja.",
    images: ["reglas.png"],
    sections: [
      {
        heading: "¿Para qué sirve?",
        body:
          "Una regla describe un patrón que hay que detectar. Se evalúa automáticamente cada vez que entran datos nuevos, y también a pedido con «Ejecutar ahora» o de forma programada. Cada coincidencia genera una bandera roja que alguien tiene que investigar.",
      },
      {
        heading: "¿Quién puede usarlo?",
        body:
          "Cualquier usuario con sesión iniciada puede ver las reglas y sus resultados. Crear, editar, activar, desactivar o ejecutar una regla requiere el rol compliance o admin.",
      },
      {
        heading: "Los tres tipos de condición",
        items: [
          "Condiciones simples: se evalúan fila por fila. «El importe es mayor a 5000», «el MCC es 7995».",
          "Condiciones agregadas: agrupan y suman, cuentan o promedian sobre una ventana de tiempo anclada a ahora. «Más de 3 transacciones por tarjeta en 24 horas».",
          "Condiciones de velocidad: miden el tiempo entre una transacción y la anterior de la misma entidad. «Dos cargos de la misma tarjeta separados por menos de 35 segundos». Mirá la ficha «Condiciones de velocidad».",
        ],
      },
      {
        heading: "Cómo crear una regla",
        items: [
          "Entrá a Reglas y presioná «Nueva regla».",
          "Elegí el monitor sobre el que se evalúa y ponele un nombre que describa el patrón, no la implementación.",
          "Agregá las condiciones que necesites: simples, agregadas, de velocidad, o una combinación.",
          "Elegí la severidad y las acciones. Antes de guardar, «Probar contra histórico» te dice cuántas coincidencias habría producido sobre los datos ya cargados.",
        ],
      },
      {
        heading: "Por qué probar contra el histórico",
        body:
          "Una regla demasiado amplia genera cientos de banderas rojas que nadie alcanza a revisar, y el efecto práctico es que se dejan de mirar todas, incluidas las buenas. El botón «Probar contra histórico» corre la regla sobre los datos ya ingeridos sin guardar nada, para que puedas juzgar señal contra ruido antes de activarla.",
      },
      {
        heading: "Si una regla no dispara",
        body:
          "Revisá que el campo de tiempo de un agregado o de una condición de velocidad sea de tipo fecha: si es un número, la comparación no encuentra nada y la regla no avisa de eso. Al guardar, el sistema rechaza las condiciones que no podrían dispararse nunca y explica el motivo.",
      },
    ],
  },
];

/** La ficha de un slug, o undefined si no existe. */
export function articuloPorSlug(slug: string): HelpArticle | undefined {
  return HELP_ARTICLES.find((a) => a.slug === slug);
}
