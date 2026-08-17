import { NextResponse } from "next/server"

/**
 * GET /api/live — liveness probe.
 *
 * Es el "Health check path" que consulta cactus para decidir si el contenedor
 * está sano y puede recibir tráfico. Por eso, a diferencia del resto del
 * backend, NO exige autenticación: una sonda no tiene sesión, y si respondiera
 * 401 el servicio quedaría marcado como caído y nunca recibiría tráfico.
 *
 * Tampoco consulta la base de datos, y es a propósito: si el health check
 * dependiera de Postgres, un hipo de red haría que la plataforma reinicie un
 * contenedor que estaba perfectamente vivo. Acá solo se responde "el proceso
 * está en pie y sirviendo".
 *
 * Para saber si las bases responden está `/api/health`, que sí autentica y sí
 * las consulta — ese es diagnóstico, no liveness.
 */
export const runtime = "nodejs"
export const dynamic = "force-dynamic"

export function GET() {
  return NextResponse.json({ ok: true, service: "liquidaciones-backend" })
}
