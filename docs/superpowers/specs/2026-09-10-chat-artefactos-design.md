# Artefactos de primera clase — Diseño

**Fecha:** 2026-09-10
**Estado:** Aprobado para pasar a plan de implementación
**Parte 2 de 2.** Depende de `2026-09-10-chat-multimonitor-design.md`, que debe
estar implementado antes: los artefactos registran de qué monitores salen sus
datos y reutilizan la resolución de alias contra allowlist definida ahí.

## Objetivo

Tres pedidos que colapsan en un mismo cambio de modelo:

1. **Guardar** un artefacto generado por el asistente.
2. **Invocarlo** después directamente, con **datos frescos**.
3. **Exportarlo** a Excel, PDF, HTML, CSV o PNG.

Los tres necesitan lo mismo: que el artefacto tenga **identidad**. Hoy
`ChatArtifact` es un sub-documento embebido en `ChatMessage`, sin id. Sin id no
hay a qué apuntar para guardar, ni para invocar, ni para `GET /export`.

Se suma un cuarto pedido, de presentación: que los datos que hoy salen como
texto plano se vean **formateados**.

## Decisión de fondo: el artefacto declara su consulta, no sus números

Hoy el LLM emite el artefacto con los datos ya adentro (`chartSpec.data`),
después de haberlos obtenido con `query_monitor_data`. Esos números son un
fósil: la consulta que los produjo no se guarda en ningún lado, así que
"re-ejecutar el artefacto" no es algo que se pueda hacer.

Se invierte el contrato: **el artefacto declara de dónde salen sus datos, no
cuáles son.**

```go
// ArtifactSource: una consulta que alimenta al artefacto. El LLM ya sabe
// escribir este objeto — Query es literalmente el input de la herramienta
// query_monitor_data que viene usando.
type ArtifactSource struct {
    Monitor string               `bson:"monitor" json:"monitor"` // alias
    Query   models.ChatQueryInput `bson:"query"  json:"query"`
    Label   string               `bson:"label,omitempty" json:"label,omitempty"`
}
```

Renderizar un artefacto = ejecutar sus `sources` y aplicarles la presentación
(`chartType`, `xKey`, `yKeys`, formato de columnas). **Un solo camino de
código**: la primera vez que se ve el artefacto en el chat y la vez que se
invoca seis meses después recorren exactamente el mismo path. No hay una rama
"datos guardados" y otra "datos frescos" que puedan divergir.

La instantánea deja de ser una forma distinta de artefacto y pasa a ser un
**caché del último resultado**, con su fecha de corrida. Se usa para pintar algo
de inmediato al abrir mientras se re-ejecuta, y como fallback legible si la
re-ejecución falla.

### Proyección y rótulos: el LLM sigue dando forma a la salida

Con los datos inline, el LLM elegía los nombres de las columnas porque las
escribía él: `{"region": "Antioquia", "total": 12345}`. Con `sources` los datos
llegan crudos del pipeline, y sin nada más el resultado sería feo y filtroso:

- Una agregación emite `_id`, `aggValue` y `count`
  (`buildDataAggregatePipeline`). Un gráfico así tendría los ejes rotulados
  **`_id`** y **`aggValue`**.
- Un `conditionGroup` va por `QueryData`, que devuelve el `bson.M` **completo**
  de cada documento: todos los campos, incluido el `_id` ObjectID crudo.

Así que la presentación gana dos campos, ambos escritos por el LLM y ambos
aplicados **en cada render y cada re-ejecución**:

```go
type ChartSpec struct {
    ...
    // Columns: proyección — qué columnas mostrar y en qué orden. Vacío = todas.
    Columns []string `bson:"columns,omitempty" json:"columns,omitempty"`
    // Labels: nombre de presentación por columna. Se aplica al renderizar,
    // nunca toca los datos ni las claves de xKey/yKeys.
    Labels map[string]string `bson:"labels,omitempty" json:"labels,omitempty"`
}
```

`xKey` y `yKeys` siguen refiriéndose a las claves **reales** (`_id`,
`aggValue`); `Labels` solo cambia lo que se pinta en el eje, la leyenda y la
cabecera de la tabla. Mantenerlos separados evita que renombrar algo rompa la
referencia.

El prompt lo pide explícitamente, con el ejemplo de abajo. Un artefacto sin
`Labels` se renderiza con las claves crudas: es feo pero no falla.

### Varias fuentes

Acá `chart` y `table` **no** se comportan igual, y la diferencia importa.

**`table` concatena.** Los resultados se apilan agregando una columna
discriminadora `_monitor` con el `Label` de la fuente (o su alias si no hay
label). Es lo que se espera de una tabla comparativa.

**`chart` pivotea.** El motivo del multi-fuente es «una serie por monitor», y
concatenar **no** produce eso: produce una sola serie con valores de `x`
repetidos, que Recharts apila mal. Hay que pivotear: una fila por valor de
`xKey`, una columna por fuente.

```
fuente A: [{_id: "Antioquia", aggValue: 100}, {_id: "Valle", aggValue: 80}]
fuente B: [{_id: "Antioquia", aggValue:  60}]

pivot →   [{_id: "Antioquia", "Bancolombia": 100, "Davivienda": 60},
           {_id: "Valle",     "Bancolombia":  80, "Davivienda": null}]
```

`yKeys` pasa a ser la lista de labels de las fuentes, y las claves faltantes
quedan en `null` (Recharts corta la línea, que es lo correcto: no había dato).
El pivote se hace sobre `xKey`; si un `chart` multi-fuente no declara `xKey`,
el artefacto es inválido y se descarta.

### La línea de alcance, explícita

| Tipo | Fuente de datos | ¿Re-ejecutable? | Export |
|---|---|---|---|
| `chart` | `sources` | Sí | PNG, CSV, XLSX, PDF, HTML |
| `table` | `sources` | Sí | CSV, XLSX, PDF, HTML |
| `custom` | `code` inline | **No** | HTML (el propio código), impresión del navegador |

Los artefactos `custom` son HTML/JS que el LLM escribe para lo que un gráfico de
barra/línea/torta no expresa, con sus datos embebidos. No tienen consulta que
re-ejecutar. El botón "actualizar" aparece **deshabilitado con el motivo
visible**, no oculto.

**`custom` no exporta a PDF ni XLSX.** No se puede rasterizar un iframe
sandboxeado cross-origin, y `jspdf` no renderiza HTML arbitrario. Se dice de
frente en la UI en vez de fallar en silencio.

### Artefactos que ya existen

Los artefactos ya persistidos en `chat_messages` no tienen `sources`. Se tratan
como instantánea: se muestran con sus datos guardados y no son re-ejecutables,
igual que un `custom`. **Sin migración.** La regla es una sola: `sources` vacío
⇒ instantánea.

## Modelo de datos

### Prerrequisito: mover `ChatQueryInput` a `models`

`ChatQueryInput` y `ChatAggregateSpec` viven hoy en el paquete `services`
(`chat_llm_parsing.go`, `chat_query.go`). `ArtifactSource` es parte de un modelo
persistido, y `models` **no puede importar `services`** —`services` ya importa
`models`, sería un ciclo de imports y no compila.

Primer paso de la implementación, antes de tocar nada más: **mover
`ChatQueryInput` y `ChatAggregateSpec` a `models`**, dejando en `services` la
lógica que opera sobre ellas (`ParseQueryToolInput`, `buildDataAggregatePipeline`).
Es un movimiento mecánico de dos structs, pero descubrirlo a mitad de camino
obliga a rehacer el modelo.

### `chat_artifacts`

Colección nueva:

```go
type ChatArtifact struct {
    ID             primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
    UserID         primitive.ObjectID   `bson:"user_id"         json:"userId"`
    ConversationID primitive.ObjectID   `bson:"conversation_id" json:"conversationId"`
    MessageID      primitive.ObjectID   `bson:"message_id"      json:"messageId"`
    // MonitorIDs: los monitores que las sources consultan, resueltos al
    // crear. Es la allowlist del artefacto cuando se re-ejecuta fuera de su
    // conversación (ver Autorización).
    MonitorIDs []primitive.ObjectID `bson:"monitor_ids" json:"monitorIds"`

    Type      ChatArtifactType `bson:"type"  json:"type"`
    Title     string           `bson:"title" json:"title"`
    ChartSpec *ChartSpec       `bson:"chart_spec,omitempty" json:"chartSpec,omitempty"`
    Code      string           `bson:"code,omitempty"       json:"code,omitempty"`
    Sources   []ArtifactSource `bson:"sources,omitempty"    json:"sources,omitempty"`

    // Saved / SavedName: un artefacto nace efímero (vive en su mensaje) y se
    // vuelve permanente cuando el usuario lo guarda con un nombre.
    Saved     bool   `bson:"saved"                json:"saved"`
    SavedName string `bson:"saved_name,omitempty" json:"savedName,omitempty"`

    // CachedData / RanAt: resultado de la última ejecución.
    CachedData []map[string]interface{} `bson:"cached_data,omitempty" json:"cachedData,omitempty"`
    RanAt      time.Time                `bson:"ran_at,omitempty"      json:"ranAt,omitempty"`

    CreatedAt time.Time `bson:"created_at" json:"createdAt"`
}
```

**Colisión de nombres, a resolver antes de escribir código.** `ChatArtifact` ya
existe en `models` como el sub-documento embebido, y este spec dice que los
mensajes viejos lo siguen deserializando: no puede haber dos structs con ese
nombre en el mismo paquete. El nombre `ChatArtifact` queda para la entidad
nueva —es la que sobrevive— y **la embebida pasa a `LegacyChatArtifact`**, usada
solo para decodificar mensajes viejos. Mismo criterio que `LegacyMonitorID` en
la parte 1.

`ChatMessage.Artifact` (embebido) se reemplaza por
`ArtifactID *primitive.ObjectID`. Los mensajes viejos conservan su
`LegacyChatArtifact`; al leer un mensaje con `artifact` y sin `artifact_id` se
convierte a un `ChatArtifact` sin id, instantánea (no guardable, no
re-ejecutable). Se resuelve en el repositorio para que ningún llamador tenga que
saberlo.

Índices: `{user_id: 1, saved: 1, created_at: -1}` para la biblioteca,
`{conversation_id: 1}` para el borrado en cascada.

`DeleteConversation` borra también los artefactos **no guardados** de la
conversación. Los guardados sobreviven —la biblioteca es del usuario, no de la
conversación— y quedan con `ConversationID` apuntando a algo que ya no existe,
que es correcto: su valor está en las `sources`, no en el hilo.

## Ejecución y autorización

```go
// ArtifactRun es el resultado de ejecutar un artefacto: los datos ya
// proyectados y (para un chart multi-fuente) pivoteados, más el momento
// de la corrida.
type ArtifactRun struct {
    Data  []map[string]interface{}
    RanAt time.Time
}

// RunArtifact ejecuta las sources del artefacto y devuelve la tabla
// resultante. Es el ÚNICO camino por el que se obtienen datos de un
// artefacto: lo usan el render inicial, la invocación y las tres
// exportaciones. NO persiste nada — ver "Cuándo se escribe la caché".
func (s *ChatService) RunArtifact(
    ctx context.Context,
    art *models.ChatArtifact,
    userID primitive.ObjectID,
) (ArtifactRun, error)
```

### Cuándo se escribe la caché

`RunArtifact` es puro: ejecuta y devuelve, no escribe. **Solo `POST /run`
persiste `CachedData` y `RanAt`**, después de una ejecución exitosa.

Las exportaciones re-ejecutan pero **no** mutan el artefacto. Bajar un Excel no
debería cambiar el estado de nada, y si escribieran caché, exportar desde la
biblioteca alteraría la fecha que ve otro usuario mirando el mismo artefacto en
pantalla.

**Autorización.** Se conserva la regla de la parte 1 —el alias se resuelve
contra una allowlist, nunca se acepta un ObjectID crudo— pero la allowlist es
`art.MonitorIDs`, no la de una conversación: un artefacto guardado se invoca
desde la biblioteca, fuera de todo hilo.

Antes de ejecutar:

1. `art.UserID == userID`, o `ErrArtifactNotFound` (mismo criterio que
   `ErrConversationNotFound`: no se distingue "no existe" de "no es tuyo").
2. Cada `source.Monitor` se resuelve contra los alias derivados de
   `art.MonitorIDs`. Alias desconocido ⇒ error, **sin ejecutar consulta**.

`art.MonitorIDs` se fija al **crear** el artefacto, resolviendo los alias contra
`conv.MonitorIDs` de ese momento. Un artefacto nunca puede ganar acceso a un
monitor que su conversación de origen no tenía.

**Monitor renombrado.** El alias se deriva del nombre, así que renombrar un
monitor rompe la resolución de un artefacto guardado. Al fallar la resolución
por nombre se intenta un segundo pase posicional contra `art.MonitorIDs` en
orden; si tampoco resuelve, el artefacto muestra su caché con un aviso de que su
fuente ya no está disponible. Nunca se ejecuta contra un monitor distinto del
que se resolvió.

## API

| Método | Ruta | Qué hace |
|---|---|---|
| `POST` | `/chat/artifacts/:id/save` | Marca guardado, body `{name}` |
| `DELETE` | `/chat/artifacts/:id/save` | Lo devuelve a efímero |
| `GET` | `/chat/artifacts` | Biblioteca del usuario (`?saved=true`) |
| `POST` | `/chat/artifacts/:id/run` | Re-ejecuta y devuelve datos frescos + `ranAt` |
| `GET` | `/chat/artifacts/:id/export.xlsx` | Excel, datos frescos |
| `DELETE` | `/chat/artifacts/:id` | Borra el artefacto guardado |

Todas validan propiedad. `/run` y `/export.xlsx` pasan por `RunArtifact`.

## Exportación

| Formato | Dónde | Con qué |
|---|---|---|
| **XLSX** | Backend | `excelize` (ya en `go.mod`) |
| **PDF** | Frontend | `jspdf` + `jspdf-autotable` + `pdf-tokens.ts` |
| **HTML** | Frontend | Documento autónomo con marca |
| **CSV / PNG** | Frontend | `chat-export.ts`, que se extiende |

**XLSX va al backend** porque el frontend no tiene librería de Excel y
`excelize` ya está: cero dependencias nuevas y sin viaje de ida y vuelta de los
datos. Cabecera en negrita, ancho de columna por contenido, números como números
(no texto) y fechas con formato de fecha.

**PDF y HTML van al frontend** porque ahí están los tokens de marca, que
CLAUDE.md exige y `pdf-tokens.ts` ya sabe leer. El PDF sigue el patrón de
`red-flag-report.ts`: `readReportColors`, márgenes, cabecera, `autoTable`. Para
un `chart`, se embebe el PNG del SVG con el `exportChartAsPNG` que ya existe.

**Todas las exportaciones re-ejecutan las fuentes**: exportar trae los datos de
hoy, no los del día en que se creó el artefacto. El documento lleva impresa la
fecha de corrida para que no haya ambigüedad sobre qué se está mirando.

### Documento HTML

Un `.html` autónomo —estilos embebidos, sin recursos externos— con cabecera de
marca Thureos, título, fecha de corrida, chips de los monitores consultados, la
tabla formateada y, si es un `chart`, el PNG embebido como data URI. Se abre en
pestaña nueva o se descarga.

El mismo módulo que arma este documento alimenta al PDF: una sola definición de
"cómo se ve un informe de artefacto", dos destinos.

## Formato rico

### Burbujas del chat: markdown sin `innerHTML`

Hoy las burbujas usan `whitespace-pre-wrap` (`page.tsx`): si el LLM escribe
`**negrita**`, una tabla markdown o una lista, se ven los asteriscos y los pipes
crudos. Se renderizan de verdad.

**Cómo, y por qué no por el camino obvio.** Lo natural sería
`marked` + `DOMPurify` + `dangerouslySetInnerHTML`. **No se hace acá.**

El texto del asistente está influido por CSVs que suben los usuarios: una celda
con una inyección de prompt puede moldear lo que el LLM escribe. Inyectar eso
como HTML en el origen de la app —donde vive el JWT— es exactamente lo que el
diseño original evitó al meter el código generado en un iframe sandboxeado sin
`allow-same-origin`. DOMPurify es sólido, pero el costo de un bypass ahí es la
sesión del usuario, y esto es una plataforma de cumplimiento.

En su lugar: **`marked.lexer()` para obtener el árbol de tokens, y un mapeo de
tokens a elementos de React.** No se produce HTML en ningún momento del camino.

- Tokens soportados: `paragraph`, `strong`, `em`, `codespan`, `code`, `list`,
  `listitem`, `table`, `heading`, `link`, `blockquote`, `hr`, `br`, `text`.
- Cualquier token no soportado se renderiza como su texto plano. Nunca se
  interpreta HTML crudo: los tokens `html` de marked se tratan como texto.
- `link`: solo `http:`, `https:` y `mailto:`. Todo lo demás (incluido
  `javascript:`) se renderiza como texto sin enlace.

Ventaja sobre la ruta con sanitizador: **no hay superficie de `innerHTML`**, así
que no existe la clase de bypass que habría que vigilar. Mantiene además la
invariante del proyecto de que ningún componente escribe HTML crudo. Los
artefactos `custom` siguen en su iframe sandboxeado, sin cambios.

Dependencia nueva: `marked` (solo el lexer). No se agrega `dompurify`.

### Tabla con formato por tipo

`TableArtifact` hoy hace `String(row[col] ?? "")`: montos sin separador de
miles, fechas ISO crudas, nulos como cadena vacía. Se reutiliza el criterio de
`formatValue` de `red-flag-report.ts`, elevándolo a un módulo compartido:

- Números: `Intl.NumberFormat("es-CO")`, alineados a la derecha, en JetBrains
  Mono (la tipografía que el manual de marca asigna a montos e identificadores).
- Fechas ISO: `formatDate`.
- Booleanos: "Sí" / "No". Nulos y vacíos: "—".
- Objetos: JSON compacto.

`red-flag-report.ts` pasa a importar de ese módulo en vez de tener su copia.

## Biblioteca e invocación

Panel de artefactos guardados junto al chat. Se guarda con un botón desde el
canvas, poniéndole nombre. La lista es del usuario, con chips de los monitores
que consulta cada uno y la fecha de la última corrida.

Abrir un artefacto lo **ejecuta contra los datos actuales**: se pinta la caché
de inmediato, se dispara `/run`, y se reemplaza al llegar mostrando `ranAt`. Si
`/run` falla, queda la caché con un aviso explícito de que son datos de tal
fecha. Desde el panel se exporta a cualquier formato sin pasar por el chat.

**Deliberadamente no se le da al LLM una herramienta para listar y ejecutar
artefactos guardados por nombre.** Es tentador, pero agrega superficie de
decisión del modelo sobre datos persistidos sin que se haya pedido. Queda
anotado como extensión aditiva.

## Prompt

El bloque `<artifact>` del system prompt cambia para pedir `sources` en vez de
datos inline:

```
<artifact>
{"type": "chart", "title": "...",
 "sources": [{"monitor": "transacciones", "label": "Bancolombia",
              "query": {"aggregate": {"field": "monto", "function": "sum", "groupBy": "region"}}}],
 "chartSpec": {"chartType": "bar", "xKey": "_id", "yKeys": ["aggValue"],
               "labels": {"_id": "Región", "aggValue": "Monto total"}}}
</artifact>
```

Y para una tabla sobre filas crudas, donde la proyección es lo que evita volcar
el documento entero:

```json
{"type": "table", "title": "Operaciones sobre 50M",
 "sources": [{"monitor": "transacciones",
              "query": {"conditionGroup": {"logic": "AND", "conditions": [
                  {"field": "monto", "operator": "gt", "value": 50000000}]}, "limit": 200}}],
 "chartSpec": {"columns": ["fecha", "monto", "beneficiario"],
               "labels": {"fecha": "Fecha", "monto": "Monto", "beneficiario": "Beneficiario"}}}
```

Con la instrucción de que las `sources` deben ser las mismas consultas que ya
ejecutó con la herramienta, para que el artefacto muestre lo que acaba de
describir. `custom` se mantiene con `code` inline y sin `sources`.

**Nombres de columna de una agregación.** `buildDataAggregatePipeline`
(`chat_query.go`) emite `_id` (el valor de `groupBy`), `aggValue` (el resultado
de la función) y `count`. El prompt debe decirlo explícitamente, porque el LLM
no tiene forma de adivinarlo y hoy no lo necesitaba: escribía los datos inline
con los nombres que quisiera. Ahora los datos llegan del pipeline y `xKey` /
`yKeys` tienen que coincidir con esos tres nombres.

### Cambio en `ParseArtifact`

`ParseArtifact` (`chat_llm_parsing.go`) hoy **exige** `chartSpec` con
`len(Data) > 0` para `chart` y `table`. Bajo el contrato nuevo los datos vienen
de `sources` y `Data` llega vacío, así que esa validación rechazaría todos los
artefactos nuevos.

Pasa a: un artefacto `chart`/`table` es válido si tiene **`sources` no vacío
(camino nuevo) o `chartSpec.Data` no vacío (instantánea/legacy)**. Si tiene
`sources`, se valida que cada una traiga `monitor` y una `query` parseable con
`ParseQueryToolInput`.

Se conserva el comportamiento actual en lo demás: **un artefacto que no parsea
se descarta sin abortar el texto de la respuesta**.

## Manejo de errores

| Situación | Comportamiento |
|---|---|
| `/run` falla | Se muestra la caché con su fecha y un aviso. Nunca una pantalla vacía. |
| Artefacto sin `sources` (legacy o `custom`) | Instantánea; "actualizar" deshabilitado con el motivo visible. |
| Alias irresoluble (monitor renombrado o borrado) | Segundo pase posicional; si falla, caché + aviso de fuente no disponible. |
| Export de `custom` a PDF/XLSX | La opción no se ofrece; se ofrece HTML e impresión. |
| Artefacto de otro usuario | `ErrArtifactNotFound`, indistinguible de "no existe". |
| `sources` devuelve cero filas | Estado vacío explícito, no un gráfico en blanco. |

## Testing

**Go:**
- `RunArtifact` con una fuente.
- **`table` multi-fuente**: concatenación y columna `_monitor` con el label.
- **`chart` multi-fuente**: pivote sobre `xKey`, una columna por fuente, `null`
  donde una fuente no tiene esa `x`. `chart` multi-fuente sin `xKey` → inválido.
- **Proyección y rótulos**: `columns` recorta y ordena; `columns` vacío devuelve
  todo; `labels` no altera los datos ni rompe la referencia de `xKey`/`yKeys`.
- `RunArtifact` **no** persiste caché; `POST /run` sí; exportar no muta.
- **Autorización**: alias fuera de `art.MonitorIDs` → error y ninguna consulta
  ejecutada; artefacto de otro usuario → `ErrArtifactNotFound`.
- Artefacto sin `sources` → instantánea, nunca ejecuta.
- Resolución posicional cuando el nombre del monitor cambió.
- Generación del XLSX: cabecera, tipos numéricos y de fecha correctos.
- `ParseArtifact`: `chart`/`table` con `sources` y **sin** `chartSpec.Data` →
  válido (es la regresión que rompería todo artefacto nuevo); con `Data` y sin
  `sources` → válido como instantánea; sin ninguno de los dos → inválido;
  `sources` con `query` no parseable → artefacto descartado, texto intacto.

**TypeScript** (Vitest, en `src/lib/`, donde ya viven los tests):
- Mapeo de tokens markdown a elementos, con **casos hostiles**: `<script>`,
  `<img onerror=...>`, `javascript:` en un enlace, HTML crudo entre texto. En
  todos, el resultado debe ser texto visible, nunca un elemento ejecutable.
- `formatValue` por tipo: números, fechas ISO, booleanos, nulos, objetos.
- Armado del documento HTML: sin recursos externos, con fecha de corrida.

## Extensiones aditivas (anotadas, fuera de alcance)

- **Herramienta del LLM para invocar artefactos guardados** por nombre.
- **Programar un artefacto** (correrlo cada mes y mandarlo por correo). El
  modelo ya lo soporta: un artefacto guardado es una consulta con nombre.
- **Compartir artefactos entre usuarios**, hoy privados por `UserID`.
