## ADDED Requirements

### Requirement: La importación de proyectos acepta archivos CSV con delimitador coma o punto y coma
El sistema SHALL aceptar archivos CSV con delimitador coma o punto y coma, en codificación UTF-8 (con o sin BOM) o Windows-1252, y SHALL detectar el delimitador automáticamente a partir de la fila de encabezados.

#### Scenario: Archivo con delimitador punto y coma
- **WHEN** se sube un CSV cuyos encabezados contienen más puntos y comas que comas
- **THEN** el sistema parsea las filas usando `;` como separador

#### Scenario: Archivo UTF-8 con BOM o Windows-1252
- **WHEN** se sube un archivo que comienza con BOM UTF-8 o que no es UTF-8 válido
- **THEN** el sistema lo decodifica y lo procesa sin errores de codificación

### Requirement: La importación exige las columnas requeridas
El sistema SHALL rechazar la importación completa cuando el CSV no incluye los encabezados de `nombre`, `direccion` (o sus alias/ equivalentes en inglés) y `precio_m2`/`precio por unidad`.

#### Scenario: Faltan columnas requeridas
- **WHEN** el encabezado del CSV no contiene ninguna columna que mapee a un campo requerido
- **THEN** el sistema responde con un error que lista las columnas faltantes y no importa ninguna fila

### Requirement: La fecha de entrega acepta formatos de fecha comunes
El sistema SHALL aceptar `fecha_entrega` en formato `YYYY-MM-DD` y SHALL, además, aceptar los formatos comunes `DD/MM/YYYY`, `M/D/YYYY`, `DD-MM-YYYY` y `YYYY/MM/DD`, normalizando todo valor aceptado al formato canónico `YYYY-MM-DD` antes de persistir.

#### Scenario: Fecha en formato ISO
- **WHEN** una fila trae `fecha_entrega` con el valor `2027-06-30`
- **THEN** el proyecto se importa con fecha de entrega `2027-06-30`

#### Scenario: Fecha en formato día/mes/año con separador slash
- **WHEN** una fila trae `fecha_entrega` con el valor `30/06/2027`
- **THEN** el proyecto se importa con fecha de entrega normalizada a `2027-06-30`

#### Scenario: Fecha en formato mes/día/año inequívoco
- **WHEN** una fila trae `fecha_entrega` con el valor `6/30/2027`
- **THEN** el sistema lo interpreta como mes/día/año (el día `30` no puede ser mes) y el proyecto se importa con fecha `2027-06-30`

#### Scenario: Fecha en formato con guiones
- **WHEN** una fila trae `fecha_entrega` con el valor `30-06-2027`
- **THEN** el proyecto se importa con fecha de entrega `2027-06-30`

#### Scenario: Fecha ambigua día/mes
- **WHEN** una fila trae `fecha_entrega` con un valor slash ambiguo como `05/06/2027`
- **THEN** el sistema lo interpreta como día/mes/año (`5 de junio de 2027`)

#### Scenario: Fecha inválida
- **WHEN** una fila trae `fecha_entrega` con un valor que no corresponde a ninguna fecha válida en los formatos aceptados (p. ej. `31/02/2027` o `próximamente`)
- **THEN** esa fila se reporta como error indicando los formatos de fecha aceptados y no se importa esa fila, sin afectar a las demás

### Requirement: Los errores de fila no abortan la importación completa
El sistema SHALL validar cada fila de datos de forma independiente y SHALL continuar importando las filas válidas, reportando al final el número de filas importadas, actualizadas, omitidas y los errores por número de fila.

#### Scenario: Mezcla de filas válidas e inválidas
- **WHEN** el CSV contiene una fila con fecha inválida y otra fila completamente válida
- **THEN** la fila válida se importa y la inválida aparece en la lista de errores con su número de fila
