---
name: project-stack-conventions
description: Convenciones del stack frontend de este proyecto — estilos, paleta, estructura, patrones que no deben violarse
metadata:
  type: project
---

**Why:** el proyecto tiene convenciones muy especificas que no son obvias a primera vista. Violarlas introduce inconsistencia que molesta al equipo.

**How to apply:** antes de escribir cualquier JSX o CSS, verificar estas convenciones.

## Estilos: INLINE solamente en paginas de dominio

Las paginas en `app/(dashboard)/` usan **inline styles** (`style={{ ... }}`), NO Tailwind. Tailwind solo se usa en `components/ui/` (shadcn) y en el layout del dashboard (`layout.tsx`). No mezclar.

## Paleta de colores — USAR ESTOS VALORES EXACTOS

```
Teal accent (primario):   #07c5a8
Azul header oscuro:       #1e3a8a
Azul header claro:        #dbeafe
Azul header más claro:    #eff6ff
Azul borde:               #bfdbfe
Verde exito text:         #15803d
Verde exito bg:           #f0fdf4
Verde exito borde:        #d1fae5
Rojo error text:          #b91c1c
Rojo error bg:            #fef2f2
Amarillo pendiente text:  #b45309
Amarillo pendiente bg:    #fff7ed
Amarillo pendiente borde: #fde68a
Gris texto primario:      #111827
Gris texto secundario:    #374151
Gris muted:               #6b7280
Gris placeholder:         #9ca3af
Gris borde:               #e5e7eb
Gris bg sutil:            #f9fafb
Blanco cards:             #fff
```

## Estructura de carpetas

```
app/(dashboard)/cargos-str/page.tsx   <- pagina principal del modulo
components/cargos-str/                <- componentes del modulo (carpeta nueva para NetSuite)
components/ui/                        <- componentes shadcn (no modificar sin razon)
components/layout/                    <- Sidebar, TopBar (no modificar sin razon)
```

## Patron de componentes en page.tsx

Los componentes `ResultsTable` y `MultiSelect` estan definidos **inline en el mismo archivo** `page.tsx`, no en archivos separados. Los nuevos componentes de NetSuite van en `components/cargos-str/` (archivos separados) porque son reutilizables y más grandes.

## Framework

- Next.js 15 App Router
- React 19
- TypeScript estricto (nunca `any`)
- `'use client'` en todas las paginas de dominio (son interactivas)

## No hay TanStack Query ni SWR

El repo no tiene ninguna libreria de data fetching. Usar `fetch` + `useEffect` + `useState`. No agregar dependencias sin justificacion.
