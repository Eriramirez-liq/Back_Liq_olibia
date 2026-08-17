package router

import (
	"bia-bills/controllers"
	"bia-bills/providers/postgres"

	"github.com/gin-gonic/gin"
)

// Rutas del módulo de Liquidaciones.
//
// ⚠️ TRASVASE: archivo `liquidations_router.go`, aparte de `router.go`, que tiene
// ~600 líneas del servicio y NO se puede pisar.
//
// Toda la inyección de dependencias del módulo vive acá adentro. Así la única
// edición manual en el `router.go` de bia-bills es una línea:
//
//	RegisterLiquidations(apiPrefix)
//
// Todo lo demás es copiar archivos. Ver docs/backend/migracion-a-go.md.
//
// Las rutas quedan bajo el prefijo del servicio, o sea /ms-bill/liquidations/...
// Ese es el path que el front usa en su endpoints.ts para lo ya migrado.
func RegisterLiquidations(apiPrefix *gin.RouterGroup) {
	// ── Providers ───────────────────────────────────────────────────────────
	db := postgres.NewLiquidationsDB()

	// ── Controllers ─────────────────────────────────────────────────────────
	healthController := controllers.NewLiquidationsHealthController(db)

	// ── Rutas ───────────────────────────────────────────────────────────────
	liquidationsGroup := apiPrefix.Group("/liquidations")
	liquidationsGroup.GET("/health", healthController.Health)
}
