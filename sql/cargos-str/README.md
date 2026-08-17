# Cargos STR — DDL de las dos bases de BIA

Reemplaza al paquete que apuntaba a `bia-bi` (`services/liquidations-str-ingest/sql/`).
Esa base se deja de usar: el módulo pasa a las dos bases de BIA en el RDS de dev
(`c4-rds-bia-dev`), **`file-compiler`** y **`calculator-prices`**.

Los identificadores de base van en **inglés**, como el resto de las tablas de
esas bases (`agents`, `operators`, `operator_rates`). El código de la aplicación
sigue en español y `lib/cargos-str.ts` traduce entre los dos mundos.

## Qué queda dónde

```
Excel BalanceSTR*.xlsx
        │        ▲
        │        └── public.agents (catálogo XM, ya existente): nombres legales
        │
        ├─▶ file-compiler
        │     public.liquidations_str_inputs      insumo crudo, sin sumar
        │
        └─▶ calculator-prices
              public.liquidations_str_charges     valor a pagar
                                                          │
                                                          └─▶ NetSuite (fase 2)
```

Las dos comparten `load_id`, así se traza cualquier valor a pagar hasta el
insumo que lo produjo.

## Las dos tablas

| Base | Tabla | Qué guarda |
|------|-------|-----------|
| file-compiler | `liquidations_str_inputs` | `load_id`, `period`, `operator_code`, `invoice_amount` y hasta tres `reinvoice_N_amount` |
| calculator-prices | `liquidations_str_charges` | `load_id`, `period`, `operator_code`, `operator_name`, `amount_payable` |

**Una sola tabla por base**: cada una guarda lo vigente *y* el histórico. No hay
vistas ni tablas auxiliares.

**No se creó ningún catálogo de operadores**: la homologación se resuelve contra
`public.agents`, que ya existe en file-compiler.

## Dos decisiones de diseño

**Append-only.** Nada se reemplaza ni se borra. Cada cargue inserta filas nuevas
con su `load_id` y su `created_at`; recargar un período deja el anterior como
historial. Por eso no hay `updated_at` y la unicidad es `(load_id, operator_code)`
y no `(period, operator_code)`.

Como consecuencia, **toda lectura debe quedarse con el registro más reciente de
cada (`period`, `operator_code`)**. Esa regla se escribe una sola vez, en la
constante `VIGENTES` de [`lib/cargos-str.ts`](../../lib/cargos-str.ts), y todos
los endpoints pasan por ese módulo. Una consulta que fuera directo a la tabla sin
aplicarla sumaría cargas viejas con nuevas y **duplicaría los montos** — con
cifras de mil millones, el error no se nota a simple vista. Si algún día hace
falta consultar estas tablas desde afuera de la app, conviene envolver esa regla
en una vista antes de hacerlo.

**La homologación se resuelve contra `public.agents`, sin tabla propia.** Las 24
abreviaturas que aparecen como encabezados en los archivos BalanceSTR existen las
24 en el catálogo de agentes de XM, con su nombre legal — incluidos DISPAC,
ENERGUAVIARE y PUTUMAYO, que **no** están en `public.operators` de
calculator-prices.

| Dato | Dónde vive |
|---|---|
| Abreviatura → agente, y su nombre legal | `public.agents` |
| Qué agentes son operadores de red del negocio, y cómo se agrupan (`CSID` + `CSSD` → AIRE) | el diccionario `HOMOLOGACION_STR` de `lib/parsers/insumos-str.ts` |
| Nombre a mostrar de un OR | `agents.name` del agente del grupo con `activity = 'OPERADOR DE RED'` — verificado: hay exactamente uno por operador en los 23 |

La agrupación no es deducible de `agents`: `CSID` y `CSSD` **no comparten NIT**
(el de CSID está cargado literalmente como `"undefined"`), así que se mantiene en
código. Cambiar un operador requiere deploy — es el costo de no tener tabla de
configuración, asumido a propósito.

`netsuite_vendor_id` no tiene dónde vivir todavía: se define en la fase 2, junto
con el resto de la integración con NetSuite.

## Cómo aplicarlos

```bash
node sql/cargos-str/apply.js all --dry-run     # muestra qué correría, sin conectarse
node sql/cargos-str/apply.js file-compiler
node sql/cargos-str/apply.js calculator-prices
node sql/cargos-str/apply.js --list-dbs        # bases disponibles en el servidor
```

Los scripts son idempotentes (`IF NOT EXISTS`): correrlos dos veces deja el mismo
estado. Al terminar, `apply.js` verifica que las 24 abreviaturas del parser sigan
existiendo en `public.agents` — si XM diera de baja un código, el parser dejaría
de encontrar esa columna y conviene enterarse acá.

Credenciales por entorno o desde cualquiera de los dos `.env` del workspace
(ambos gitignored). Estas bases usan un usuario distinto al de `bia-bi`:

```
DB_HOST / DB_PORT          el RDS de dev
DB_USER2 / DB_PASSWORD2    usuario con acceso a file-compiler y calculator-prices
```

El driver `pg` es dependencia del repo desde la migración; si faltara, el runner
reusa la copia del dev-server STR.

## Por qué en `public` y sin schema propio

El usuario `liquidaciones_dev` **no tiene permiso para crear schemas** en esas
bases (`CREATE ON DATABASE` denegado), pero sí para crear tablas en `public`. De
ahí el prefijo `liquidations_` en los nombres.

No es una decisión que quede clavada. Cuando concedan el permiso:

```sql
GRANT CREATE ON DATABASE "file-compiler"     TO liquidaciones_dev;
GRANT CREATE ON DATABASE "calculator-prices" TO liquidaciones_dev;
```

mover las tablas es instantáneo y no pierde datos:

```sql
CREATE SCHEMA liquidations;
ALTER TABLE public.liquidations_str_inputs SET SCHEMA liquidations;
```

## Límite conocido

Hay **tres columnas de ajuste** porque el caso real trae hasta tres archivos de
refactura por lote. Si algún mes llega un cuarto, el preview falla con un mensaje
explícito — nunca lo descarta en silencio. Ampliar implica agregar la columna acá
y su manejo en el confirmar.
