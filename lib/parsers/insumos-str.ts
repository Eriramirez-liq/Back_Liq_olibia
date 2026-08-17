import * as XLSX from "xlsx"
import { ResultadoParser } from "@/lib/parsers/types"

/**
 * Una fila por OPERADOR DE RED — no por archivo.
 *
 * Es la misma forma que tiene la tabla `liquidaciones_cargos_str` de
 * file-compiler, así lo que el usuario valida en pantalla es exactamente lo que
 * queda guardado.
 *
 * `or_nombre` no lo resuelve el parser: sale del catálogo de agentes de XM
 * (`public.agents` en file-compiler) y lo completa el endpoint. Así el parser
 * sigue siendo una función pura, testeable sin base de datos.
 */
export type FilaSTR = {
  or_codigo:         string
  or_nombre?:        string
  periodo:           string        // "AAAA-MM" — el del filtro de Nueva carga
  valor_factura:     number
  refactura_1_valor: number | null // ajuste más antiguo del lote
  refactura_2_valor: number | null
  refactura_3_valor: number | null // ajuste más reciente
  valor_a_pagar:     number        // factura + ajustes
  detalle?:          Record<string, unknown>
}

/** Tope de archivos de ajuste por lote — es el ancho de la tabla. */
export const MAX_REFACTURAS = 3

// ─── Homologación de códigos de columna → código de operador ────────────────
// Las columnas de los archivos BalSTR traen códigos de agente de XM ("CMMD",
// "CSID"…). Los mismos códigos existen en `public.agents` de file-compiler, con
// su nombre legal — de ahí sale `or_nombre`.
//
// Lo que este diccionario agrega, y `agents` no puede saber, es CUÁLES agentes
// son operadores de red del negocio y CÓMO SE AGRUPAN: AIRE llega en dos
// columnas (CSID y CSSD) que se suman, y esos dos agentes ni siquiera comparten
// NIT en el catálogo. Por eso vive acá y no en base.
export const HOMOLOGACION_STR: Record<string, string> = {
  CMMD: "AFINIA",
  CSID: "AIRE",
  CSSD: "AIRE",            // mismo OR (se suman ambas columnas)
  ENID: "ENELAR",
  CHCD: "CHEC",
  CDND: "CEDENAR",
  CNSD: "CENS",
  ESSD: "ESSA",
  CQTD: "ELECTROCAQUETA",
  HLAD: "ELECTROHUILA",
  EMSD: "EMSA",
  EBSD: "EBSA",
  CASD: "ENERCA",
  EEPD: "EEP_PEREIRA",
  EBPD: "BAJO_PUTUMAYO",
  EPSD: "CELSIA_VALLE",
  EPTD: "PUTUMAYO",
  EDQD: "EDEQ",
  EGVD: "ENERGUAVIARE",
  EDPD: "DISPAC",
  EPMD: "EPM",
  EMID: "EMCALI",
  CEOD: "CEO",
  ENDD: "ENEL",
}

/** Códigos de agente que el parser busca en los encabezados. */
export const CODIGOS_AGENTE_STR = Object.keys(HOMOLOGACION_STR)

// ─── Diccionario mes en nombre → número (1-12) ─────────────────────────────
// Ya NO define el período de la carga —ese sale del filtro— pero sí sirve para
// ORDENAR los archivos de ajuste del más viejo al más nuevo.
const MES_MAP: Array<[string, number]> = [
  ["enero", 1], ["ene", 1],
  ["febrero", 2], ["feb", 2],
  ["marzo", 3], ["mar", 3],
  ["abril", 4], ["abr", 4],
  ["mayo", 5], ["may", 5],
  ["junio", 6], ["jun", 6],
  ["julio", 7], ["jul", 7],
  ["agosto", 8], ["ago", 8],
  ["septiembre", 9], ["sep", 9],
  ["octubre", 10], ["oct", 10],
  ["noviembre", 11], ["nov", 11],
  ["diciembre", 12], ["dic", 12],
]

/**
 * Clave de orden de un archivo de ajuste, a partir de su nombre.
 * "BalanceSTRTipoReFactu2025-NOV-2.xlsx" → 202511.
 * Si el nombre no trae mes o año reconocible, devuelve Infinity para que quede
 * al final; el desempate final es alfabético, así el orden es reproducible.
 */
function ordenDeAjuste(nombreArchivo: string): number {
  const lower = nombreArchivo.toLowerCase()

  let mes: number | null = null
  for (const [clave, num] of MES_MAP) {
    if (lower.includes(`-${clave}`) || lower.includes(`_${clave}`)) { mes = num; break }
  }
  if (mes == null) return Number.POSITIVE_INFINITY

  const anio = lower.match(/(20\d{2})/)
  if (!anio) return Number.POSITIVE_INFINITY

  return Number(anio[1]) * 100 + mes
}

function toNum(v: unknown): number | null {
  if (v == null || v === "") return null
  if (typeof v === "number") return isNaN(v) ? null : v
  const s = String(v).replace(/[^0-9.,\-]/g, "").trim()
  if (!s) return null
  // Quitar comas como separador de miles
  const n = parseFloat(s.replace(/,/g, ""))
  return isNaN(n) ? null : n
}

// Auto-detecta la fila del header eligiendo aquella con la MAYOR cantidad
// de celdas individuales que contienen códigos de operador.
//
// Esto evita el falso positivo donde una fila descriptiva con todos los
// códigos en una sola celda (ej. "Reporte ... CMMD CSID CSSD") era elegida
// en vez del header real donde cada código está en una celda separada.
function detectarFilaHeader(matrix: (string | number)[][]): number {
  const codigos = Object.keys(HOMOLOGACION_STR)
  const maxScan = Math.min(matrix.length, 20)
  let best = 6  // default: equivalente a Python header=6
  let bestScore = 0
  for (let i = 0; i < maxScan; i++) {
    const row = matrix[i] ?? []
    let cellsWithCode = 0
    for (const cell of row) {
      const text = String(cell ?? "").toUpperCase()
      if (codigos.some(c => text.includes(c))) cellsWithCode++
    }
    if (cellsWithCode > bestScore) {
      best = i
      bestScore = cellsWithCode
    }
  }
  return best
}

// Busca una pestaña por nombre tolerando diferencias de espacios,
// underscores y mayúsculas (ej. "BalSTR01_Ajuste" ≡ "BalSTR01 Ajuste").
function buscarPestana(wb: XLSX.WorkBook, nombre: string): XLSX.WorkSheet | null {
  const norm = (s: string) => s.replace(/[\s_]+/g, "").toUpperCase()
  const target = norm(nombre)
  for (const sheetName of wb.SheetNames) {
    if (norm(sheetName) === target) return wb.Sheets[sheetName] ?? null
  }
  return null
}

// Búsqueda flexible de la fila BIAC-BIAE:
//   - Revisa las primeras 4 columnas (no solo la B)
//   - Normaliza espacios, mayúsculas y cualquier tipo de guión (- – —)
//   - Sólo requiere que el texto contenga "BIAC" y "BIAE"
function buscarFilaBiac(matrix: (string | number)[][], desde: number): number {
  for (let i = desde; i < matrix.length; i++) {
    const row = matrix[i] ?? []
    for (let col = 0; col <= 3; col++) {
      const text = String(row[col] ?? "")
        .replace(/[–—]/g, "-")
        .replace(/\s+/g, " ")
        .trim()
        .toUpperCase()
      if (text.includes("BIAC") && text.includes("BIAE")) {
        return i
      }
    }
  }
  return -1
}

/**
 * Extrae los valores por operador de UN archivo, sumando las columnas que
 * homologan al mismo OR (AIRE = CSID + CSSD).
 */
function extraerValoresPorOR(
  buffer: Buffer,
  nombre: string,
  pestanas: string[],
  alertas: string[],
): Record<string, number> | null {
  const valoresPorOR: Record<string, number> = {}

  let wb: XLSX.WorkBook
  try {
    wb = XLSX.read(buffer, { type: "buffer", cellDates: false })
  } catch (e) {
    alertas.push(`[${nombre}] no se pudo leer como Excel: ${e}`)
    return null
  }

  // Acumular diagnósticos por pestaña (para reportar si no se encontró BIAC-BIAE)
  const diagnosticos: string[] = []

  for (const tabName of pestanas) {
    const ws = buscarPestana(wb, tabName)
    if (!ws) continue  // pestaña no existe — silencioso

    const matrix = XLSX.utils.sheet_to_json<unknown[]>(ws, {
      header: 1,
      defval: "",
      raw: true,
    }) as unknown as (string | number)[][]

    if (matrix.length === 0) {
      diagnosticos.push(`pestaña "${tabName}" vacía`)
      continue
    }

    const filaHeader = detectarFilaHeader(matrix)
    const headers = (matrix[filaHeader] ?? []).map(h => String(h ?? "").trim())

    const filaBiacIdx = buscarFilaBiac(matrix, filaHeader + 1)
    if (filaBiacIdx < 0) {
      const sample: string[] = []
      for (let i = filaHeader + 1; i < Math.min(matrix.length, filaHeader + 8); i++) {
        const cellA = String(matrix[i]?.[0] ?? "").trim()
        const cellB = String(matrix[i]?.[1] ?? "").trim()
        const combined = [cellA, cellB].filter(Boolean).join(" | ")
        if (combined) sample.push(combined)
      }
      diagnosticos.push(
        `pestaña "${tabName}": BIAC-BIAE no encontrado (header detectado fila ${filaHeader + 1}). ` +
        `Primeras filas: ${sample.slice(0, 3).join("  ·  ") || "(vacías)"}`,
      )
      continue
    }
    const biacRow = matrix[filaBiacIdx] ?? []

    // Match de códigos en headers — case-insensitive vía toUpperCase
    for (let j = 0; j < headers.length; j++) {
      const headerUpper = (headers[j] ?? "").toUpperCase()
      for (const [codigo, orCodigo] of Object.entries(HOMOLOGACION_STR)) {
        if (headerUpper.includes(codigo)) {
          const val = toNum(biacRow[j])
          if (val != null) {
            valoresPorOR[orCodigo] = (valoresPorOR[orCodigo] ?? 0) + val
          }
          break
        }
      }
    }
  }

  if (Object.keys(valoresPorOR).length === 0) {
    const diag = diagnosticos.length > 0 ? ` — ${diagnosticos.join(" | ")}` : ""
    alertas.push(`[${nombre}] no se encontró fila "BIAC - BIAE" o no hubo valores para los operadores${diag}.`)
    return null
  }

  return valoresPorOR
}

/**
 * Parsea uno o varios archivos de Insumos STR (BalanceSTR*.xlsx).
 *
 * Devuelve UNA FILA POR OPERADOR, con el valor de la factura y cada ajuste en su
 * propia columna — la misma forma que la tabla destino.
 *
 * Algoritmo:
 *   1. El período es el que llega por parámetro (el del filtro de Nueva carga).
 *      Se aplica a TODOS los archivos del lote, sin importar qué mes diga el
 *      nombre de un archivo de refactura.
 *   2. Clasificar los archivos: "tipofactu" es la factura, "tiporefactu" son los
 *      ajustes, ordenados por el mes de su nombre (del más viejo al más nuevo).
 *   3. Por cada archivo, leer las pestañas que le corresponden, ubicar la fila
 *      "BIAC - BIAE" y acumular por operador homologando las columnas.
 *   4. Pivotar: una fila por operador con factura, ajustes y total a pagar.
 */
export async function parsearInsumosSTR(
  buffers: { buffer: Buffer; nombre: string }[],
  anio: number,
  mes: number,
): Promise<ResultadoParser<FilaSTR>> {
  const filas: FilaSTR[]          = []
  const alertas: string[]         = []
  const erroresCriticos: string[] = []

  if (buffers.length === 0) {
    erroresCriticos.push("No se recibió ningún archivo.")
    return { filas, alertas, erroresCriticos }
  }

  // ── PASO 1: El período sale del filtro ──────────────────────────────────
  const periodo = `${anio}-${String(mes).padStart(2, "0")}`
  alertas.push(`Período de liquidación: ${periodo} (seleccionado en Nueva carga).`)

  // ── PASO 2: Clasificar y ordenar los archivos ───────────────────────────
  const facturas: { buffer: Buffer; nombre: string }[] = []
  const ajustes:  { buffer: Buffer; nombre: string }[] = []

  for (const archivo of buffers) {
    const lower = archivo.nombre.toLowerCase()
    if (lower.includes("tiporefactu")) ajustes.push(archivo)
    else if (lower.includes("tipofactu")) facturas.push(archivo)
    else alertas.push(`[${archivo.nombre}] omitido — el nombre no contiene "tipofactu" ni "tiporefactu".`)
  }

  // Orden reproducible: por el mes del nombre, y alfabético para desempatar.
  ajustes.sort((a, b) => {
    const d = ordenDeAjuste(a.nombre) - ordenDeAjuste(b.nombre)
    return d !== 0 ? d : a.nombre.localeCompare(b.nombre)
  })

  // El ancho de la tabla destino es fijo. Un cuarto ajuste NO se descarta en
  // silencio: se corta la carga para que nadie liquide de menos sin enterarse.
  if (ajustes.length > MAX_REFACTURAS) {
    erroresCriticos.push(
      `El lote trae ${ajustes.length} archivos de refactura y el máximo admitido es ${MAX_REFACTURAS}. ` +
      `Archivos: ${ajustes.map(a => a.nombre).join(", ")}. ` +
      "Cargalos en dos veces o pedí ampliar las columnas de ajuste.",
    )
    return { filas, alertas, erroresCriticos }
  }

  if (facturas.length === 0) {
    alertas.push("El lote no trae archivo de factura (tipofactu): solo se registrarán ajustes.")
  }

  // ── PASO 3: Extraer valores de cada archivo ─────────────────────────────
  const valoresFactura: Record<string, number> = {}
  const archivosFactura: string[] = []
  for (const f of facturas) {
    const valores = extraerValoresPorOR(f.buffer, f.nombre, ["BalSTR01", "BalSTR02"], alertas)
    if (!valores) continue
    archivosFactura.push(f.nombre)
    for (const [or, v] of Object.entries(valores)) {
      valoresFactura[or] = (valoresFactura[or] ?? 0) + v
    }
  }

  // Un elemento por archivo de ajuste, en el orden ya definido.
  const valoresAjuste: Array<{ nombre: string; valores: Record<string, number> }> = []
  for (const a of ajustes) {
    const valores = extraerValoresPorOR(
      a.buffer, a.nombre, ["BalSTR01_Ajuste", "BalSTR02_Ajuste"], alertas,
    )
    if (valores) valoresAjuste.push({ nombre: a.nombre, valores })
  }

  // ── PASO 4: Pivotar a una fila por operador ─────────────────────────────
  const operadores = new Set<string>([
    ...Object.keys(valoresFactura),
    ...valoresAjuste.flatMap(a => Object.keys(a.valores)),
  ])

  for (const orCodigo of [...operadores].sort()) {
    const valorFactura = valoresFactura[orCodigo] ?? 0

    // null = ese archivo no vino en el lote. 0 = vino y el operador tenía cero.
    const refacturas: (number | null)[] = [null, null, null]
    for (let i = 0; i < valoresAjuste.length; i++) {
      refacturas[i] = valoresAjuste[i]!.valores[orCodigo] ?? 0
    }

    const total = valorFactura + refacturas.reduce<number>((acc, v) => acc + (v ?? 0), 0)

    filas.push({
      or_codigo:         orCodigo,
      periodo,
      valor_factura:     valorFactura,
      refactura_1_valor: refacturas[0]!,
      refactura_2_valor: refacturas[1]!,
      refactura_3_valor: refacturas[2]!,
      valor_a_pagar:     total,
      detalle: {
        archivos_factura: archivosFactura,
        archivos_ajuste:  valoresAjuste.map(a => a.nombre),
      },
    })
  }

  if (filas.length === 0 && erroresCriticos.length === 0) {
    alertas.push("No se generaron registros — revisá los archivos cargados.")
  }

  return { filas, alertas, erroresCriticos }
}
