# Estado — Cargas STR (handoff)

> Última actualización: 2026-08-05. Punto de retome para la próxima sesión.

## Dónde estamos

La fase de **Cargas STR** se divide en dos:

1. **Fase 1 — Cargue de insumos + cálculo del valor a pagar por operador.** ✅ **HECHA Y VALIDADA end-to-end.**
2. **Fase 2 — Conexión con NetSuite para elaborar órdenes de compra.** ⏳ Pendiente.

## Fase 1 — qué quedó funcionando

- **Base de datos** `bia-bi` (RDS dev, PostgreSQL): schema **`liquidations_str`** aplicado, con
  `network_operators` (23 operadores + sus códigos de columna del Excel) y
  `str_charges` (operator_id, consumption_period, invoice_amount, reinvoice_amount,
  `amount_payable` GENERATED = invoice + reinvoice).
  - DDL: [`sql/`](./sql/) (`000_schema.sql`, `001_tables.sql`, `002_seed_operators.sql`).
  - Se aplica con `apply.ps1` / `apply.sh` (psql) o `apply.js` (Node + pg). Credenciales
    en el `.env` raíz de olibia-web (`DB_*`).
- **Extracción** (parser fiel a `App_Liquidaciones/lib/parsers/insumos-str.ts`):
  factu → hojas `BalSTR01`/`BalSTR02`; refactu → `BalSTR01_Ajuste`/`BalSTR02_Ajuste`;
  fila `BIAC-BIAE` en la columna B; homologación de columnas por operador
  (`AIRE = CSID + CSSD`). Código en [`dev-server/parser.js`](./dev-server/parser.js).
- **Cálculo**: `valor a pagar = Σ factura + Σ refactura`. Validado con archivos reales:
  **CHEC = 70.812.140 − 906 − 3.536 = 70.807.698** (subiendo factu + 2 refactu juntos).
- **UI**: se prueba desde el módulo real de olibia-web (no se tocó el front). El
  dev-server implementa los endpoints que la UI ya llama (`/api/cargas/preview`,
  `/api/cargas/confirmar`, + stubs) y se conecta vía `.env.local`
  (`NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL=http://localhost:4000`).

## Cómo levantarlo para probar

```bash
# Terminal 1 — backend local STR
cd services/liquidations-str-ingest/dev-server
npm install        # una vez
npm start          # http://localhost:4000  (DEBE estar corriendo)

# Terminal 2 — olibia-web
npm run dev        # reiniciar si .env.local se creó después
```
UI: login → **Finance → Liquidaciones → Cargas → Nueva carga → Insumos STR** →
subir los `BalanceSTR*.xlsx` (factu + refactu del settlement) → preview → confirmar.

- Archivos reales de ejemplo: `App_Liquidaciones/archivos_ejemplo/STR/`.
- Utilidad de análisis de estructura: `dev-server/inspect.js`.
- ⚠️ Si la UI da "Internal Server Error" → el dev-server (:4000) no está corriendo.

## Reglas de negocio confirmadas

- El **valor a pagar** de un operador en una liquidación = factura del mes en curso
  **+ todas las refacturas** (ajustes de meses previos) pendientes. El lote se
  etiqueta con el mes del archivo **factu**.
- Los montos pueden ser **negativos** (ajustes). Precisión `NUMERIC(18,2)`.

## Fase 2 — pendiente (próximos pasos)

1. Cargar `netsuite_vendor_id` de cada operador en `network_operators`
   (hoy están en NULL; los completa Finanzas).
2. Tomar los `str_charges` de un período y generar las **Purchase Orders** en NetSuite
   (mapeo: `netsuite_vendor_id` → entity, `consumption_period-01` → tranDate,
   `amount_payable.toFixed(2)` → rate). Ver `docs/PROCESO.md` §NetSuite.
3. (Opcional) empaquetar el dev-server como Lambda real según `docs/ARCHITECTURE.md`.

## Notas / gotchas

- El clasificador de seguridad del harness bloquea conexiones a BD/red hasta que el
  usuario aprueba explícitamente.
- No hay `psql` en la máquina; para correr SQL directo se usa Node + `pg`.
- `.env.local` (override local) y `.env` están gitignored — no commitear credenciales.
