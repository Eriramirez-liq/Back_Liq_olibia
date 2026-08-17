import { beforeEach, describe, expect, it, vi } from "vitest"
import { jwtVerify } from "jose"
import { db } from "@/lib/db"
import {
  authFromAuthorizationHeader,
  bearerToken,
  resolveOlibiaUser,
  verifyOlibiaIdToken,
} from "@/lib/auth-olibia"

/**
 * Tests del puente de auth con olibia-web. No tocan red ni DB: `jose` y
 * `lib/db` están mockeados, igual que el resto de la suite (ver vitest.config).
 */

vi.mock("jose", () => ({
  createRemoteJWKSet: vi.fn(() => ({ mockJwks: true })),
  jwtVerify: vi.fn(),
}))

vi.mock("@/lib/db", () => ({
  db: { user: { findUnique: vi.fn(), create: vi.fn() } },
}))

vi.mock("bcryptjs", () => ({
  default: { hash: vi.fn(async () => "hash-inservible") },
}))

const jwtVerifyMock = vi.mocked(jwtVerify)
const findUnique = vi.mocked(db.user.findUnique)
const create = vi.mocked(db.user.create)

const PROJECT_ID = "bia-eva-dev"

const validPayload = {
  sub: "firebase-uid-123",
  email: "Ana.Perez@bia.app",
  name: "Ana Pérez",
}

beforeEach(() => {
  vi.clearAllMocks()
  process.env.FIREBASE_PROJECT_ID = PROJECT_ID
  process.env.OLIBIA_ALLOWED_EMAIL_DOMAINS = "@bia.app"
  process.env.OLIBIA_AUTO_PROVISION_USERS = "true"
  process.env.OLIBIA_DEFAULT_ROL = "ANALISTA"
})

describe("bearerToken", () => {
  it("extrae el token del header Bearer, sin importar el case del scheme", () => {
    expect(bearerToken("Bearer abc.def.ghi")).toBe("abc.def.ghi")
    expect(bearerToken("bearer abc.def.ghi")).toBe("abc.def.ghi")
  })

  it("ignora headers ausentes, vacíos o de otro scheme", () => {
    expect(bearerToken(null)).toBeNull()
    expect(bearerToken(undefined)).toBeNull()
    expect(bearerToken("Bearer")).toBeNull()
    expect(bearerToken("Bearer   ")).toBeNull()
    expect(bearerToken("Basic dXNlcjpwYXNz")).toBeNull()
  })
})

describe("verifyOlibiaIdToken", () => {
  it("valida contra el issuer y audience del proyecto de Firebase del front", async () => {
    jwtVerifyMock.mockResolvedValue({ payload: validPayload } as never)

    const identity = await verifyOlibiaIdToken("token")

    expect(identity).toEqual({
      email: "ana.perez@bia.app",
      uid: "firebase-uid-123",
      nombre: "Ana Pérez",
    })
    expect(jwtVerifyMock).toHaveBeenCalledWith("token", expect.anything(), {
      issuer: `https://securetoken.google.com/${PROJECT_ID}`,
      audience: PROJECT_ID,
      algorithms: ["RS256"],
    })
  })

  it("cae al usuario del correo cuando el token no trae name", async () => {
    jwtVerifyMock.mockResolvedValue({
      payload: { sub: "uid", email: "ana@bia.app" },
    } as never)

    expect((await verifyOlibiaIdToken("token"))?.nombre).toBe("ana")
  })

  it("rechaza si el token es inválido o está vencido", async () => {
    jwtVerifyMock.mockRejectedValue(new Error("JWTExpired"))

    expect(await verifyOlibiaIdToken("token")).toBeNull()
  })

  it("rechaza si el token no trae email o sub", async () => {
    jwtVerifyMock.mockResolvedValue({ payload: { sub: "uid" } } as never)
    expect(await verifyOlibiaIdToken("token")).toBeNull()

    jwtVerifyMock.mockResolvedValue({ payload: { email: "ana@bia.app" } } as never)
    expect(await verifyOlibiaIdToken("token")).toBeNull()
  })

  it("rechaza —sin llamar a jose— si falta FIREBASE_PROJECT_ID", async () => {
    delete process.env.FIREBASE_PROJECT_ID
    delete process.env.NEXT_PUBLIC_FIREBASE_PROJECT_ID

    expect(await verifyOlibiaIdToken("token")).toBeNull()
    expect(jwtVerifyMock).not.toHaveBeenCalled()
  })
})

describe("resolveOlibiaUser", () => {
  const identity = { email: "ana.perez@bia.app", uid: "uid", nombre: "Ana Pérez" }

  it("devuelve el usuario existente con su rol", async () => {
    findUnique.mockResolvedValue({
      id: "u1",
      nombre: "Ana Pérez",
      rol: "ADMINISTRADOR",
      activo: true,
    } as never)

    expect(await resolveOlibiaUser(identity)).toEqual({
      id: "u1",
      nombre: "Ana Pérez",
      rol: "ADMINISTRADOR",
    })
    expect(create).not.toHaveBeenCalled()
  })

  it("rechaza usuarios desactivados", async () => {
    findUnique.mockResolvedValue({
      id: "u1",
      nombre: "Ana",
      rol: "ANALISTA",
      activo: false,
    } as never)

    expect(await resolveOlibiaUser(identity)).toBeNull()
  })

  it("rechaza correos fuera del dominio permitido, sin consultar la DB", async () => {
    expect(await resolveOlibiaUser({ ...identity, email: "ana@gmail.com" })).toBeNull()
    expect(findUnique).not.toHaveBeenCalled()
  })

  it("provisiona el usuario la primera vez con el rol por defecto", async () => {
    findUnique.mockResolvedValue(null as never)
    create.mockResolvedValue({ id: "u2", nombre: "Ana Pérez", rol: "ANALISTA" } as never)

    expect(await resolveOlibiaUser(identity)).toEqual({
      id: "u2",
      nombre: "Ana Pérez",
      rol: "ANALISTA",
    })
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({
        data: expect.objectContaining({
          email: "ana.perez@bia.app",
          nombre: "Ana Pérez",
          rol: "ANALISTA",
          activo: true,
        }),
      })
    )
  })

  it("no provisiona si OLIBIA_AUTO_PROVISION_USERS=false", async () => {
    process.env.OLIBIA_AUTO_PROVISION_USERS = "false"
    findUnique.mockResolvedValue(null as never)

    expect(await resolveOlibiaUser(identity)).toBeNull()
    expect(create).not.toHaveBeenCalled()
  })
})

describe("authFromAuthorizationHeader", () => {
  it("resuelve el usuario a partir del header que manda el proxy de olibia", async () => {
    jwtVerifyMock.mockResolvedValue({ payload: validPayload } as never)
    findUnique.mockResolvedValue({
      id: "u1",
      nombre: "Ana Pérez",
      rol: "ANALISTA",
      activo: true,
    } as never)

    expect(await authFromAuthorizationHeader("Bearer token")).toEqual({
      id: "u1",
      nombre: "Ana Pérez",
      rol: "ANALISTA",
    })
  })

  it("no autentica sin header, y no intenta validar nada", async () => {
    expect(await authFromAuthorizationHeader(null)).toBeNull()
    expect(jwtVerifyMock).not.toHaveBeenCalled()
  })
})
