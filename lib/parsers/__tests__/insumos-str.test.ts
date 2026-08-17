import { existsSync, readFileSync, readdirSync } from "fs"
import { join } from "path"
import { describe, expect, it } from "vitest"
import { MAX_REFACTURAS, parsearInsumosSTR } from "@/lib/parsers/insumos-str"

/**
 * Validación de Cargos STR contra archivos REALES.
 *
 * Reproduce el número de referencia que validó la sesión "Back Insumos STR" con
 * el servicio aparte, ejecutando el parser de ESTE backend — el mismo que corre
 * `/api/cargas/preview` en su rama `INSUMOS_STR`.
 *
 *   CHEC = 70.812.140 (factura) − 906 − 3.536 (ajustes) = 70.807.698
 *
 * Los archivos viven en `archivos_ejemplo/STR/` y NO están versionados (son
 * insumos reales). Si no están, esos tests se saltan en vez de fallar.
 */

const DIR_EJEMPLOS = join(process.cwd(), "archivos_ejemplo", "STR")

// El período que el usuario elige en el wizard. Desde la migración, es la ÚNICA
// fuente del período: ya no se deduce del nombre del archivo.
const ANIO = 2026
const MES = 5
const PERIODO = "2026-05"

const CHEC_FACTURA = 70_812_140
const CHEC_AJUSTES = -906 + -3_536
const CHEC_A_PAGAR = 70_807_698
const TOTAL_LOTE = 1_460_833_304
const OPERADORES = 23

const hayArchivos =
  existsSync(DIR_EJEMPLOS) && readdirSync(DIR_EJEMPLOS).some((f) => f.endsWith(".xlsx"))

const cargarBuffers = () =>
  readdirSync(DIR_EJEMPLOS)
    .filter((f) => f.endsWith(".xlsx"))
    .map((nombre) => ({ buffer: readFileSync(join(DIR_EJEMPLOS, nombre)), nombre }))

describe.skipIf(!hayArchivos)("parsearInsumosSTR — archivos reales de archivos_ejemplo/STR", () => {
  it("devuelve una fila por operador, no una por archivo", async () => {
    const { filas, erroresCriticos } = await parsearInsumosSTR(cargarBuffers(), ANIO, MES)

    expect(erroresCriticos).toEqual([])
    expect(filas).toHaveLength(OPERADORES)
    expect(new Set(filas.map((f) => f.or_codigo)).size).toBe(OPERADORES)
  })

  it("usa el período del filtro, no el del nombre del archivo", async () => {
    const { filas } = await parsearInsumosSTR(cargarBuffers(), ANIO, MES)

    // El lote trae refacturas de 2025-NOV y 2026-MAR; todas quedan en el
    // período seleccionado.
    expect(new Set(filas.map((f) => f.periodo))).toEqual(new Set([PERIODO]))
  })

  it("toma el período aunque no coincida con ningún archivo del lote", async () => {
    const { filas } = await parsearInsumosSTR(cargarBuffers(), 2026, 7)

    expect(new Set(filas.map((f) => f.periodo))).toEqual(new Set(["2026-07"]))
  })

  it("reproduce el valor a pagar de CHEC validado end-to-end", async () => {
    const { filas } = await parsearInsumosSTR(cargarBuffers(), ANIO, MES)
    const chec = filas.find((f) => f.or_codigo === "CHEC")!

    expect(chec.valor_factura).toBe(CHEC_FACTURA)
    // Dos archivos de ajuste en el lote: cada uno en su columna, sin sumarlos.
    expect(chec.refactura_1_valor! + chec.refactura_2_valor!).toBe(CHEC_AJUSTES)
    expect(chec.valor_a_pagar).toBe(CHEC_A_PAGAR)
  })

  it("deja en NULL las columnas de ajuste que el lote no trajo", async () => {
    const { filas } = await parsearInsumosSTR(cargarBuffers(), ANIO, MES)

    // El lote tiene 2 refacturas: la tercera columna queda sin dato en TODOS.
    expect(filas.every((f) => f.refactura_3_valor === null)).toBe(true)
    expect(filas.every((f) => f.refactura_1_valor !== null)).toBe(true)
  })

  it("distingue 'no vino el archivo' (null) de 'vino en cero' (0)", async () => {
    const { filas } = await parsearInsumosSTR(cargarBuffers(), ANIO, MES)
    const aire = filas.find((f) => f.or_codigo === "AIRE")!

    // AIRE aparece en los ajustes con valor cero, no ausente.
    expect(aire.refactura_1_valor! + aire.refactura_2_valor!).toBe(0)
    expect(aire.refactura_1_valor).not.toBeNull()
    // Y suma sus dos columnas del Excel (CSID + CSSD) en un solo operador.
    expect(aire.valor_factura).toBe(142_265_108)
  })

  it("el total del lote cuadra con la suma de los operadores", async () => {
    const { filas } = await parsearInsumosSTR(cargarBuffers(), ANIO, MES)

    const total = filas.reduce((acc, f) => acc + f.valor_a_pagar, 0)
    expect(total).toBe(TOTAL_LOTE)

    // Y cada fila cuadra consigo misma.
    for (const f of filas) {
      const suma =
        f.valor_factura +
        (f.refactura_1_valor ?? 0) +
        (f.refactura_2_valor ?? 0) +
        (f.refactura_3_valor ?? 0)
      expect(suma).toBe(f.valor_a_pagar)
    }
  })
})

describe("parsearInsumosSTR — reglas del lote", () => {
  const falso = (nombre: string) => ({ buffer: Buffer.alloc(0), nombre })

  it("corta la carga si llegan más ajustes que columnas disponibles", async () => {
    const { erroresCriticos, filas } = await parsearInsumosSTR(
      [
        falso("BalanceSTRTipoFactu2026-MAY.xlsx"),
        falso("BalanceSTRTipoReFactu2025-NOV-1.xlsx"),
        falso("BalanceSTRTipoReFactu2025-DIC-1.xlsx"),
        falso("BalanceSTRTipoReFactu2026-ENE-1.xlsx"),
        falso("BalanceSTRTipoReFactu2026-MAR-1.xlsx"),
      ],
      2026,
      5,
    )

    // Falla explícita: nunca descartar un ajuste en silencio.
    expect(filas).toHaveLength(0)
    expect(erroresCriticos).toHaveLength(1)
    expect(erroresCriticos[0]).toContain(String(MAX_REFACTURAS))
    expect(erroresCriticos[0]).toContain("BalanceSTRTipoReFactu2026-MAR-1.xlsx")
  })

  it("avisa si no viene ningún archivo", async () => {
    const { erroresCriticos } = await parsearInsumosSTR([], 2026, 5)
    expect(erroresCriticos).toHaveLength(1)
  })
})
