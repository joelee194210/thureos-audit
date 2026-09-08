# Bitácora de Cargas — Diseño

**Fecha:** 2026-09-08
**Estado:** Aprobado para pasar a plan de implementación (diseño validado en chat, sección por sección)

## Objetivo

Hoy, subir un archivo a un monitor (`POST /monitors/:id/upload`, `monitor_handler.go:304`) lo ingiere sin ninguna validación de estructura contra los datos ya existentes de ese monitor. Esto permitió que un archivo con columnas distintas al resto del histórico se ingiriera igual, mezclando documentos de dos formas distintas en la misma colección de Mongo (confirmado en el monitor "CBCG Tarjetas": campos tipo `paiscomercio`/`clavepais`/`importe` conviviendo con `fechatx`/`horatx`/`monto`/`ticket`) y produciendo campos en blanco cuando la UI espera un campo que esa fila nunca tuvo.

Este feature agrega:
1. Una validación de estructura ANTES de ingerir (rechaza el archivo completo si no coincide, con la opción de aprobar el cambio como nueva base).
2. Una validación por fila para archivos que sí coinciden en estructura (una fila con un valor del tipo incorrecto se salta, no rompe el resto).
3. Una verificación previa automática al hacer click en "Subir" — antes de comprometer nada, si hay problemas se muestra una alerta con el detalle y el usuario elige cancelar o subir de todas formas.
4. Una bitácora persistida (nueva, no existía) de cada intento real de carga, con filtros, visible en la página `/uploads` ya existente.

## Fuera de alcance (explícito)

- **Limpieza de monitores ya mezclados** (ej. "CBCG Tarjetas"). La validación aplica desde el próximo upload en cualquier monitor — los datos ya ingeridos con estructuras mezcladas quedan como están. Es una tarea manual aparte.
- **Alertas push/email** cuando se rechaza un archivo. La alerta es sincrónica, en el momento de la subida (ver Flujo 3) — no hay notificación asíncrona a terceros.
- **Configurar la estrictitud de la comparación por monitor.** Es fija: mismos nombres de campo (sin importar el orden) + mismo tipo inferido por campo, para todos los monitores. No hay tolerancia configurable.
- **Validación por fila para monitores de tipo `json`.** `ingestDelimited` (CSV/TXT) e `IngestExcel` comparten la misma forma de fila (`[]string`, valores sin tipar todavía) — la validación por fila se implementa una sola vez y cubre ambos. `IngestJSON` ya recibe valores tipados nativamente por el parser de JSON (`encoding/json`), una forma de dato distinta que necesitaría su propio validador — queda fuera de este plan, se agrega en una iteración aparte si hace falta. La validación de ESTRUCTURA completa (`compareSchema`, rechazo de archivo) sí aplica a los 4 formatos por igual.

## Diferencia con el comportamiento actual

`monitor.Schema` ya se fija en el primer upload y no se pisa en los siguientes (`ingestion_service.go:200-201,279-280,377-378`: `if len(monitor.Schema) == 0 { monitor.Schema = detectSchemaFromCSV(...) }`) — este feature no cambia ese mecanismo, lo aprovecha como la "base" contra la que se compara. Lo que falta hoy es que nada compare un upload nuevo contra esa base antes de escribir en Mongo.

## Arquitectura

### Comparación de estructura (función pura, nueva)

En `ingestion_service.go`, una función que toma el schema detectado del archivo recién subido y el `monitor.Schema` existente, y devuelve un diff:

```go
type SchemaDiff struct {
    Match         bool
    MissingFields []string       // en el monitor, no en el archivo
    ExtraFields   []string       // en el archivo, no en el monitor
    TypeMismatches []FieldTypeMismatch
}

type FieldTypeMismatch struct {
    Field        string
    ExpectedType models.FieldType
    ActualType   models.FieldType
}

func compareSchema(expected []models.SchemaField, actual []models.SchemaField) SchemaDiff
```

Nombres se comparan como conjunto (sin importar orden). `Match` es `true` solo si `MissingFields`, `ExtraFields` y `TypeMismatches` están todos vacíos. Si `len(expected) == 0` (monitor sin datos todavía, primer upload), `Match` es siempre `true` — no hay base contra la cual comparar.

### Validación por fila (nueva, dentro de cada `Ingest*`)

Para un archivo que pasó la comparación de estructura, cada fila se valida contra el tipo esperado de cada campo (reusando `inferType`/`looksLikeDate`, ya existentes en `ingestion_service.go:508,526`). Una fila cuyo valor no puede interpretarse como el tipo esperado del campo (ej. `monto` es `FieldNumber` y la fila trae `"n/a"`) se excluye de las `documents` a insertar, y se registra en una lista de `RowRejection{RowIndex int, Field string, Reason string}` que va a la bitácora. El resto de las filas válidas se ingieren igual que hoy.

### Endpoint de verificación previa (dry-run, nuevo)

`POST /monitors/:id/upload/check` — mismo `multipart.File` que `/upload`, pero corre SOLO el parseo + `compareSchema` + validación por fila, sin llamar `InsertData` ni escribir en la bitácora. Devuelve:

```json
{
  "match": false,
  "missingFields": ["clavepais"],
  "extraFields": ["ticket"],
  "typeMismatches": [{"field": "monto", "expectedType": "number", "actualType": "string"}],
  "totalRows": 200,
  "rowsThatWouldPass": 195,
  "rowsThatWouldFail": 5
}
```

El frontend llama este endpoint automáticamente al hacer click en "Subir" (Flujo 3). Reusa la misma función `compareSchema` y la misma validación por fila que el upload real — no hay lógica duplicada, el handler real simplemente además persiste.

### `UploadData` actualizado (`monitor_handler.go:304`)

Después de parsear y detectar el schema del archivo, antes de `InsertData`:
1. `diff := compareSchema(monitor.Schema, detectedSchema)`
2. Si `!diff.Match`: NO se ingiere nada. Se sube el archivo original a GridFS (bucket nuevo `upload_rejections_files`, mismo patrón que `red_flag_report_repo.go`). Se crea una entrada en la bitácora con estado `rejected_structure`, el diff, y el `gridfs_file_id`. Se responde `422` con el mismo diff que el dry-run.
3. Si `diff.Match`: se ingieren las filas válidas (validación por fila incluida). Se crea una entrada en la bitácora con estado `accepted` (0 rechazos de fila) o `partial` (algunos rechazos de fila), con `RowRejection[]` si aplica. `monitor.RecordCount` se incrementa solo en `RowsAccepted` (el conteo real insertado), nunca en `TotalRows` — mismo criterio que ya usa `ingestDelimited` hoy (`count` devuelto por `InsertData`, no el total de filas leídas).

### Endpoint de aprobación (nuevo)

`POST /monitors/:id/upload-log/:logId/approve` — protegido con `RequireComplianceOrAbove()` (mismo middleware que ya protege `/upload`). Solo válido sobre una entrada con estado `rejected_structure` y `gridfs_file_id` presente. Descarga el archivo de GridFS, lo re-ingiere (mismo camino que un upload normal, pero forzando `diff.Match = true` porque el usuario ya confirmó el cambio), actualiza `monitor.Schema` a la nueva estructura, actualiza la entrada de la bitácora a estado `approved`, y borra el blob de GridFS tras la ingesta exitosa (mismo cuidado de no dejar binarios huérfanos que ya tiene `red_flag_report_repo.go:87`).

Si `gridfs_file_id` no está presente (GridFS no estaba disponible al momento del rechazo — ver Manejo de errores), el endpoint devuelve un error explicando que hay que volver a subir el archivo.

## Modelo de datos

Colección nueva `upload_log`:

```go
type UploadLogEntry struct {
    ID              primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
    MonitorID       primitive.ObjectID   `bson:"monitor_id" json:"monitorId"`
    MonitorName     string               `bson:"monitor_name" json:"monitorName"`
    FileName        string               `bson:"file_name" json:"fileName"`
    SourceType      models.SourceType    `bson:"source_type" json:"sourceType"`
    UploadedBy      primitive.ObjectID   `bson:"uploaded_by" json:"uploadedBy"`
    UploadedByEmail string               `bson:"uploaded_by_email" json:"uploadedByEmail"`
    UploadedAt      time.Time            `bson:"uploaded_at" json:"uploadedAt"`
    Status          UploadStatus         `bson:"status" json:"status"` // "accepted" | "partial" | "rejected_structure" | "approved"
    TotalRows       int                  `bson:"total_rows" json:"totalRows"`
    RowsAccepted    int                  `bson:"rows_accepted" json:"rowsAccepted"`
    RowsRejected    int                  `bson:"rows_rejected" json:"rowsRejected"`
    SchemaDiff      *SchemaDiff          `bson:"schema_diff,omitempty" json:"schemaDiff,omitempty"`
    RowRejections   []RowRejection       `bson:"row_rejections,omitempty" json:"rowRejections,omitempty"`
    SampleRows      []map[string]any     `bson:"sample_rows,omitempty" json:"sampleRows,omitempty"` // primeras 5 filas, para auditoría
    GridFSFileID    *primitive.ObjectID  `bson:"gridfs_file_id,omitempty" json:"-"` // solo en rejected_structure, se borra al aprobar
}
```

`SampleRows` limitado a 5 filas — suficiente para auditoría sin duplicar el archivo completo en la colección de metadata (el archivo completo, cuando aplica, ya vive en GridFS).

Índices: `{monitor_id: 1, uploaded_at: -1}` (filtro por monitor + orden cronológico, el acceso más común) y `{status: 1}` (filtro por estado).

## Frontend — extensión de `/uploads`

La página existente (`frontend/src/app/(dashboard)/uploads/page.tsx`) ya tiene: búsqueda por texto, filtro por tipo de fuente, rango de fechas, orden. Se agrega:

- **Filtro por monitor** (dropdown de selección exacta, no solo el texto libre que ya existe).
- **Filtro por estado** (Todos / Aceptado / Parcial / Rechazado / Aprobado).
- **Tabla de entradas individuales**, nueva, debajo del resumen agregado por día que ya existe (convive con él, no lo reemplaza — evita regresión de lo que ya funciona), con columnas: Archivo, Monitor, Fecha/hora, Subido por, Estado (badge), Registros totales, Aceptados, Rechazados.
- **Fila expandible** en entradas `rejected_structure` o `partial`: muestra el diff de columnas (`MissingFields`/`ExtraFields`/`TypeMismatches`) o la lista de `RowRejections`.
- **Botón "Aprobar"** en filas `rejected_structure`, visible solo si el usuario es `admin`/`compliance` (mismo criterio de rol que ya usa el botón de subida).

### Diálogo de verificación previa (nuevo, en el formulario de subida de cada monitor)

Al hacer click en "Subir" (ya sea en la página de detalle de un monitor o donde exista el control de carga): antes de enviar el archivo al endpoint real, se llama `/upload/check`. Si `match === false` o `rowsThatWouldFail > 0`, se muestra un diálogo con el mismo diff visual que la fila expandible de la bitácora, y dos botones: "Cancelar" (no se sube nada) y "Subir de todas formas" (procede con el upload real, que generará la entrada correspondiente en la bitácora). Si todo coincide, se sube directo sin diálogo.

## Manejo de errores

- **Archivo ilegible** (no parsea como el `SourceType` configurado): error `400` distinto de un rechazo de estructura — nunca llega a `compareSchema`, no genera entrada en la bitácora (no hubo un intento real de comparar nada, es un error de formato).
- **GridFS no disponible** al momento de un rechazo de estructura: se loguea la entrada igual (con `gridfs_file_id: nil`), el rechazo se registra, pero la aprobación posterior pedirá volver a subir el archivo en vez de re-ingerir automático (mismo patrón defensivo que `red_flag_report_repo.go:51-55`, que no tumba el servidor si el bucket no abre).
- **Filas rechazadas por tipo**: no interrumpen el resto del archivo — se acumulan en `RowRejections` y se restan del conteo de aceptados.

## Testing

- `compareSchema`: TDD puro — match exacto, campo faltante, campo de más, mismatch de tipo, orden de columnas irrelevante, `expected` vacío (primer upload) siempre matchea.
- Validación por fila: fila válida, fila con tipo incorrecto por campo, fila con menos columnas de las esperadas.
- Sin tests de UI/componentes — mismo criterio que el resto del proyecto (no hay convención de testing de componentes acá, confirmado en el sistema de tabs recién implementado).

## Persistencia y privacidad

`SampleRows` y `RowRejections` pueden contener datos sensibles (montos, ids de cuenta, nombres — mismo dominio AML que el resto del proyecto). El acceso de lectura a la bitácora sigue el mismo criterio que `IngestionHistory` hoy (`router.go:92`, cualquier usuario autenticado — no hay restricción de rol para VER); la acción de aprobar sí está restringida a `admin`/`compliance` (`RequireComplianceOrAbove()`), igual que la subida misma. No se aplica ningún recorte adicional tipo `partialize` (a diferencia del sistema de tabs) — esta bitácora es un registro de auditoría de compliance, su propósito explícito es la trazabilidad completa, no minimizar retención.
