import { createRemoteJWKSet, jwtVerify } from "jose"
import bcrypt from "bcryptjs"
import { randomUUID } from "crypto"
import { db } from "@/lib/db"
import { Rol } from "@prisma/client"

/**
 * Autenticación de requests que llegan desde olibia-web.
 *
 * El front (olibia-web) guarda el Firebase ID token del usuario en una cookie
 * httpOnly y lo reenvía como `Authorization: Bearer <idToken>` desde su proxy
 * server-side `/api/liquidations-proxy/[...path]`. Este módulo valida ese token
 * contra las llaves públicas de Google y lo mapea a un `users` de este backend.
 *
 * Es el camino de auth para todo consumo desde el front. El login propio de
 * este backend (NextAuth + credenciales) sigue existiendo y tiene prioridad;
 * ver `lib/auth.ts`, que compone ambos.
 */

// Llaves públicas con las que Google firma los Firebase ID tokens. `jose`
// cachea y respeta el TTL del endpoint, por eso el set es un singleton.
const FIREBASE_JWKS_URL =
  "https://www.googleapis.com/service_accounts/v1/jwk/securetoken@system.gserviceaccount.com"

let jwks: ReturnType<typeof createRemoteJWKSet> | null = null

function getJwks() {
  if (!jwks) jwks = createRemoteJWKSet(new URL(FIREBASE_JWKS_URL))
  return jwks
}

function firebaseProjectId(): string | undefined {
  return process.env.FIREBASE_PROJECT_ID ?? process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID
}

// Dominios de correo aceptados. Espeja el `ALLOWED_DOMAIN` del login propio
// (auth.ts) pero es configurable por si entra otro dominio corporativo.
function allowedDomains(): string[] {
  return (process.env.OLIBIA_ALLOWED_EMAIL_DOMAINS ?? "@bia.app")
    .split(",")
    .map((d) => d.trim().toLowerCase())
    .filter(Boolean)
}

// Si el usuario autenticado en olibia no existe todavía en `users`, se crea con
// rol de solo lectura ampliable desde Administración. Poner en "false" para
// exigir que los usuarios estén dados de alta previamente (403 si no existen).
function autoProvisionEnabled(): boolean {
  return (process.env.OLIBIA_AUTO_PROVISION_USERS ?? "true").toLowerCase() !== "false"
}

function defaultRol(): Rol {
  const raw = (process.env.OLIBIA_DEFAULT_ROL ?? "ANALISTA").toUpperCase()
  return raw in Rol ? (raw as Rol) : Rol.ANALISTA
}

export interface OlibiaIdentity {
  email: string
  uid: string
  nombre: string
}

/** Extrae el token de un header `Authorization: Bearer <token>`. */
export function bearerToken(authorization: string | null | undefined): string | null {
  if (!authorization) return null
  const [scheme, ...rest] = authorization.trim().split(/\s+/)
  if (scheme?.toLowerCase() !== "bearer") return null
  const token = rest.join("")
  return token.length > 0 ? token : null
}

/**
 * Valida un Firebase ID token (firma RS256 + iss/aud del proyecto + expiración)
 * y devuelve la identidad. `null` si el token es inválido, está vencido o el
 * backend no tiene configurado FIREBASE_PROJECT_ID.
 */
export async function verifyOlibiaIdToken(token: string): Promise<OlibiaIdentity | null> {
  const projectId = firebaseProjectId()
  if (!projectId) {
    console.warn(
      "[auth-olibia] FIREBASE_PROJECT_ID no configurado: se rechazan los tokens de olibia-web"
    )
    return null
  }

  try {
    const { payload } = await jwtVerify(token, getJwks(), {
      issuer: `https://securetoken.google.com/${projectId}`,
      audience: projectId,
      algorithms: ["RS256"],
    })

    const email = typeof payload.email === "string" ? payload.email.toLowerCase().trim() : ""
    const uid = typeof payload.sub === "string" ? payload.sub : ""
    if (!email || !uid) return null

    const nombre =
      (typeof payload.name === "string" && payload.name.trim()) || email.split("@")[0] || email

    return { email, uid, nombre }
  } catch {
    // Token vencido, firma inválida, aud/iss de otro proyecto, red caída al
    // traer las JWKS. En todos los casos el request no está autenticado: el
    // front reintenta con `getIdToken(true)` cuando ve el 401.
    return null
  }
}

export interface OlibiaUser {
  id: string
  nombre: string
  rol: string
}

/**
 * Resuelve el `users` local que corresponde a una identidad de olibia.
 * `null` si el dominio no está permitido, el usuario está inactivo, o no existe
 * y el auto-provisioning está apagado.
 */
export async function resolveOlibiaUser(identity: OlibiaIdentity): Promise<OlibiaUser | null> {
  const { email, nombre } = identity

  if (!allowedDomains().some((domain) => email.endsWith(domain))) return null

  const existing = await db.user.findUnique({
    where: { email },
    select: { id: true, nombre: true, rol: true, activo: true },
  })

  if (existing) {
    if (!existing.activo) return null
    return { id: existing.id, nombre: existing.nombre, rol: existing.rol }
  }

  if (!autoProvisionEnabled()) return null

  // Password aleatorio e inservible: el alta viene de Firebase, este usuario no
  // debe poder entrar por el /login de credenciales de este backend.
  const created = await db.user.create({
    data: {
      email,
      nombre,
      password: await bcrypt.hash(randomUUID(), 10),
      rol: defaultRol(),
      activo: true,
    },
    select: { id: true, nombre: true, rol: true },
  })

  console.info(`[auth-olibia] usuario provisionado desde olibia-web: ${email} (${created.rol})`)

  return { id: created.id, nombre: created.nombre, rol: created.rol }
}

/** Autentica un request por su header Authorization. `null` si no aplica. */
export async function authFromAuthorizationHeader(
  authorization: string | null | undefined
): Promise<OlibiaUser | null> {
  const token = bearerToken(authorization)
  if (!token) return null

  const identity = await verifyOlibiaIdToken(token)
  if (!identity) return null

  return resolveOlibiaUser(identity)
}
