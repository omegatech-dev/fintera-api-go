## 1. Parser de fecha tolerante

- [x] 1.1 Agregar `parseProjectCSVDate(value string) (string, error)` en `internal/services/project_csv.go`: probar `2006-01-02` primero, luego `DD/MM/YYYY`, `M/D/YYYY`, `DD-MM-YYYY`, `YYYY/MM/DD` con desambiguación día/mes (primer componente > 12 → mes/día; segundo componente > 12 → mes/día; empate → día/mes), devolviendo siempre `YYYY-MM-DD`
- [x] 1.2 Usar `parseProjectCSVDate` en `BuildProject` para `delivery_date`, guardando el valor normalizado en `project.DeliveryDate`
- [x] 1.3 Actualizar el mensaje de error de fecha a: `delivery_date debe tener formato YYYY-MM-DD (también se acepta DD/MM/YYYY o M/D/YYYY)`

## 2. Tests

- [x] 2.1 Agregar tests unitarios en `internal/services/project_csv_test.go` para `parseProjectCSVDate`: ISO (`2027-06-30`), `30/06/2027`, `6/30/2027`, `30-06-2027`, `2027/06/30`, ambigua `05/06/2027` → `2027-06-05`, inválidas (`31/02/2027`, texto) y fechas de 1 dígito (`6/3/2027`)
- [x] 2.2 Agregar test de integración en `BuildProject`/`ParseProjectCSV` con un CSV cuyas filas usan formatos de fecha mixtos, verificando importación exitosa y normalización

## 3. Verificación

- [ ] 3.1 Ejecutar `go build ./...` y `go test ./...` desde la raíz
- [ ] 3.2 Ejecutar `gofmt`/`make fmt` sobre los archivos modificados y confirmar que `go vet ./internal/services` no reporta problemas
- [ ] 3.3 Probar con el archivo real de reproducción (`plantilla-proyectos-ejemplo(in) (1) utf-8.csv` con fechas `6/30/2027` y `10/20/2027`) y confirmar que ambas filas importan con fecha normalizada

## 4. Documentación

- [x] 4.1 Actualizar `docs/architecture/` (guía o `api-and-auth.md`) si documenta los formatos aceptados por la importación CSV de proyectos
- [x] 4.2 Registrar tarea de seguimiento para alinear el validador del frontend (`src/features/projects/csvImport.js` en el repo web) con los formatos aceptados

## Seguimiento (fuera de este cambio)

- **Frontend (repo web)**: el validador de la vista previa del importador (`src/features/projects/csvImport.js`, comentario en `project_csv.go` lo referencia) replica la validación de `fecha_entrega` con `YYYY-MM-DD` estricto. Alinearlo con los formatos aceptados por el backend (DD/MM/YYYY, M/D/YYYY, DD-MM-YYYY, YYYY/MM/DD con preferencia día-primero) para que la vista previa no marque como error filas que el backend aceptaría.
