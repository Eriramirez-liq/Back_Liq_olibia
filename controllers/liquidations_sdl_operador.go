package controllers

import (
	"net/http"

	"bia-bills/repositories"
	sdlOperador "bia-bills/services/sdl_operador"

	contextBia "github.com/biaenergy/bia-commons-go/context"
	// gin/errors y NO gin: el paquete completo arrastra handlers → cache →
	// go-redis, que no está en el go.sum de este repo. Mismo import que usan los
	// controllers de Cargos STR, Tarifas SDL y TC1.
	ginCommons "github.com/biaenergy/bia-commons-go/gin/errors"
	"github.com/biaenergy/bia-commons-go/tracing"
	"github.com/gin-gonic/gin"
)

// Endpoints de SDL por Operador (preliquidación).
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_` para no pisar ninguno de los
// controllers del servicio. Ver docs/backend/migracion-a-go.md.
//
// ── No hay preview ni multipart ─────────────────────────────────────────────
// Igual que TC1: el archivo se parsea en el navegador y llegan las filas ya
// normalizadas en JSON. Acá el motivo no es el peso sino el parser — son 2.000
// líneas con una rareza por operador, ya probadas en TypeScript.

type LiquidationsSdlOperadorController struct {
	service sdlOperador.SdlOperadorService
}

func NewLiquidationsSdlOperadorController(
	service sdlOperador.SdlOperadorService,
) LiquidationsSdlOperadorController {
	return LiquidationsSdlOperadorController{service: service}
}

type confirmarSdlOperadorRequest struct {
	// El período lo elige la persona en Nueva carga; el operador es de quien se
	// está cargando el archivo.
	Period       string `json:"period" binding:"required"`
	OperatorCode string `json:"operator_code" binding:"required"`

	Rows []sdlOperador.Row `json:"rows" binding:"required"`

	// Para el historial de cargas. El id del usuario NO viene acá: sale del
	// header, que es el único dato de identidad que no puede falsear quien arma
	// el request.
	CreatedBy  string `json:"created_by"`
	SourceFile string `json:"source_file"`
}

// Confirm guarda las fronteras de una preliquidación.
//
// POST /liquidations/sdl-operador/confirm
func (controller LiquidationsSdlOperadorController) Confirm(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlOperadorController.Confirm")
	defer span.End()

	var body confirmarSdlOperadorRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(err.Error()))
		return
	}

	meta := sdlOperador.LoadMeta{
		CreatedBy:   body.CreatedBy,
		CreatedByID: c.GetHeader("x-user-id"),
		SourceFile:  body.SourceFile,
	}

	loadID, err := controller.service.Confirm(ctx, body.Period, body.OperatorCode, body.Rows, meta)
	if err != nil {
		// Lo que rechaza el servicio son datos del cargue —período con mala forma,
		// período del archivo que no cuadra, frontera vacía—, así que es un 422 y
		// no un 500: el request está bien armado, lo que no sirve es su contenido.
		//
		// La forma { error } es la que el front normaliza; ver CLAUDE.md.
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"load_id": loadID, "borders": len(body.Rows)})
}

// Rows devuelve las fronteras vigentes por (período, operador).
//
// GET /liquidations/sdl-operador?periods=2026-07&operators=CHEC
func (controller LiquidationsSdlOperadorController) Rows(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlOperadorController.Rows")
	defer span.End()

	filas, err := controller.service.CurrentRows(ctx, repositories.SdlPreliqFilter{
		Periods:       listaDelQuery(c, "periods"),
		OperatorCodes: listaDelQuery(c, "operators"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"rows": filas})
}

// Loads lista el historial de cargues.
//
// GET /liquidations/sdl-operador/loads?periods=2026-07
func (controller LiquidationsSdlOperadorController) Loads(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlOperadorController.Loads")
	defer span.End()

	cargues, err := controller.service.Loads(ctx, listaDelQuery(c, "periods"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"loads": cargues})
}

// Operators lista los operadores que reportan preliquidación SDL.
//
// GET /liquidations/sdl-operador/operators
func (controller LiquidationsSdlOperadorController) Operators(c *gin.Context) {
	c.JSON(http.StatusOK, controller.service.Operators())
}

// Status dice cuántos operadores ya cargaron su preliquidación y cuáles faltan.
//
// GET /liquidations/sdl-operador/status?period=2026-07
func (controller LiquidationsSdlOperadorController) Status(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlOperadorController.Status")
	defer span.End()

	estado, err := controller.service.Status(ctx, c.Query("period"))
	if err != nil {
		// El único error del servicio acá es un período con mala forma, que es un
		// dato del request y no una falla del servidor.
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, estado)
}

// Periods lista los períodos con preliquidaciones cargadas.
//
// GET /liquidations/sdl-operador/periods
func (controller LiquidationsSdlOperadorController) Periods(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsSdlOperadorController.Periods")
	defer span.End()

	periodos, err := controller.service.Periods(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, periodos)
}
