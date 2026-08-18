package cargos_str

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"bia-bills/models"
	"bia-bills/repositories"

	"github.com/biaenergy/bia-commons-go/tracing"
	"github.com/google/uuid"
)

// Lógica de negocio de Cargos STR.
//
// Flujo: el usuario elige período y sube los BalanceSTR → Preview parsea y
// devuelve lo que se va a guardar → el usuario valida en pantalla → Confirm
// persiste en las dos bases.

type CargosStrService interface {
	// Preview parsea el lote y completa los nombres. No escribe nada.
	Preview(ctx context.Context, files []UploadedFile, year, month int) (ParseResult, error)
	// Confirm persiste el lote y devuelve el id de la carga.
	Confirm(ctx context.Context, rows []StrRow, meta LoadMeta) (string, error)
	// Loads devuelve el historial de cargues, más reciente primero.
	Loads(ctx context.Context, periods []string) ([]repositories.StrLoad, error)
	// CurrentCharges devuelve el valor a pagar vigente por (período, operador).
	CurrentCharges(ctx context.Context, filter repositories.StrChargeFilter) ([]repositories.StrCharge, error)
	// TotalsByPeriod suma lo vigente de cada período.
	TotalsByPeriod(ctx context.Context, periods []string) (map[string]float64, error)
	// Periods lista los períodos con datos, más reciente primero.
	Periods(ctx context.Context) ([]string, error)
}

// LoadMeta es lo que hace falta para que el cargue aparezca en el historial.
//
// Antes esto vivía en Supabase, escrito por el backend TypeScript. Al salir
// Supabase del circuito, sin estos datos el módulo de Cargas se queda sin
// historial ni estado del período.
type LoadMeta struct {
	// Nombre para mostrar. Lo manda el front, así que NO es una verificación de
	// identidad; sirve para que el historial sea legible.
	CreatedBy string
	// Id del header x-user-id, puesto server-side. Este es el confiable.
	CreatedByID string
	// Nombres de los archivos del lote, en el orden en que llegaron.
	SourceFiles []string
}

type cargosStrService struct {
	strRepository    repositories.LiquidationsStrRepository
	agentsRepository repositories.LiquidationsAgentsRepository
}

func NewCargosStrService(
	strRepository repositories.LiquidationsStrRepository,
	agentsRepository repositories.LiquidationsAgentsRepository,
) CargosStrService {
	return cargosStrService{
		strRepository:    strRepository,
		agentsRepository: agentsRepository,
	}
}

func (service cargosStrService) Preview(
	ctx context.Context,
	files []UploadedFile,
	year, month int,
) (ParseResult, error) {
	ctx, span := tracing.StartSpan(ctx, "services.cargos_str.Preview")
	defer span.End()

	resultado := ParseStrInputs(files, year, month)
	if len(resultado.CriticalErrors) > 0 {
		return resultado, nil
	}

	// El nombre sale del catálogo de agentes. Si ese catálogo no responde, el
	// preview se muestra igual con una advertencia: lo que el usuario valida en
	// pantalla son los montos, y bloquearlo por un nombre sería peor.
	nombres, err := service.agentsRepository.NamesByOperator(ctx, Homologation())
	if err != nil {
		resultado.Warnings = append(resultado.Warnings, fmt.Sprintf(
			"No se pudo consultar el catálogo de agentes para los nombres: %v. Los montos no se ven afectados.", err))
		return resultado, nil
	}

	var sinNombre []string
	for i := range resultado.Rows {
		nombre, ok := nombres[resultado.Rows[i].OperatorCode]
		if !ok {
			sinNombre = append(sinNombre, resultado.Rows[i].OperatorCode)
			nombre = resultado.Rows[i].OperatorCode // se muestra el código como fallback
		}
		resultado.Rows[i].OperatorName = nombre
	}

	if len(sinNombre) > 0 {
		sort.Strings(sinNombre)
		resultado.Warnings = append(resultado.Warnings, fmt.Sprintf(
			"Sin nombre en el catálogo de agentes (se usa el código): %s.", strings.Join(sinNombre, ", ")))
	}

	return resultado, nil
}

// Confirm persiste el lote en las dos bases.
//
// No hay transacción que abarque ambas, así que el orden importa:
//
//  1. file-compiler (el desglose)
//  2. calculator-prices (el valor a pagar, que es lo que la matriz muestra)
//
// Si la segunda falla, se borran las filas de la primera por load_id. Así nunca
// queda un insumo sin su resultado, ni un resultado a medias visible en la
// matriz. Como el modelo es append-only, reintentar con un load_id nuevo es
// seguro.
func (service cargosStrService) Confirm(ctx context.Context, rows []StrRow, meta LoadMeta) (string, error) {
	ctx, span := tracing.StartSpan(ctx, "services.cargos_str.Confirm")
	defer span.End()

	if len(rows) == 0 {
		return "", fmt.Errorf("el lote no tiene filas para guardar")
	}

	loadID := uuid.NewString()

	// El nombre se resuelve acá y NO se toma de lo que mandó el navegador:
	// operator_name es NOT NULL y es lo que va a ver Finanzas.
	nombres, err := service.agentsRepository.NamesByOperator(ctx, Homologation())
	if err != nil {
		return "", fmt.Errorf("no se pudo consultar el catálogo de agentes: %w", err)
	}

	var sinNombre []string
	for _, r := range rows {
		if _, ok := nombres[r.OperatorCode]; !ok {
			sinNombre = append(sinNombre, r.OperatorCode)
		}
	}
	if len(sinNombre) > 0 {
		sort.Strings(sinNombre)
		return "", fmt.Errorf(
			"no se pudo resolver el nombre de estos operadores en el catálogo de agentes: %s",
			strings.Join(sinNombre, ", "))
	}

	insumos := make([]models.LiquidationsStrInput, 0, len(rows))
	cargos := make([]models.LiquidationsStrCharge, 0, len(rows))
	for _, r := range rows {
		insumos = append(insumos, models.LiquidationsStrInput{
			LoadID:           loadID,
			Period:           r.Period,
			OperatorCode:     r.OperatorCode,
			InvoiceAmount:    r.InvoiceAmount,
			Reinvoice1Amount: r.Reinvoice1Amount,
			Reinvoice2Amount: r.Reinvoice2Amount,
			Reinvoice3Amount: r.Reinvoice3Amount,
			CreatedBy:        meta.CreatedBy,
			CreatedByID:      meta.CreatedByID,
			SourceFiles:      strings.Join(meta.SourceFiles, ", "),
		})
		cargos = append(cargos, models.LiquidationsStrCharge{
			LoadID:        loadID,
			Period:        r.Period,
			OperatorCode:  r.OperatorCode,
			OperatorName:  nombres[r.OperatorCode],
			AmountPayable: r.AmountPayable,
		})
	}

	if err := service.strRepository.InsertInputs(ctx, insumos); err != nil {
		return "", fmt.Errorf("no se pudo guardar el insumo: %w", err)
	}

	if err := service.strRepository.InsertCharges(ctx, cargos); err != nil {
		// Limpieza: la carga no llegó a existir, no debe quedar insumo huérfano.
		if errBorrado := service.strRepository.DeleteInputsByLoad(ctx, loadID); errBorrado != nil {
			log.Printf("cargos_str: falló el rollback del insumo de la carga %s, hay que borrarlo a mano: %v",
				loadID, errBorrado)
		}
		return "", fmt.Errorf("no se pudo guardar el resultado: %w", err)
	}

	return loadID, nil
}

func (service cargosStrService) CurrentCharges(
	ctx context.Context,
	filter repositories.StrChargeFilter,
) ([]repositories.StrCharge, error) {
	ctx, span := tracing.StartSpan(ctx, "services.cargos_str.CurrentCharges")
	defer span.End()

	return service.strRepository.CurrentCharges(ctx, filter)
}

func (service cargosStrService) TotalsByPeriod(
	ctx context.Context,
	periods []string,
) (map[string]float64, error) {
	ctx, span := tracing.StartSpan(ctx, "services.cargos_str.TotalsByPeriod")
	defer span.End()

	return service.strRepository.TotalsByPeriod(ctx, periods)
}

func (service cargosStrService) Loads(
	ctx context.Context,
	periods []string,
) ([]repositories.StrLoad, error) {
	ctx, span := tracing.StartSpan(ctx, "services.cargos_str.Loads")
	defer span.End()

	return service.strRepository.Loads(ctx, periods)
}

func (service cargosStrService) Periods(ctx context.Context) ([]string, error) {
	ctx, span := tracing.StartSpan(ctx, "services.cargos_str.Periods")
	defer span.End()

	return service.strRepository.PeriodsWithCharges(ctx)
}
