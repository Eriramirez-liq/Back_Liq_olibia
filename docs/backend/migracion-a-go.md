# El backend se porta a Go, y se despliega desde `bia-bills`

> **Última actualización:** 2026-08-17
> **Estado:** Fase 0 hecha (esqueleto). Ver el plan por fases al final.

---

## Por qué

Todos los backends de la organización están en Go, y **cactus solo despliega
repos de la organización**. Este repo es personal y privado: el intento de
desplegarlo falló al clonar, y los Deploy Keys no sirven porque la llave de
cactus ya está registrada en otro repositorio (GitHub permite una sola por
llave).

La decisión fue: **desarrollar acá en Go, y trasvasar a `bia-bills` lo que se
quiera desplegar**, porque ese repo sí está autorizado en cactus.

---

## Las dos reglas que sostienen el arreglo

**1 · El módulo se llama `bia-bills`.**

Nuestro `go.mod` declara `module bia-bills`, igual que el suyo, y las carpetas
están en las **mismas rutas exactas**. Así un archivo se copia con `cp` a la
misma ruta y **ningún import cambia**. Si el módulo tuviera otro nombre, cada
trasvase exigiría reescribir imports — y ahí es donde se cuelan los errores.

**2 · Nunca editar en `bia-bills`.**

Siempre acá, y copiar. En cuanto alguien parchea directo allá, los dos repos
divergen y el trasvase deja de ser mecánico. Si algo hay que arreglar sobre lo
ya desplegado, se arregla acá y se vuelve a copiar.

---

## Estructura (espejo de bia-bills)

| Carpeta | Qué va |
|---|---|
| `entities/` | Tipos de dominio y constantes. Sin infraestructura. |
| `models/` | Structs que mapean tablas (GORM). Una por tabla, sin lógica. |
| `mappers/` | Traducción entre `models` (BD) y `entities` (dominio). |
| `repositories/` | Acceso a datos. Interfaz por repositorio, para poder mockear. |
| `services/<dominio>/` | Lógica de negocio. **Un paquete por dominio**, como `services/bill` en bia-bills — nunca archivos en la raíz de `services/`. |
| `controllers/` | Handlers HTTP delgados: parsean, delegan, serializan. |
| `providers/postgres/` | Conexión y migraciones. |
| `router/` | Armado de rutas e inyección de dependencias manual. |
| `errors/` | Errores de dominio y su traducción a HTTP. |
| `utils/` | Helpers sin dominio. |

## Herramientas

Las mismas de bia-bills, en el `Makefile`: `staticcheck`, `golangci-lint`,
`mockery --all --keeptree`, `swag`, `go test` con cobertura.

**Su CI corre sobre nuestro código** una vez trasvasado: SonarQube, staticcheck
y golangci-lint se ejecutan en `bia-bills`. Lo que copiemos tiene que pasar sus
umbrales de calidad y cobertura, no solo compilar.

## Prerrequisito verificado

`bia-commons-go` es un módulo privado de la organización. El acceso ya funciona
con las credenciales de git de esta máquina:

```bash
GOPRIVATE="github.com/biaenergy/*" go list -m github.com/biaenergy/bia-commons-go@v1.25.2
# → github.com/biaenergy/bia-commons-go v1.25.2
```

En CI, bia-bills lo resuelve con `ssh-add` y
`git config url."git@github.com:".insteadOf https://github.com`.

---

## Lo que cambia respecto de la versión TypeScript

**La autenticación desaparece.** Los servicios de BIA no validan tokens: leen la
identidad de un header que inyecta la plataforma.

```go
ctx := contextBia.RequestContext(c)   // trae x-user-id y x-request-id
```

El proxy de olibia ya manda `x-user-id`. Así que el puente que armamos —`jose`,
validación contra las llaves de Google, mapeo a `users`— **no se porta**. Son
~200 líneas menos y una clase entera de complejidad que se va.

**Tres bases, no una.** `NewPostgresDB` de bia-bills abre **una sola** conexión a
su propia base. Cargos STR necesita además `file-compiler` y `calculator-prices`,
así que el provider multi-base es pieza nueva en su código y conviene diseñarla
bien.

**Las URLs cambian.** Lo desplegado en bia-bills vive bajo su prefijo:
`/ms-bill/liquidations/...`. Cada endpoint migrado se agrega con esa URL en el
`endpoints.ts` del front, y el resto sigue apuntando al backend TypeScript
mientras exista.

**Las migraciones se numeran en su cadena.** Formato golang-migrate,
`NNN_nombre.up.sql` / `.down.sql`, continuando desde la 192 de bia-bills.

---

## Plan por fases

| Fase | Qué | Estado |
|---|---|---|
| **0** | Esqueleto: `go.mod`, estructura espejo, Makefile, acceso a módulos privados | ✅ hecha |
| **1** | Provider multi-base, router con `ginCommons`, endpoint de humo | pendiente |
| **2** | Vertical STR: parser con `excelize`, repositorios, servicio, controller, tests | pendiente |
| **3** | Trasvase a bia-bills: copiar, registrar rutas, migraciones, variables, PR | pendiente |
| **4** | Resto de módulos, uno por uno | pendiente |

El TypeScript de cada módulo se borra cuando su versión Go está en producción.

## El riesgo a vigilar

**Los parsers.** El de STR está validado al peso contra archivos reales:

```
CHEC = 70.812.140 − 4.442 = 70.807.698     lote = 1.460.833.304
```

Reescribirlo en Go es donde más fácil se cuela una regresión silenciosa —
detección automática de la fila de encabezados, búsqueda flexible de la fila
`BIAC-BIAE`, homologación de columnas, orden de los ajustes. **No dar por bueno
el parser en Go hasta que reproduzca esos números con los mismos archivos.**
