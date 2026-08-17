import { headers } from "next/headers"
import { auth as nextAuth, handlers, signIn, signOut } from "@/auth"
import { authFromAuthorizationHeader } from "@/lib/auth-olibia"

export { handlers, signIn, signOut }

type AppUser = {
  id: string
  nombre: string
  rol: string
}

type AppSession = {
  user: AppUser
  /**
   * Cómo se autenticó el request:
   *  - "credentials": login propio de este backend (NextAuth, cookie de sesión).
   *  - "olibia": Firebase ID token reenviado por el proxy de olibia-web.
   */
  source: "credentials" | "olibia"
}

/**
 * Sesión del request. Dos caminos, en este orden:
 *
 *  1. Cookie de NextAuth — el /login propio de este backend y su dashboard.
 *  2. `Authorization: Bearer <Firebase ID token>` — todo el consumo desde
 *     olibia-web, que lo inyecta server-side en /api/liquidations-proxy.
 *
 * Devuelve la misma forma en ambos casos para que las rutas no tengan que
 * distinguir el origen: `const session = await auth()`.
 */
export async function auth(): Promise<AppSession | null> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const session = (await nextAuth()) as any
  if (session?.user?.id) {
    return {
      user: {
        id: session.user.id,
        nombre: session.user.nombre ?? session.user.name ?? "Usuario",
        rol: session.user.rol ?? "ANALISTA",
      },
      source: "credentials",
    }
  }

  try {
    const authorization = (await headers()).get("authorization")
    const user = await authFromAuthorizationHeader(authorization)
    if (user) return { user, source: "olibia" }
  } catch {
    // headers() fuera de un request scope: no hay nada que autenticar.
  }

  return null
}
