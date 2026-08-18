// Genera el fixture dorado de Tarifas SDL: las tarifas que produce el parser
// TypeScript con los archivos de ejemplo, para que el port a Go se compare contra
// ellas (services/tarifas_sdl/testdata/golden_ts.json).
//
// Está APAGADO por defecto porque escribe un archivo, y un test que escribe en
// disco no debería correr en cada `npm run test`. Se regenera a mano cuando
// cambian los archivos de ejemplo:
//
//   REGENERAR_GOLDEN_SDL=1 npx vitest run lib/parsers/__tests__/zz-sdl-golden.test.ts
//
// Este archivo se borra cuando el TypeScript de Tarifas SDL salga de producción,
// junto con el resto del módulo.
import { describe, it } from "vitest"
import * as fs from "fs"
import * as path from "path"
import { parsearInsumosTarifasSDL } from "@/lib/parsers/insumos-tarifas-sdl"

const habilitado = process.env.REGENERAR_GOLDEN_SDL === "1"

describe.skipIf(!habilitado)("golden SDL", () => {
  it("exporta las tarifas del parser TypeScript", () => {
    const base = path.join(process.cwd(), "archivos_ejemplo", "formatos_or", "Insumos SDL")
    const archivos: { nombre: string; data: Uint8Array }[] = []
    for (const sub of ["Cargos ADD DT", "Cargos por uso de la red"]) {
      for (const n of fs.readdirSync(path.join(base, sub))) {
        if (!n.toLowerCase().endsWith(".xlsx")) continue
        archivos.push({ nombre: n, data: new Uint8Array(fs.readFileSync(path.join(base, sub, n))) })
      }
    }

    const res = parsearInsumosTarifasSDL(archivos)
    fs.writeFileSync(
      path.join(process.cwd(), "services", "tarifas_sdl", "testdata", "golden_ts.json"),
      JSON.stringify(
        {
          archivos: archivos.length,
          alertas: res.alertas,
          erroresCriticos: res.erroresCriticos,
          orsSinDatos: res.orsSinDatos,
          filas: res.filas,
        },
        null,
        2,
      ),
    )
    console.log(`filas TS: ${res.filas.length}  sinDatos: ${res.orsSinDatos.join(",")}`)
  })
})
