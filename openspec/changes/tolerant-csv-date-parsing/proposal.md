## Why

La importación de proyectos por CSV rechaza filas completas cuando `fecha_entrega` no está exactamente en formato `YYYY-MM-DD`. En la práctica, los usuarios editan la plantilla en Excel y este reformatea la columna de fechas al formato regional (p. ej. `6/30/2027` o `30/06/2027`), por lo que la reimportación de la propia plantilla falla con "0 filas listas, N con errores". Es un punto de fricción recurrente de soporte que se resuelve hoy corrigiendo el CSV a mano.

## What Changes

- El parser de la importación de proyectos CSV (`internal/services/project_csv.go`) acepta `fecha_entrega` en formatos de fecha comunes además de `YYYY-MM-DD`: `DD/MM/YYYY`, `M/D/YYYY`, `DD-MM-YYYY`, `YYYY/MM/DD`, con desambiguación cuando el valor lo permite.
- Los valores aceptados se normalizan a `YYYY-MM-DD` antes de persistir, para que el almacenamiento y la API sigan siendo consistentes.
- Los mensajes de error de fecha se aclaran para indicar los formatos aceptados.
- El CSV sigue siendo la fuente: la lógica de fechas de otros imports (clientes históricos) no se toca.

## Capabilities

### New Capabilities

- `project-csv-import`: Comportamiento de la importación de proyectos por CSV, incluyendo detección de delimitador, columnas requeridas, validación y normalización de valores por fila (unidades, tipos, tasas y fechas de entrega).

### Modified Capabilities

<!-- Ninguna: no existen specs previas en openspec/specs/ -->

## Impact

- **Código**: `internal/services/project_csv.go` (solo `BuildProject`, parseo de `delivery_date`) y `internal/services/project_csv_test.go` (nuevos tests).
- **API**: sin cambios de contrato; el campo persistido sigue siendo `YYYY-MM-DD`. Comportamiento más tolerante en la entrada del endpoint de importación de proyectos.
- **Frontend**: la vista previa del importador (otro repositorio, `csvImport.js`) replica esta validación; quedará pendiente alinearla en un cambio separado.
- **Riesgo**: bajo; es relajación de un formato de entrada con normalización posterior y tests que cubren la desambiguación día/mes.
