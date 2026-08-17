# El backend se porta a Go, y se despliega desde `bia-bills`

> **Última actualización:** 2026-08-17
> **Estado:** Fases 0 y 1 hechas. Ver el plan por fases al final.

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

## Las cinco reglas que sostienen el arreglo

**1 · El módulo se llama `bia-bills`.**

Nuestro `go.mod` declara `module bia-bills`, igual que el suyo, y las carpetas
están en las **mismas rutas exactas**. Así un archivo se copia con `cp` a la
misma ruta y **ningún import cambia**. Si el módulo tuviera otro nombre, cada
trasvase exigiría reescribir imports — y ahí es donde se cuelan los errores.

**2 · Nunca editar en `bia-bills`.**

Siempre acá, y copiar. En cuanto alguien parchea directo allá, los dos repos
divergen y el trasvase deja de ser mecánico. Si algo hay que arreglar sobre lo
ya desplegado, se arregla acá y se vuelve a copiar.

**3 · Todo archivo nuestro lleva prefijo `liquidations_`.**

Sus paquetes ya tienen `entities/const.go`, `providers/postgres/database.go`,
`router/router.go` y 75 controllers. Un archivo nuestro con uno de esos nombres
**pisaría el suyo** al copiar. Por eso: `entities/liquidations_const.go`,
`providers/postgres/liquidations_database.go`, `router/liquidations_router.go`,
`controllers/liquidations_health.go`. Los servicios son la excepción: van en su
propio subdirectorio (`services/cargos_str/`), que ya es un nombre nuevo.

**4 · El módulo no referencia símbolos de bia-bills que no declare él mismo.**

Aprendido a los golpes: el provider usaba `entities.ProdEnviroment`, que existe
allá pero no acá — este repo no compilaba. Y declararlo acá habría colisionado al
copiar el archivo a su paquete `entities`. La salida es que el módulo sea
autocontenido: `LiqSQLDebug` lee la misma variable de entorno con nombre propio.

Vale para constantes, helpers y tipos. Lo único compartido son los paquetes
externos: `bia-commons-go`, gin, gorm.

**5 · Una sola línea de edición manual en su `router.go`.**

Toda la inyección de dependencias del módulo vive en
`router/liquidations_router.go`. Registrar el módulo allá es agregar:

```go
RegisterLiquidations(apiPrefix)
```

Nada más. Todo el resto es copiar archivos.

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
| **1** | Provider multi-base, router con `ginCommons`, endpoint de diagnóstico | ✅ hecha |
| **2** | Vertical STR: parser con `excelize`, repositorios, servicio, controller, tests | ✅ hecha |
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

Reproducidos: por el parser en Go, por el servicio, y end to end contra las
bases de dev (23 operadores, los 23 nombres resueltos).

---

## Tests

`make test` corre todo. Los de integración se saltean solos si no hay
credenciales, así que sin VPN también pasa.

**Por qué hay tests con sqlmock además de los de integración.** `file-compiler`
y `calculator-prices` son RDS externos: el CI de bia-bills no los alcanza —su
`docker-compose` levanta la base *del servicio*, con migraciones, y las nuestras
no viven ahí. Sin los de sqlmock, `repositories` y `providers/postgres`
reportarían 0% allá y SonarQube tumba el PR.

| paquete | sin base | con base |
|---|---|---|
| `controllers` | 89.5% | 89.5% |
| `models` | 100% | 100% |
| `providers/postgres` | 34.8% | 91.3% |
| `repositories` | 100% | 100% |
| `router` | 100% | 100% |
| `services/cargos_str` | 94.5% | 94.5% |

El resto de `providers/postgres` es el código que abre conexiones de verdad; no
se puede cubrir sin base y no debería.

Lo que cada uno protege:

- **`router`** — los paths exactos del `endpoints.ts` del front. Es el único
  test que se queja si renombrar una ruta rompe la pantalla de Cargos STR.
- **`repositories`** — el `DISTINCT ON` en toda lectura. Sin él se suman las
  cargas viejas con las nuevas y los montos se **duplican**; con cifras de mil
  millones eso no se ve en pantalla. También fija `IN (?)` (ver más abajo) y a
  qué base va cada escritura.
- **`services/cargos_str`** — el test dorado contra los archivos reales de
  `testdata/`, versionados a propósito.
- **`models`** — los `TableName`, y que los ajustes sean punteros: `NULL` es "ese
  archivo de refactura no vino" y `0` es "vino en cero".

**Los tests se validaron por mutación.** Se rompió el código a mano en seis
puntos —quitar el `DISTINCT ON`, volver al `ANY(?)`, cruzar las bases de
escritura, cambiar un path, errar un nombre de tabla, sacar el filtro de
`OPERADOR DE RED`— y los seis dieron rojo. Vale repetirlo al tocar estas
consultas: un test que no falla cuando debería es peor que no tener test.

### Tres trampas que ya costaron una sesión cada una

1. **GORM no expande slices en `= ANY(?)`.** Genera `ANY('CMMD','CSID',…)` y
   Postgres responde `syntax error at or near ","`. Va `IN (?)`. Estaba en cuatro
   consultas y el síntoma era traicionero: el preview devolvía 200 con los montos
   correctos y los nombres vacíos.
2. **El orden de iteración de un mapa en Go es aleatorio.** Un encabezado con dos
   códigos atribuía el monto a un operador distinto en cada corrida. El parser
   recorre `codigosOrdenados`, no el mapa.
3. **`"$ 1.000"` vale 1, no 1000.** El punto es decimal y la coma es de miles.
   Es idéntico al parser TypeScript y es lo correcto para `RawCellValue`, donde
   Excel entrega los montos crudos. No "arreglarlo" sin leer el test.

### Nota del Makefile

Los targets usan `$(PKGS)`, no `./...`: la dependencia npm `flatted` trae una
implementación en Go que `./...` levantaba. En bia-bills no pasa —no hay
`node_modules`— así que al trasvasar da igual.
