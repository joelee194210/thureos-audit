# Centro de Ayuda

**Fecha:** 2026-09-10
**Estado:** aprobado para planificar

## Por qué

Thureos Compliance no tiene ninguna ayuda en producto. Quien no estuvo en la conversación
donde se construyó una pantalla no tiene forma de saber qué hace, quién puede usarla, ni
cómo hacer la tarea más común — y varias de las pantallas cargan conceptos que no se
adivinan mirándolas: decimales implícitos, timestamp derivado, condiciones de velocidad,
ventanas de tiempo.

El formato se copia de **`bienes_te_p`** (Sistema de Bienes Patrimoniales), otra plataforma
del usuario. Lo que se copia no es su CSS —es Bootstrap claro y acá el tema por defecto es
oscuro— sino su **estructura**: un índice de tarjetas agrupadas como el menú, una ficha por
módulo con secciones cortas, y capturas de la pantalla real al costado.

Lo más valioso de esa referencia es la **estructura retórica** que repite en sus 27 módulos:

> ¿Para qué sirve? → ¿Quién puede usarlo? → Qué vas a encontrar → Cómo hacer *X* (en pasos)

Esa secuencia es la que hay que respetar con más cuidado que la grilla.

## Alcance

Dos rutas nuevas, un módulo de contenido y un juego de capturas. Sin backend.

## Fuera de alcance (explícito)

- **Editar la ayuda desde la interfaz.** El contenido es estático y versiona con el código.
  Si más adelante hace falta editarlo sin desplegar, se mueve a Mongo sin tocar la
  presentación — el componente ya recibe los datos por props.
- **Buscador dentro de la ayuda.** Con ~16 fichas, agrupadas y en una sola pantalla, buscar
  es scrollear. Se agrega cuando el volumen lo pida.
- **Ayuda contextual** (el `?` al lado de cada campo, los `HelpTooltip` de `thureos-main`).
  Es otro subsistema, con su propio anclaje por código de campo; este spec no lo cubre.
- **Traducción.** El producto es solo español.
- **Versionado del contenido por versión del producto.**

---

## Pieza 1 — El formato

### Índice (`/ayuda`)

- Encabezado: título "Centro de Ayuda" con ícono, y una línea explicando qué es.
- Los módulos se agrupan por área, **en el mismo orden que el menú superior**: Operación,
  Monitoreo, Catálogos, Administración, y por último Conceptos (ver Pieza 3).
- Cada grupo: su nombre y debajo una grilla de tarjetas, `grid-cols-1 md:grid-cols-2
  lg:grid-cols-3`.
- Cada tarjeta es un enlace a la ficha: ícono en un cuadro tintado, título, y el resumen de
  una o dos líneas. Sin recorte de texto: el resumen se escribe corto, no se trunca.

### Ficha (`/ayuda/[slug]`)

- Miga de pan: `Ayuda / <título>`.
- Encabezado: ícono, título, resumen, y a la derecha el botón "Volver a la ayuda".
- Cuerpo en dos columnas, `lg:grid-cols-3`:
  - **Izquierda (2/3):** las secciones, cada una en su tarjeta — un encabezado y debajo o
    bien un párrafo, o bien una lista numerada. Nunca las dos cosas.
  - **Derecha (1/3):** primero la tarjeta de "Quién puede usarlo", después las capturas
    apiladas, cada una en su tarjeta con el borde redondeado y sin desbordar.
- En pantallas angostas las columnas se apilan y las capturas quedan al final, después del
  texto: en un teléfono la explicación importa más que la imagen.

### Lo que cambia respecto de la referencia

| Bienes Patrimoniales | Acá | Por qué |
|---|---|---|
| `text-gray-900`, `bg-white` literales | Tokens: `bg-card`, `text-ink-muted`, `border-border` | El tema por defecto es oscuro y el manual de marca prohíbe colores literales |
| Íconos FontAwesome (`fa-cube`) | `lucide-react`, **los mismos que usa el menú** para cada módulo | Ya están importados; y que el ícono de la ficha sea el del menú es la mitad de la orientación |
| `permiso_modulo` como código crudo (`activos`) | El rol en lenguaje llano | Acá los roles son tres (admin, compliance, viewer), no una tabla de permisos |
| Página suelta | Integrada al sistema de pestañas | Cada pantalla acá abre como pestaña |

---

## Pieza 2 — El contenido

Un módulo TypeScript, `frontend/src/lib/help/articles.ts`, con un registro por módulo. Es el
equivalente del `data.php` de la referencia:

```ts
export type HelpSection =
  | { heading: string; body: string }
  | { heading: string; items: string[] };

export interface HelpArticle {
  slug: string;
  title: string;
  icon: LucideIcon;
  group: HelpGroup;
  /** Frase en español llano: "Cualquier usuario con sesión iniciada." */
  quienPuede: string;
  summary: string;
  images: string[];
  sections: HelpSection[];
}
```

`HelpSection` es una unión: una sección tiene párrafo **o** lista, nunca ambos. Eso hace
imposible por tipos la ficha inconsistente, que es el defecto que aparece cuando el contenido
lo escriben varias personas.

**Las secciones siguen la estructura retórica de la referencia.** Toda ficha abre con "¿Para
qué sirve?" y sigue con "¿Quién puede usarlo?"; el resto varía según el módulo, pero los
procedimientos van siempre como lista numerada y las explicaciones como párrafo.

### Los módulos

Trece pantallas del menú:

| Grupo | Fichas |
|---|---|
| Operación | Dashboard · Banderas rojas · Cargas |
| Monitoreo | Monitores · Reglas · Screening · Dashboards · Analista IA |
| Catálogos | MCC · Países |
| Administración | Usuarios · Bitácora de acceso · Configuración |

---

## Pieza 3 — Las fichas de conceptos

Tres fichas que no corresponden a una pantalla del menú sino a un flujo que atraviesa
varias, y que son donde la gente se traba de verdad:

- **El esquema de un monitor** — decimales implícitos y timestamp derivado. Por qué el
  archivo dice `500000` y el sistema guarda `5000`, qué pasa si cambiás los decimales con
  datos ya cargados, y por qué hace falta un campo de fecha real para medir tiempo entre
  transacciones.
- **Condiciones de velocidad** — en qué se diferencian de un agregado con ventana: allá la
  ventana está anclada a "ahora" y se cuenta cuántos eventos caen dentro; acá se mide la
  distancia entre un evento y el anterior de la misma entidad.
- **Generar reglas con IA** — el modo guiado, por qué conviene elegir los campos antes de
  escribir el criterio, y qué significa que una sugerencia se descarte.

Estas tres van en un grupo propio, **Conceptos**, al final del índice. Llevan más contenido
que las trece anteriores y no tienen captura obligatoria: una ilustra mejor con el ejemplo
numérico en el texto que con una foto de un formulario.

---

## Pieza 4 — Las capturas

No existen: hay que tomarlas. Se toman con Playwright contra la aplicación local, con datos
de prueba sembrados, en un solo ancho (1440), que es lo que hace la referencia: sus fichas
apilan una o dos capturas y todas son de un único ancho. Una ficha puede llevar más de una
imagen cuando el módulo tiene listado y detalle, como hace la referencia con Activos.

Viven en `frontend/public/ayuda/<slug>.png` y se nombran por el slug de la ficha.

**El riesgo real de esta pieza es que envejezcan.** En la referencia son 31 PNG sueltos sin
ningún proceso que los regenere; una captura desactualizada es documentación que miente, y
en una herramienta de cumplimiento eso se paga en confianza. Por eso se entrega junto con
ellas un script, `frontend/scripts/capturar-ayuda.ts`, que:

1. levanta la aplicación contra una base de datos de prueba,
2. siembra los datos mínimos para que cada pantalla se vea poblada y no vacía,
3. recorre la lista de rutas y guarda cada captura con su nombre,
4. falla ruidosamente si una ruta no responde o queda en blanco.

Refrescar la ayuda pasa a ser un comando en vez de una tarde. El script no corre en CI: se
corre a mano cuando una pantalla cambia.

**Los datos de las capturas son inventados**, no un recorte de producción: son pantallas de
un sistema de prevención de lavado y van a quedar versionadas en el repositorio.

---

## Integración con el sistema de pestañas

Cada pantalla abre como pestaña. Dos detalles:

- `SEGMENT_LABELS` (`frontend/src/lib/tab-labels.ts`) gana `ayuda: "Ayuda"`.
- **El nombre de la pestaña de una ficha se registra como etiqueta de entidad.** Sin eso,
  `labelForPath` toma el último segmento de `/ayuda/monitores` y lo convierte en "Monitores"
  — una pestaña de ayuda indistinguible de la pantalla real. La ficha registra
  `"Ayuda: <título>"` con `registerEntityLabel`, igual que hace la pestaña de un monitor con
  su nombre.

Las dos rutas son páginas normales de Next.js. **No entran en `TAB_CONTENT_REGISTRY`**: ese
registro existe para el contenido que debe conservar estado al cambiar de pestaña, y la
ayuda no tiene estado.

## Dónde se entra

- Un ítem **"Ayuda"** al final del menú **Operación**, con el ícono `CircleHelp` de
  `lucide-react` (verificado que existe en la versión instalada; `HelpCircle` es su alias
  viejo y no se usa).
- Sin rol restringido: la ayuda la ve cualquiera con sesión iniciada, incluido un viewer.

## Manejo de errores

- Slug inexistente (`/ayuda/loquesea`): la ficha muestra un estado vacío en español con un
  enlace de vuelta al índice. No es un 404 de Next: el usuario llegó a una ruta de la
  aplicación y merece una salida, no una pantalla de error.
- Captura faltante: la ficha se renderiza sin ella. Una imagen que no cargó nunca debe
  romper el texto, que es lo que la persona vino a leer.

## Testing

- **`articles.ts` es datos, y los datos se validan**: un test de vitest recorre todas las
  fichas y comprueba que los slugs son únicos, que cada `group` es uno de los grupos
  declarados, que ninguna sección queda vacía, y que **toda imagen referenciada existe en
  `public/ayuda/`**. Ese último es el que atrapa el error real: renombrar una captura y
  olvidar la referencia.
- Un test comprueba que **cada ruta del menú superior tiene su ficha**. Cuando alguien
  agregue una pantalla nueva sin documentarla, el test lo dice. Es la única defensa contra
  que la ayuda se quede atrás del producto.
- Verificación manual: recorrer el índice y tres fichas en modo claro y en modo oscuro, en
  escritorio y en móvil, comprobando que las capturas no desbordan y que las columnas se
  apilan en el orden correcto.

## Orden de implementación

1. **El formato con dos fichas reales** — las rutas, los componentes, el ítem del menú, la
   integración con pestañas, y el contenido de Monitores y Reglas. Al terminar esto la ayuda
   se puede ver y juzgar; si el formato no convence, se corrige antes de escribir las otras
   catorce.
2. **Las once fichas de pantalla restantes.**
3. **Las tres fichas de conceptos.**
4. **El script de capturas y las capturas.**

El paso 1 es deliberadamente el más chico que produce algo mirable. El paso 4 va al final
porque las capturas se toman de pantallas que no cambian con este trabajo, y hacerlas antes
solo adelanta su envejecimiento.
