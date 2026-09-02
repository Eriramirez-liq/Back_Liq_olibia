---
name: project-netsuite-integration
description: Estado del plan de integracion Cargos STR con Oracle NetSuite — fase actual, decisiones clave y dependencias pendientes
metadata:
  type: project
---

El plan de integracion frontend NetSuite esta documentado en `mejoras/netsuite-frontend-plan.md` (consolidado 2026-05-22). El plan del Arquitecto esta en `mejoras/netsuite-integration-plan.md`.

**Why:** la integracion agrega OC (Ordenes de Compra) en NetSuite para cada fila del pivot Cargos STR. Es la funcionalidad principal pendiente del modulo.

**How to apply:** antes de tocar `app/(dashboard)/cargos-str/page.tsx` o `components/cargos-str/`, leer ambos planes. El Addendum 2026-05-20 prevalece sobre el cuerpo principal en caso de conflicto. Las secciones marcadas `[OBSOLETO]` no deben implementarse.

## Decisiones resueltas (2026-05-22)

- **D1 RESUELTO:** `/estados` usa `codigo` (string "OR-AFINIA"). Keys del mapa: `${periodoId}|${codigo}`.
- **D4 PENDIENTE confirmacion:** se asume `GET /api/cargos-str/netsuite/envio/:id` (Opcion A). El Arquitecto debe confirmar antes de FE-6.
- **Nombre del boton RESUELTO:** `BotonCrearOC.tsx` con label "Crear OC". `BotonGenerarOC` es obsoleto.
- **Firma toggleSeleccion RESUELTO:** `toggleSeleccion(orId: string)` — sin periodoId, seleccion por fila.
- **Componentes para FE-2 RESUELTO:** `FilaOperador.tsx` + `CeldaMonto.tsx` (no `CeldaCargo.tsx`).

## Estado del plan de PRs

- FE-1: Skeleton + tipos TS — COMPLETADO 2026-05-22. 8 archivos en `components/cargos-str/` + `_dev/mocks/netsuite.ts`. tsc pasa sin errores. Sección "Progreso de ejecucion" agregada al plan.
- FE-2: Tabla con FilaOperador + CeldaMonto + BotonCrearOC (pendiente — bloqueado hasta validacion de Erika sobre FE-1)
- FE-3: Badges + DetalleModal (pendiente)
- FE-4: ModalConfirmar + creacion lote (pendiente)
- FE-5: Panel + polling (pendiente)
- FE-6: Integracion real (depende de APIs backend + confirmacion D4)
- FE-7: Pulido + a11y (pendiente)

## Archivos creados en FE-1 (2026-05-22)

- `components/cargos-str/types.ts` — todos los tipos compartidos
- `components/cargos-str/FilaOperador.tsx` — stub, return null
- `components/cargos-str/CeldaMonto.tsx` — stub, return null
- `components/cargos-str/BotonCrearOC.tsx` — stub funcional minimo (button con label)
- `components/cargos-str/ModalConfirmarLote.tsx` — stub condicional (null si !abierto)
- `components/cargos-str/PanelLoteEnCurso.tsx` — stub, return null
- `components/cargos-str/DetalleEnvioModal.tsx` — stub condicional (null si !abierto)
- `_dev/mocks/netsuite.ts` — 4 exports de mocks (ESTADOS_ENVIO, LOTE_EN_CURSO, DETALLE_OK, DETALLE_ERROR)
- `.gitignore` — se agrego `_dev/` al final

## Endpoints nuevos que el frontend necesita del backend

Todos en `/api/cargos-str/netsuite/`:
- `POST /lote`
- `POST /lote/:id/procesar`
- `GET /lote/:id`
- `GET /estados?periodoIds=&orIds=` — los orIds son codigos string, no UUIDs (D1 resuelto)
- `GET /envio/:id` — asumido (Opcion A, D4 pendiente confirmacion del Arquitecto)
- `POST /envio/:id/reenviar`
- `POST /lote/:id/cancelar`
- `GET /lote/activo` (sugerido por frontend, no esta en el plan del Arquitecto — pendiente acordar)
