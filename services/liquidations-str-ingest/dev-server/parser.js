/* Parser de Insumos STR — portado de App_Liquidaciones/lib/parsers/insumos-str.ts
 * Lógica:
 *   - tipo factu   → hojas BalSTR01, BalSTR02
 *   - tipo refactu → hojas BalSTR01_Ajuste, BalSTR02_Ajuste
 *   - en la columna B (agentes) se ubica la fila "BIAC-BIAE"
 *   - por cada columna cuyo código pertenece a un operador, se toma el valor
 *     de esa fila y se acumula por operador (AIRE = CSID + CSSD)
 *   - amount_payable = Σ factura + Σ refactura
 */
const XLSX = require('xlsx');

// código de columna → código de operador (seed network_operators.str_column_codes)
const HOMOLOGACION = {
  CMMD: 'AFINIA', CSID: 'AIRE', CSSD: 'AIRE', ENID: 'ENELAR', CHCD: 'CHEC',
  CDND: 'CEDENAR', CNSD: 'CENS', ESSD: 'ESSA', CQTD: 'ELECTROCAQUETA',
  HLAD: 'ELECTROHUILA', EMSD: 'EMSA', EBSD: 'EBSA', CASD: 'ENERCA',
  EEPD: 'EEP_PEREIRA', EBPD: 'BAJO_PUTUMAYO', EPSD: 'CELSIA_VALLE',
  EPTD: 'PUTUMAYO', EDQD: 'EDEQ', EGVD: 'ENERGUAVIARE', EDPD: 'DISPAC',
  EPMD: 'EPM', EMID: 'EMCALI', CEOD: 'CEO', ENDD: 'ENEL',
};

const NOMBRES = {
  AFINIA: 'Afinia', AIRE: 'Aire', ENELAR: 'Enelar', CHEC: 'CHEC', CEDENAR: 'Cedenar',
  CENS: 'CENS', ESSA: 'ESSA', ELECTROCAQUETA: 'Electrocaquetá', ELECTROHUILA: 'Electrohuila',
  EMSA: 'EMSA', EBSA: 'EBSA', ENERCA: 'Enerca', EEP_PEREIRA: 'EEP Pereira',
  BAJO_PUTUMAYO: 'Bajo Putumayo', CELSIA_VALLE: 'Celsia Valle', PUTUMAYO: 'Putumayo',
  EDEQ: 'EDEQ', ENERGUAVIARE: 'Energuaviare', DISPAC: 'Dispac', EPM: 'EPM',
  EMCALI: 'Emcali', CEO: 'CEO', ENEL: 'Enel',
};

const MES_MAP = [
  ['enero', 1], ['ene', 1], ['febrero', 2], ['feb', 2], ['marzo', 3], ['mar', 3],
  ['abril', 4], ['abr', 4], ['mayo', 5], ['may', 5], ['junio', 6], ['jun', 6],
  ['julio', 7], ['jul', 7], ['agosto', 8], ['ago', 8], ['septiembre', 9], ['sep', 9],
  ['octubre', 10], ['oct', 10], ['noviembre', 11], ['nov', 11], ['diciembre', 12], ['dic', 12],
];

function detectarMesConsumo(nombre) {
  const l = nombre.toLowerCase();
  for (const [k, n] of MES_MAP) if (l.includes(`-${k}`) || l.includes(`_${k}`)) return n;
  return null;
}

function toNum(v) {
  if (v == null || v === '') return null;
  if (typeof v === 'number') return isNaN(v) ? null : v;
  const s = String(v).replace(/[^0-9.,\-]/g, '').trim();
  if (!s) return null;
  const n = parseFloat(s.replace(/,/g, ''));
  return isNaN(n) ? null : n;
}

function detectarFilaHeader(matrix) {
  const codigos = Object.keys(HOMOLOGACION);
  const maxScan = Math.min(matrix.length, 20);
  let best = 6, bestScore = 0;
  for (let i = 0; i < maxScan; i++) {
    const row = matrix[i] || [];
    let c = 0;
    for (const cell of row) {
      const t = String(cell ?? '').toUpperCase();
      if (codigos.some((code) => t.includes(code))) c++;
    }
    if (c > bestScore) { best = i; bestScore = c; }
  }
  return best;
}

function buscarPestana(wb, nombre) {
  const norm = (s) => s.replace(/[\s_]+/g, '').toUpperCase();
  const target = norm(nombre);
  for (const sn of wb.SheetNames) if (norm(sn) === target) return wb.Sheets[sn] || null;
  return null;
}

function buscarFilaBiac(matrix, desde) {
  for (let i = desde; i < matrix.length; i++) {
    const row = matrix[i] || [];
    for (let col = 0; col <= 3; col++) {
      const t = String(row[col] ?? '').replace(/[–—]/g, '-').replace(/\s+/g, ' ').trim().toUpperCase();
      if (t.includes('BIAC') && t.includes('BIAE')) return i;
    }
  }
  return -1;
}

/**
 * @param {{buffer:Buffer, nombre:string}[]} archivos
 * @param {number} anio  @param {number} mes  (mes de facturación)
 * @returns {{ periodoConsumo:string|null, extraccion:Array, alertas:string[], erroresCriticos:string[] }}
 */
function parsearInsumosSTR(archivos, anio, mes) {
  const extraccion = [];
  const alertas = [];
  const erroresCriticos = [];
  if (!archivos.length) { erroresCriticos.push('No se recibió ningún archivo.'); return { periodoConsumo: null, extraccion, alertas, erroresCriticos }; }

  // mes de consumo del lote (del archivo factu)
  let mesNum = null, source = '';
  for (const { nombre } of archivos) {
    const l = nombre.toLowerCase();
    if (l.includes('tipofactu') && !l.includes('tiporefactu')) { mesNum = detectarMesConsumo(nombre); if (mesNum) { source = nombre; break; } }
  }
  if (mesNum == null) for (const { nombre } of archivos) { mesNum = detectarMesConsumo(nombre); if (mesNum) { source = nombre; break; } }
  if (mesNum == null) { erroresCriticos.push('No se pudo determinar el mes de consumo (ningún archivo tiene mes en el nombre, ej. "-ene").'); return { periodoConsumo: null, extraccion, alertas, erroresCriticos }; }

  const anioConsumo = mesNum > mes ? anio - 1 : anio;
  const periodoConsumo = `${anioConsumo}-${String(mesNum).padStart(2, '0')}`;
  alertas.push(`Mes de consumo del lote: ${periodoConsumo} (detectado de "${source}").`);

  for (const { buffer, nombre } of archivos) {
    const l = nombre.toLowerCase();
    let tipo, pestanas;
    if (l.includes('tiporefactu')) { tipo = 'REFACTURA'; pestanas = ['BalSTR01_Ajuste', 'BalSTR02_Ajuste']; }
    else if (l.includes('tipofactu')) { tipo = 'FACTURA'; pestanas = ['BalSTR01', 'BalSTR02']; }
    else { alertas.push(`[${nombre}] omitido — el nombre no contiene "tipofactu" ni "tiporefactu".`); continue; }

    let wb;
    try { wb = XLSX.read(buffer, { type: 'buffer', cellDates: false }); }
    catch (e) { alertas.push(`[${nombre}] no se pudo leer como Excel: ${e.message}`); continue; }

    const valoresPorOR = {};
    const columnasPorOR = {};
    const diagnosticos = [];

    for (const tab of pestanas) {
      const ws = buscarPestana(wb, tab);
      if (!ws) continue;
      const matrix = XLSX.utils.sheet_to_json(ws, { header: 1, defval: '', raw: true });
      if (!matrix.length) { diagnosticos.push(`hoja "${tab}" vacía`); continue; }

      const filaHeader = detectarFilaHeader(matrix);
      const headers = (matrix[filaHeader] || []).map((h) => String(h ?? '').trim());
      const filaBiac = buscarFilaBiac(matrix, filaHeader + 1);
      if (filaBiac < 0) { diagnosticos.push(`hoja "${tab}": BIAC-BIAE no encontrado (header fila ${filaHeader + 1})`); continue; }
      const biacRow = matrix[filaBiac] || [];

      for (let j = 0; j < headers.length; j++) {
        const hu = (headers[j] || '').toUpperCase();
        for (const [code, orCode] of Object.entries(HOMOLOGACION)) {
          if (hu.includes(code)) {
            const val = toNum(biacRow[j]);
            if (val != null) {
              valoresPorOR[orCode] = (valoresPorOR[orCode] || 0) + val;
              (columnasPorOR[orCode] = columnasPorOR[orCode] || new Set()).add(code);
            }
            break;
          }
        }
      }
    }

    if (!Object.keys(valoresPorOR).length) {
      const d = diagnosticos.length ? ` — ${diagnosticos.join(' | ')}` : '';
      alertas.push(`[${nombre}] no se encontró "BIAC-BIAE" o no hubo valores${d}.`);
      continue;
    }

    for (const [orCode, valor] of Object.entries(valoresPorOR)) {
      extraccion.push({
        orCodigo: orCode,
        orNombre: NOMBRES[orCode] || orCode,
        tipo,
        valor: Math.round(valor * 100) / 100,
        columnas: Array.from(columnasPorOR[orCode] || []),
        archivo: nombre,
      });
    }
  }

  if (!extraccion.length && !erroresCriticos.length) alertas.push('No se generaron registros — revisá los archivos.');
  return { periodoConsumo, extraccion, alertas, erroresCriticos };
}

/** Consolida la extracción por operador: invoice (factu) + reinvoice (refactu) + payable. */
function consolidar(extraccion) {
  const porOR = {};
  for (const r of extraccion) {
    const o = (porOR[r.orCodigo] = porOR[r.orCodigo] || {
      orCodigo: r.orCodigo, orNombre: r.orNombre, invoice_amount: 0, reinvoice_amount: 0,
    });
    if (r.tipo === 'FACTURA') o.invoice_amount += r.valor;
    else o.reinvoice_amount += r.valor;
  }
  const cargos = Object.values(porOR).map((o) => ({
    ...o,
    invoice_amount: Math.round(o.invoice_amount * 100) / 100,
    reinvoice_amount: Math.round(o.reinvoice_amount * 100) / 100,
    amount_payable: Math.round((o.invoice_amount + o.reinvoice_amount) * 100) / 100,
  })).sort((a, b) => a.orNombre.localeCompare(b.orNombre));
  const total = Math.round(cargos.reduce((s, c) => s + c.amount_payable, 0) * 100) / 100;
  return { cargos, total };
}

module.exports = { parsearInsumosSTR, consolidar, HOMOLOGACION, NOMBRES };
