/* ============================================================================
 *  Liquidations — STR Charges
 *  apply.js · Crea el schema `liquidations_str` y sus tablas en Postgres (bia-bi).
 * ----------------------------------------------------------------------------
 *  Alternativa a apply.sh/apply.ps1 para entornos sin `psql`.
 *  Requisito: driver pg  →  `npm install pg`
 *
 *  Credenciales (en este orden de prioridad):
 *    1. process.env.DATABASE_URL
 *    2. process.env.DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME
 *    3. el archivo .env raíz de olibia-web (../../.env) — DB_* o DATABASE_URL
 *
 *  Uso:
 *    cd services/liquidations-str-ingest
 *    npm install pg
 *    node apply.js
 * ========================================================================== */
const fs = require('fs');
const path = require('path');
const { Client } = require('pg');

const SQL_DIR = path.join(__dirname, 'sql');
const FILES = ['000_schema.sql', '001_tables.sql', '002_seed_operators.sql'];
// .env raíz de olibia-web (services/liquidations-str-ingest → repo root)
const ROOT_ENV = path.join(__dirname, '..', '..', '.env');

function parseEnv(file) {
  const out = {};
  if (!fs.existsSync(file)) return out;
  for (const raw of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    const i = line.indexOf('=');
    if (i === -1) continue;
    out[line.slice(0, i).trim()] = line.slice(i + 1).trim();
  }
  return out;
}

function resolveConfig() {
  const fileEnv = parseEnv(ROOT_ENV);
  const get = (k) => process.env[k] || fileEnv[k];

  const url = get('DATABASE_URL');
  if (url) return { connectionString: url, ssl: sslFor(url) };

  const host = get('DB_HOST');
  if (!host) {
    throw new Error(
      'No hay credenciales. Definí DATABASE_URL o DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME ' +
      '(en el entorno o en el .env raíz de olibia-web).'
    );
  }
  return {
    host,
    port: Number(get('DB_PORT') || 5432),
    user: get('DB_USER'),
    password: get('DB_PASSWORD'),
    database: get('DB_NAME'),
    ssl: host === 'localhost' || host === '127.0.0.1' ? false : { rejectUnauthorized: false },
    connectionTimeoutMillis: 15000,
  };
}

function sslFor(url) {
  return /localhost|127\.0\.0\.1/.test(url) ? false : { rejectUnauthorized: false };
}

(async () => {
  const cfg = resolveConfig();
  const client = new Client(cfg);
  const target = cfg.connectionString
    ? cfg.connectionString.replace(/:\/\/([^:]+):[^@]+@/, '://$1:***@')
    : `${cfg.host}/${cfg.database} (user ${cfg.user})`;

  console.log(`Conectando a ${target} ...`);
  await client.connect();
  console.log('Conectado.\n');

  for (const f of FILES) {
    process.stdout.write(`  -> ${f} ... `);
    await client.query(fs.readFileSync(path.join(SQL_DIR, f), 'utf8'));
    console.log('OK');
  }

  const tables = await client.query(
    `SELECT table_name FROM information_schema.tables
     WHERE table_schema = 'liquidations_str' ORDER BY table_name`
  );
  const ops = await client.query(
    'SELECT count(*)::int AS n FROM liquidations_str.network_operators'
  );

  console.log('\n=== Verificación ===');
  console.log('Tablas en liquidations_str:', tables.rows.map((r) => r.table_name).join(', ') || '(ninguna)');
  console.log('Operadores sembrados:', ops.rows[0].n);

  await client.end();
  console.log('\nOK - schema aplicado correctamente.');
})().catch((e) => {
  console.error('\nERROR:', e.message);
  process.exit(1);
});
