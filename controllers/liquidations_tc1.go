package controllers

import (
	"net/http"

	"bia-bills/repositories"
	"bia-bills/services/tc1"

	contextBia "github.com/biaenergy/bia-commons-go/context"
	// gin/errors y NO gin: el paquete completo arrastra handlers → cache →
	// go-redis, que no está en el go.sum de este repo. Mismo import que usan los
	// controllers de Cargos STR y Tarifas SDL.
	ginCommons "github.com/biaenergy/bia-commons-go/gin/errors"
	"github.com/biaenergy/bia-commons-go/tracing"
	"github.com/gin-gonic/gin"
)

// Endpoints de TC1.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_` para no pisar ninguno de los
// controllers del servicio. Ver docs/backend/migracion-a-go.md.
//
// ── No hay preview ni multipart ─────────────────────────────────────────────
// A diferencia de Cargos STR y Tarifas SDL, acá no se sube el archivo: se parsea
// en el navegador y llegan las filas ya normalizadas en JSON. El archivo de
// CELSIA_VALLE pesa 98 MB y el filtro por comercializador lo deja en 156 filas,
// así que subir el crudo sería mover 98 MB para guardar unos kilobytes.

type LiquidationsTc1Controller struct {
	service tc1.Tc1Service
}

func NewLiquidationsTc1Controller(service tc1.Tc1Service) LiquidationsTc1Controller {
	return LiquidationsTc1Controller{service: service}
}

type confirmarTc1Request struct {
	// El período y el operador los elige la persona en Nueva carga. El archivo de
	// TC1 no trae período, y el operador es de quien se está cargando el archivo.
	Period       string `json:"period" binding:"required"`
	OperatorCode string `json:"operator_code" binding:"required"`

	Rows []tc1.Row `json:"rows" binding:"required"`

	// Para el historial de cargas. El id del usuario NO viene acá: sale del
	// header, que es el único dato de identidad que no puede falsear quien arma
	// el request.
	CreatedBy  string `json:"created_by"`
	SourceFile string `json:"source_file"`
}

// Confirm guarda las fronteras de un archivo TC1.
//
// POST /liquidations/tc1/confirm
func (controller LiquidationsTc1Controller) Confirm(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsTc1Controller.Confirm")
	defer span.End()

	var body confirmarTc1Request
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ginCommons.NewBadRequest(err.Error()))
		return
	}

	meta := tc1.LoadMeta{
		CreatedBy:   body.CreatedBy,
		CreatedByID: c.GetHeader("x-user-id"),
		SourceFile:  body.SourceFile,
	}

	loadID, err := controller.service.Confirm(ctx, body.Period, body.OperatorCode, body.Rows, meta)
	if err != nil {
		// Lo que rechaza el servicio son datos del cargue —período con mala forma,
		// frontera vacía o repetida—, así que es un 422 y no un 500: el request
		// está bien armado, lo que no sirve es su contenido.
		//
		// La forma { error } es la que el front normaliza; ver CLAUDE.md.
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"load_id": loadID, "borders": len(body.Rows)})
}

// Inputs devuelve las fronteras vigentes por (período, operador).
//
// GET /liquidations/tc1?periods=2026-02&operators=CENS,CHEC
func (controller LiquidationsTc1Controller) Inputs(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsTc1Controller.Inputs")
	defer span.End()

	filas, err := controller.service.CurrentInputs(ctx, repositories.Tc1Filter{
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
// GET /liquidations/tc1/loads?periods=2026-02
func (controller LiquidationsTc1Controller) Loads(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsTc1Controller.Loads")
	defer span.End()

	cargues, err := controller.service.Loads(ctx, listaDelQuery(c, "periods"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"loads": cargues})
}

// Operators lista los operadores de red que reportan TC1.
//
// GET /liquidations/tc1/operators
func (controller LiquidationsTc1Controller) Operators(c *gin.Context) {
	c.JSON(http.StatusOK, controller.service.Operators())
}

// Status dice cuántos operadores ya cargaron su TC1 del período y cuáles faltan.
//
// GET /liquidations/tc1/status?period=2026-02
func (controller LiquidationsTc1Controller) Status(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsTc1Controller.Status")
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

// Periods lista los períodos con fronteras cargadas.
//
// GET /liquidations/tc1/periods
func (controller LiquidationsTc1Controller) Periods(c *gin.Context) {
	ctx := contextBia.RequestContext(c)
	ctx, span := tracing.StartSpan(ctx, "controllers.LiquidationsTc1Controller.Periods")
	defer span.End()

	periodos, err := controller.service.Periods(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginCommons.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, periodos)
}
