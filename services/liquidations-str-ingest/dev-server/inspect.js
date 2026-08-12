// Análisis de estructura + prueba del parser sobre los BalanceSTR reales.
const fs = require('fs');
const path = require('path');
const XLSX = require('xlsx');
const { parsearInsumosSTR, consolidar, HOMOLOGACION } = require('./parser');

const DIR = 'c:\\Users\\User\\Documents\\GitHub\\olibia-web\\App_Liquidaciones\\archivos_ejemplo\\STR';
const files = fs.readdirSync(DIR).filter((f) => f.toLowerCase().endsWith('.xlsx'));

const codigos = Object.keys(HOMOLOGACION);

for (const name of files) {
  const buf = fs.readFileSync(path.join(DIR, name));
  const wb = XLSX.read(buf, { type: 'buffer', cellDates: false });
  console.log('\n============================================================');
  console.log('ARCHIVO:', name);
  console.log('HOJAS:', wb.SheetNames.join(' | '));

  // Analizar la primera hoja relevante
  const lower = name.toLowerCase();
  const pest = lower.includes('tiporefactu') ? ['BalSTR01_Ajuste', 'BalSTR02_Ajuste'] : ['BalSTR01', 'BalSTR02'];
  const norm = (s) => s.replace(/[\s_]+/g, '').toUpperCase();
  for (const target of pest) {
    const sheetName = wb.SheetNames.find((sn) => norm(sn) === norm(target));
    if (!sheetName) { console.log(`  (hoja ${target}: NO existe)`); continue; }
    const ws = wb.Sheets[sheetName];
    const m = XLSX.utils.sheet_to_json(ws, { header: 1, defval: '', raw: true });
    console.log(`\n  Hoja "${sheetName}"  (${m.length} filas)`);

    // primeras 12 filas, columnas A-F para ver estructura
    for (let i = 0; i < Math.min(m.length, 12); i++) {
      const row = (m[i] || []).slice(0, 8).map((c) => String(c ?? '').slice(0, 14));
      console.log(`    fila ${String(i).padStart(2)}: ${row.join(' | ')}`);
    }

    // detectar header (más códigos) y fila BIAC-BIAE
    let hdr = 6, best = 0;
    for (let i = 0; i < Math.min(m.length, 20); i++) {
      let c = 0;
      for (const cell of m[i] || []) if (codigos.some((cd) => String(cell ?? '').toUpperCase().includes(cd))) c++;
      if (c > best) { best = c; hdr = i; }
    }
    let biac = -1;
    for (let i = hdr + 1; i < m.length; i++) {
      for (let col = 0; col <= 3; col++) {
        const t = String((m[i] || [])[col] ?? '').replace(/[–—]/g, '-').replace(/\s+/g, ' ').trim().toUpperCase();
        if (t.includes('BIAC') && t.includes('BIAE')) { biac = i; break; }
      }
      if (biac > -1) break;
    }
    console.log(`  -> header detectado: fila ${hdr} (${best} códigos)`);
    console.log(`  -> fila BIAC-BIAE:   fila ${biac >= 0 ? biac : 'NO ENCONTRADA'}`);
    if (biac >= 0) {
      const headers = (m[hdr] || []).map((h) => String(h ?? '').trim());
      const found = [];
      for (let j = 0; j < headers.length; j++) {
        const hu = headers[j].toUpperCase();
        for (const cd of codigos) if (hu.includes(cd)) { found.push(`${headers[j]}=${(m[biac] || [])[j]}`); break; }
      }
      console.log(`  -> columnas de operador (${found.length}):`, found.slice(0, 8).join('  '), found.length > 8 ? '…' : '');
    }
  }
}

// ── Prueba del parser por archivo ────────────────────────────────────────
console.log('\n\n================ RESULTADO DEL PARSER (por archivo) ================');
for (const name of files) {
  const anio = name.includes('2025') ? 2025 : 2026;
  const buf = fs.readFileSync(path.join(DIR, name));
  const parsed = parsearInsumosSTR([{ buffer: buf, nombre: name }], anio, 12);
  const { cargos, total } = consolidar(parsed.extraccion);
  console.log(`\n--- ${name}  →  periodo ${parsed.periodoConsumo} ---`);
  if (parsed.erroresCriticos.length) console.log('  ERRORES:', parsed.erroresCriticos);
  if (parsed.alertas.length) parsed.alertas.forEach((a) => console.log('  alerta:', a));
  console.log(`  operadores: ${cargos.length}  |  total: ${total.toLocaleString('es-CO')}`);
  for (const c of cargos.slice(0, 30)) {
    console.log(`    ${c.orCodigo.padEnd(16)} fact=${String(c.invoice_amount).padStart(16)}  refact=${String(c.reinvoice_amount).padStart(12)}  pagar=${String(c.amount_payable).padStart(16)}`);
  }
}
