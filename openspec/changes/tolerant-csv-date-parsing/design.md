## Context

El importador de proyectos por CSV (`internal/services/project_csv.go`) valida `fecha_entrega` con `time.Parse("2006-01-02", ...)` de forma estricta. Los usuarios editan la plantilla en Excel y, al guardar, Excel reformatea la columna de fechas al formato regional (EE. UU.: `6/30/2027`; latinoamericano: `30/06/2027`), lo que hace que la reimportación falle con errores por fila. Verificado con un archivo real: `plantilla-proyectos-ejemplo(in) (1) utf-8.csv` con fechas `6/30/2027` y `10/20/2027` produce "0 filas listas, 2 con errores".

La validación por fila ocurre en `BuildProject` (y por extensión en `MergeInto`); el parseo de archivo, delimitador y codificación no cambia.

## Goals / Non-Goals

**Goals:**
- Aceptar `fecha_entrega` en los formatos que Excel y los usuarios producen habitualmente, sin bajar la validación de fechas inválidas.
- Normalizar todo valor aceptado a `YYYY-MM-DD` antes de persistir.
- Mensaje de error claro que indique los formatos aceptados.

**Non-Goals:**
- No se modifica la lógica de otros imports (clientes históricos usan su propio parser).
- No se cambia el formato almacenado ni el contrato de la API.
- No se alinea el frontend (otro repositorio); se anota como seguimiento.

## Decisions

1. **Parser de fecha con lista de layouts y desambiguación día/mes.**
   Se agrega una función `parseProjectCSVDate(value string) (string, error)` que:
   - Prueba primero `2006-01-02` (formato canónico, sin ambigüedad).
   - Luego intenta los layouts slash/dash: `02/01/2006`, `01/02/2006`, `02-01-2006`, `2006/01/02`, aceptando también 1 dígito para mes/día (`2/1/2006` vía normalización previa con `time.Parse` que ya tolera componentes de 1-2 dígitos en los layouts `01`/`02`).
   - Desambiguación: se prefiere la interpretación día/mes (convención local); si el primer componente es > 12, se interpreta mes/día. Si el segundo componente es > 12, obligatoriamente es mes/día. Si ambos ≤ 12, gana día/mes.
   - Devuelve siempre la fecha re-formateada a `2006-01-02`.
   *Alternativa considerada:* librería de parsing flexible (p. ej. `araddon/dateparse`). Se descarta por añadir una dependencia externa para un caso acotado.

2. **Normalización en un solo punto.** `BuildProject` guarda en `project.DeliveryDate` el valor normalizado; el resto del pipeline no cambia. Así `MergeInto` y la creación heredan la normalización gratis.

3. **Mensaje de error informativo.** El error de fecha pasa a decir: `delivery_date debe tener formato YYYY-MM-DD (también se acepta DD/MM/YYYY o M/D/YYYY)`.

## Risks / Trade-offs

- [Ambigüedad en fechas `05/06/2027`] → Se documenta la preferencia día/mes y se cubre con tests; el formato canónico de la plantilla (`YYYY-MM-DD`) nunca es ambiguo y se evalúa primero.
- [Usuarios que dependían del rechazo estricto] → Es una relajación de entrada con validación de rangos reales intacta (`time.Parse` rechaza `31/02/2027`); el rollback es trivial (revert del commit).
- [Deriva con el frontend] → El validador del portal replica esta lógica; si no se alinea, la vista previa podría marcar filas que el backend aceptaría. Se deja como tarea de seguimiento en el otro repo.

## Migration Plan

- Sin migraciones ni cambios de esquema. Despliegue normal; rollback = revert.
