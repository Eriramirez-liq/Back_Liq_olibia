# Liquidaciones — Cargos STR (Lambda)

Diseño del servicio serverless que ingiere los archivos `BalanceSTR*.xlsx` del
módulo **Cargas STR**, extrae la información y deja en la BD el valor a pagar por
operador y periodo. Ese resultado es el que se envía a **NetSuite** para crear
las órdenes de compra.

```
Excel BalanceSTR ──▶ extracción (parser) ──▶ str_charges ──▶ NetSuite (OC)
```

> **Alcance de este entregable:** solo **SQL**, **documentación** y la
> **estructura de carpetas**. Sin infra ni código de implementación.

## Contenido

| Ruta | Qué es |
|---|---|
| [`sql/`](./sql/) | DDL ejecutable + esquema DBML |
| [`docs/DATABASE.md`](./docs/DATABASE.md) | Diagrama ER + tablas |
| [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md) | Flujo del Lambda + estructura de carpetas |
| [`docs/PROCESO.md`](./docs/PROCESO.md) | **Proceso end-to-end: los 4 endpoints (cargue → extracción → procesamiento → resultado)** |

## SQL

Schema **`liquidations_str`** dentro de la base **BIA-BI** (PostgreSQL 13+).

```
sql/000_schema.sql            # CREATE SCHEMA liquidations_str
sql/001_tables.sql            # network_operators + str_charges (+ relación e índices)
sql/002_seed_operators.sql    # operadores STR + sus códigos de columna del Excel
sql/schema.dbml               # mismo esquema en DBML → pegar en dbdiagram.io
```

### Aplicar el schema a la base

Las credenciales van por variable de entorno / archivo `.env` (nunca en el repo).

```bash
cp .env.example .env          # completar DATABASE_URL con las credenciales
./apply.ps1                   # Windows (PowerShell)
./apply.sh                    # macOS / Linux
```

El script corre `000 → 001 → 002` con `psql` (requiere el cliente de PostgreSQL
en el PATH) y `ON_ERROR_STOP=1`. Es **idempotente**: `CREATE ... IF NOT EXISTS`
y `ON CONFLICT`, así que se puede re-ejecutar sin romper nada. La base **BIA-BI
debe existir** (el script crea el schema y las tablas, no la base).

## Modelo de datos

Dos tablas:

- **`str_charges`** — el resultado a pagar. Una fila por **(operador, periodo de
  consumo)** con el **valor exacto a pagar**. Es lo que viaja a NetSuite.
  `UNIQUE(operator_id, consumption_period)` → re-cargar un periodo hace UPSERT.
- **`network_operators`** — catálogo. Aporta `str_column_codes` (columnas del Excel de
  cada operador, para la extracción) y `netsuite_vendor_id` (proveedor de la OC).

```
network_operators ──< str_charges
```

Detalle y decisiones en [`docs/DATABASE.md`](./docs/DATABASE.md).
