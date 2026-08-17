package router

import (
	"bia-bills/controllers"
	"bia-bills/providers/postgres"
	"bia-bills/repositories"
	"bia-bills/services/cargos_str"

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

	// ── Repositories ────────────────────────────────────────────────────────
	strRepository := repositories.NewLiquidationsStrRepository(db)
	agentsRepository := repositories.NewLiquidationsAgentsRepository(db)

	// ── Services ────────────────────────────────────────────────────────────
	cargosStrService := cargos_str.NewCargosStrService(strRepository, agentsRepository)

	// ── Controllers ─────────────────────────────────────────────────────────
	healthController := controllers.NewLiquidationsHealthController(db)
	cargosStrController := controllers.NewLiquidationsStrController(cargosStrService)

	// ── Rutas ───────────────────────────────────────────────────────────────
	liquidationsGroup := apiPrefix.Group("/liquidations")
	liquidationsGroup.GET("/health", healthController.Health)

	cargosStrGroup := liquidationsGroup.Group("/cargos-str")
	cargosStrGroup.POST("/preview", cargosStrController.Preview)
	cargosStrGroup.POST("/confirm", cargosStrController.Confirm)
	cargosStrGroup.GET("", cargosStrController.Charges)
	cargosStrGroup.GET("/periods", cargosStrController.Periods)
}
