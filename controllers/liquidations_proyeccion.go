package controllers

import (
	"net/http"
	"strconv"

	"bia-bills/services/proyeccion"

	contextBia "github.com/biaenergy/bia-commons-go/context"
	ginCommons "github.com/biaenergy/bia-commons-go/gin/errors"
	"github.com/biaenergy/bia-commons-go/tracing"
	"github.com/gin-gonic/gin"
)

// Endpoints de Proyección Cargos OR.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// Solo la parte que ya vive en las bases de BIA: los precios por nivel de tensión
// y el valor STR del mes. La demanda —que es de donde cuelga toda la
// valorización— sigue en Facturación, en Supabase, y por eso no está acá.

type LiquidationsProyeccionController struct {
	service proyeccion.ProyeccionService
}

func NewLiquidationsProyeccionController(
	service proyeccion.ProyeccionService,
) LiquidationsProyeccionController {
	return LiquidationsProyeccionController{service: service}
}

// Prices devuelve los precios por nivel y el valor STR de cada período.
//
// GET /liquidations/proyeccion/prices?periods=2026-01,2026-02&months=3
//
// Sin `periods` devuelve todos los períodos con datos, que es lo que la pantalla
// necesita: antes la lista de meses salía de Facturación y sin ella la matriz
// quedaba vacía aunque hubiera tarifas y cargos cargados.
func (controller LiquidationsProyeccionController) Prices(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsProyeccionController.Prices")
	defer span.End()

	// Cuántos meses proyectar después del último real. 0 = solo los reales.
	meses, _ := strconv.Atoi(c.Query("months"))

	filas, err := controller.service.Prices(ctx, listaDelQuery(c, "periods"), meses)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	// Los porcentajes van con la respuesta: la pantalla los muestra como etiqueta
	// de cada fila y así no quedan duplicados en el front.
	c.JSON(http.StatusOK, gin.H{
		"percentages": proyeccion.PorcentajesVigentes(),
		"months":      filas,
	})
}
