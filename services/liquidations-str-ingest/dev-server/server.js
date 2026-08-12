/* Backend LOCAL de prueba — Fase 1: cargue de insumos STR.
 *
 * Implementa los endpoints que ya llama la UI de Cargas de olibia-web
 * (App_Liquidaciones), para probar en local subir los BalanceSTR y que se lean.
 * La UI llega acá vía el proxy /api/liquidations-proxy → NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL.
 *
 *   POST /api/cargas/preview          multipart (files[], anio, mes, tipoFuente) → PreviewResponse
 *   POST /api/cargas/confirmar        { meta, filasCompletas } → { cargaId, totalGuardados }  (UPSERT str_charges)
 *   GET  /api/cargas/estado-periodo   → estado del período (stub)
 *   GET  /api/cargas                  → historial (stub vacío)
 *   GET  /api/operadores              → operadores desde bia-bi (o stub)
 *   GET  /api/periodos                → períodos (stub)
 *   GET  /health
 *
 * Solo INSUMOS_STR está implementado de verdad; el resto son stubs para que la
 * vista de Cargas renderice sin errores. DB opcional (preview funciona sin ella).
 */
const path = require('path');
const fs = require('fs');
const express = require('express');
const multer = require('multer');
const { parsearInsumosSTR, consolidar } = require('./parser');

const PORT = Number(process.env.PORT || 4000);
const upload = multer({ storage: multer.memoryStorage(), limits: { fileSize: 30 * 1024 * 1024 } });
const app = express();
app.use(express.json({ limit: '10mb' }));

// ── DB opcional (bia-bi) ──────────────────────────────────────────────────
function loadRootEnv() {
  const p = path.join(__dirname, '..', '..', '..', '.env'); // olibia-web/.env
  const out = {};
  if (!fs.existsSync(p)) return out;
  for (const raw of fs.readFileSync(p, 'utf8').split(/\r?\n/)) {
    const l = raw.trim(); if (!l || l.startsWith('#')) continue;
    const i = l.indexOf('='); if (i > -1) out[l.slice(0, i).trim()] = l.slice(i + 1).trim();
  }
  return out;
}
let pool = null;
let dbTried = false;
function getPool() {
  if (dbTried) return pool;
  dbTried = true;
  const e = { ...loadRootEnv(), ...process.env };
  if (!e.DB_HOST && !e.DATABASE_URL) return (pool = null);
  try {
    const { Pool } = require('pg');
    pool = e.DATABASE_URL
      ? new Pool({ connectionString: e.DATABASE_URL, ssl: { rejectUnauthorized: false } })
      : new Pool({ host: e.DB_HOST, port: Number(e.DB_PORT || 5432), user: e.DB_USER, password: e.DB_PASSWORD, database: e.DB_NAME, ssl: e.DB_HOST === 'localhost' ? false : { rejectUnauthorized: false } });
  } catch { pool = null; }
  return pool;
}

// ── Extracción + consolidación (núcleo Fase 1) ────────────────────────────
function extraerStr(files, anio, mes) {
  const archivos = (files || []).map((f) => ({ buffer: f.buffer, nombre: f.originalname }));
  const parsed = parsearInsumosSTR(archivos, anio, mes);
  const { cargos, total } = consolidar(parsed.extraccion);
  return { ...parsed, cargos, totalValor: total };
}

// ── POST /api/cargas/preview ──────────────────────────────────────────────
app.post('/api/cargas/preview', upload.any(), (req, res) => {
  try {
    const anio = Number(req.body.anio);
    const mes = Number(req.body.mes);
    const tipoFuente = req.body.tipoFuente;
    const files = req.files || [];

    if (tipoFuente !== 'INSUMOS_STR') {
      return res.status(400).json({ error: `Este backend local solo implementa INSUMOS_STR (recibido: ${tipoFuente}).` });
    }
    if (!files.length) return res.status(400).json({ error: 'Insumos STR requiere al menos un archivo.' });
    if (!anio || !mes) return res.status(400).json({ error: 'anio y mes son requeridos.' });

    const r = extraerStr(files, anio, mes);

    // preview: filas legibles (las claves son las columnas de la tabla en la UI)
    const preview = r.cargos.map((c) => ({
      Operador: c.orNombre,
      Código: c.orCodigo,
      'Factura (COP)': c.invoice_amount,
      'Refactura (COP)': c.reinvoice_amount,
      'A pagar (COP)': c.amount_payable
    }));

    // filasCompletas: lo que /confirmar necesita para persistir (incluye el período detectado)
    const filasCompletas = r.cargos.map((c) => ({
      orCodigo: c.orCodigo,
      invoice_amount: c.invoice_amount,
      reinvoice_amount: c.reinvoice_amount,
      periodoConsumo: r.periodoConsumo
    }));

    res.json({
      preview,
      filasCompletas,
      total: r.cargos.length,
      alertas: r.alertas,
      erroresCriticos: r.erroresCriticos,
      existeCargaPrevia: false
    });
  } catch (e) {
    console.error('[preview] error:', e);
    res.status(500).json({ error: `Error al procesar los archivos: ${e.message}` });
  }
});

// ── POST /api/cargas/confirmar ────────────────────────────────────────────
app.post('/api/cargas/confirmar', async (req, res) => {
  try {
    const { filasCompletas } = req.body || {};
    const filas = Array.isArray(filasCompletas) ? filasCompletas : [];
    const p = getPool();
    if (!p) return res.status(503).json({ error: 'Sin conexión a bia-bi — no se pudo guardar (preview sí funciona).' });
    if (!filas.length) return res.status(400).json({ error: 'No hay filas para guardar.' });

    const client = await p.connect();
    let totalGuardados = 0;
    try {
      await client.query('BEGIN');
      for (const f of filas) {
        const op = await client.query('SELECT id FROM liquidations_str.network_operators WHERE code=$1', [f.orCodigo]);
        if (!op.rowCount) continue;
        await client.query(
          `INSERT INTO liquidations_str.str_charges (operator_id, consumption_period, invoice_amount, reinvoice_amount)
           VALUES ($1,$2,$3,$4)
           ON CONFLICT (operator_id, consumption_period)
           DO UPDATE SET invoice_amount=EXCLUDED.invoice_amount, reinvoice_amount=EXCLUDED.reinvoice_amount, updated_at=now()`,
          [op.rows[0].id, f.periodoConsumo, f.invoice_amount, f.reinvoice_amount]
        );
        totalGuardados++;
      }
      await client.query('COMMIT');
    } catch (e) { await client.query('ROLLBACK'); throw e; }
    finally { client.release(); }

    res.json({ cargaId: `local-${Date.now()}`, totalGuardados });
  } catch (e) { res.status(500).json({ error: e.message }); }
});

// ── Stubs para que la vista de Cargas renderice ───────────────────────────
app.get('/api/cargas/estado-periodo', (_req, res) => {
  res.json({ facturacion: { estado: 'pendiente' }, xm: { estado: 'pendiente' }, sdl: [], tc1: [], balance: [] });
});

app.get('/api/cargas', (_req, res) => {
  res.json({ cargas: [], total: 0, page: 1, pageSize: 20 });
});

app.get('/api/periodos', (_req, res) => res.json([]));

app.get('/api/operadores', async (_req, res) => {
  try {
    const p = getPool();
    if (!p) return res.json([]);
    const q = await p.query('SELECT id, code, name, netsuite_vendor_id, active FROM liquidations_str.network_operators ORDER BY name');
    res.json(q.rows.map((o) => ({ id: o.id, codigo: o.code, nombre: o.name, activo: o.active, netsuite_vendor_id: o.netsuite_vendor_id })));
  } catch { res.json([]); }
});

// Endpoint directo para consultar lo guardado (debug).
app.get('/api/str/cargos', async (req, res) => {
  try {
    const p = getPool();
    if (!p) return res.status(503).json({ error: 'Sin DB.' });
    const periodo = req.query.periodo;
    const q = await p.query(
      `SELECT o.code, o.name, c.invoice_amount, c.reinvoice_amount, c.amount_payable
       FROM liquidations_str.str_charges c JOIN liquidations_str.network_operators o ON o.id=c.operator_id
       ${periodo ? 'WHERE c.consumption_period=$1' : ''} ORDER BY o.name`,
      periodo ? [periodo] : []
    );
    res.json({ periodo: periodo || 'todos', operadores: q.rows });
  } catch (e) { res.status(500).json({ error: e.message }); }
});

app.get('/health', (_req, res) => res.json({ ok: true, db: !!getPool() }));

app.listen(PORT, () => {
  console.log(`\n  STR backend local en  http://localhost:${PORT}`);
  console.log(`  DB bia-bi:            ${getPool() ? 'conectada' : 'no (preview igual funciona)'}`);
  console.log(`\n  Apuntá olibia-web con:  NEXT_PUBLIC_LIQUIDATIONS_BACKEND_URL=http://localhost:${PORT}\n`);
});
