package tarifas_sdl

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"

	"bia-bills/models"
	"bia-bills/repositories"

	"github.com/biaenergy/bia-commons-go/tracing"
	"github.com/google/uuid"
)

// Lógica de negocio de Tarifas SDL.
//
// Flujo: la persona elige período en Nueva carga y sube los 33 archivos →
// Preview parsea, calcula y devuelve lo que se va a guardar → valida en pantalla
// → Confirm persiste en las dos bases.

// LoadMeta es lo que hace falta para que el cargue aparezca en el historial.
type LoadMeta struct {
	// Nombre para mostrar, lo manda el front. NO es una verificación de identidad.
	CreatedBy string
	// Id del header x-user-id, puesto server-side. Este es el confiable.
	CreatedByID string
}

// RatesJSON son las diez tarifas con los MISMOS nombres que las columnas de la
// tabla y que el endpoint de lectura.
//
// Existe para que el front tenga UNA sola forma: sin esto, el preview devolvía
// `rates.Activa.Nivel1Operador` y la lectura `active_level_1_operator`, o sea dos
// mapeos para el mismo dato.
type RatesJSON struct {
	ActiveLevel1Operator float64 `json:"active_level_1_operator"`
	ActiveLevel1Shared   float64 `json:"active_level_1_shared"`
	ActiveLevel1User     float64 `json:"active_level_1_user"`
	ActiveLevel2User     float64 `json:"active_level_2_user"`
	ActiveLevel3User     float64 `json:"active_level_3_user"`

	ReactiveLevel1Operator float64 `json:"reactive_level_1_operator"`
	ReactiveLevel1Shared   float64 `json:"reactive_level_1_shared"`
	ReactiveLevel1User     float64 `json:"reactive_level_1_user"`
	ReactiveLevel2User     float64 `json:"reactive_level_2_user"`
	ReactiveLevel3User     float64 `json:"reactive_level_3_user"`
}

// aRatesJSON pasa el resultado del motor a la forma que ve el front.
func aRatesJSON(t Tarifas) RatesJSON {
	return RatesJSON{
		ActiveLevel1Operator: t.Activa.Nivel1Operador,
		ActiveLevel1Shared:   t.Activa.Nivel1Compartido,
		ActiveLevel1User:     t.Activa.Nivel1Usuario,
		ActiveLevel2User:     t.Activa.Nivel2Usuario,
		ActiveLevel3User:     t.Activa.Nivel3Usuario,

		ReactiveLevel1Operator: t.Reactiva.Nivel1Operador,
		ReactiveLevel1Shared:   t.Reactiva.Nivel1Compartido,
		ReactiveLevel1User:     t.Reactiva.Nivel1Usuario,
		ReactiveLevel2User:     t.Reactiva.Nivel2Usuario,
		ReactiveLevel3User:     t.Reactiva.Nivel3Usuario,
	}
}

// PreviewRow es una fila de la previsualización: los componentes y las diez
// tarifas que saldrían de ellos.
//
// Van juntos a propósito: lo que la persona aprueba en pantalla es exactamente lo
// que se guarda en las dos tablas.
type PreviewRow struct {
	SdlInputRow
	Rates RatesJSON `json:"rates"`
}

// PreviewResult acompaña las filas con lo que hay que contarle al usuario.
type PreviewResult struct {
	Rows           []PreviewRow `json:"rows"`
	Warnings       []string     `json:"warnings"`
	CriticalErrors []string     `json:"critical_errors"`
}

// AuditFinding es un desacuerdo entre los insumos guardados y la tarifa guardada.
type AuditFinding struct {
	Period       string  `json:"period"`
	OperatorCode string  `json:"operator_code"`
	Column       string  `json:"column"`
	Stored       float64 `json:"stored"`
	Recomputed   float64 `json:"recomputed"`
}

// AuditResult es el resultado de verificar la coherencia entre las dos tablas.
type AuditResult struct {
	Checked  int            `json:"checked"`
	Findings []AuditFinding `json:"findings"`
	// Operadores con insumos guardados y sin tarifa, o al revés.
	Orphans []string `json:"orphans"`
}

type TarifasSdlService interface {
	// Preview parsea el lote y calcula. No escribe nada.
	Preview(ctx context.Context, files []UploadedFile) (PreviewResult, error)
	// Confirm persiste el lote y devuelve el id del cargue.
	Confirm(ctx context.Context, period string, rows []PreviewRow, meta LoadMeta) (string, error)

	// CurrentRates devuelve las tarifas vigentes.
	CurrentRates(ctx context.Context, filter repositories.SdlRateFilter) ([]repositories.SdlRate, error)
	// Periods lista los períodos con datos, más reciente primero.
	Periods(ctx context.Context) ([]string, error)
	// Loads devuelve el historial de cargues.
	Loads(ctx context.Context, periods []string) ([]repositories.SdlLoad, error)

	// Audit recalcula desde los insumos guardados y compara contra las tarifas
	// guardadas.
	Audit(ctx context.Context, periods []string) (AuditResult, error)
}

type tarifasSdlService struct {
	repository       repositories.LiquidationsSdlRepository
	agentsRepository repositories.LiquidationsAgentsRepository
}

func NewTarifasSdlService(
	repository repositories.LiquidationsSdlRepository,
	agentsRepository repositories.LiquidationsAgentsRepository,
) TarifasSdlService {
	return tarifasSdlService{repository: repository, agentsRepository: agentsRepository}
}

// completarNombres resuelve el nombre legal de cada fila contra public.agents.
//
// El código de agente lo trae el propio archivo; acá solo se traduce a nombre,
// con el MISMO filtro de actividad que usa Cargos STR. Eso es lo que hace que un
// operador se llame igual en las tablas de los dos módulos.
//
// Un agente que no resuelve es error, no advertencia: el nombre es lo que
// Finanzas ve, y guardar la fila sin él dejaría la pantalla mostrando un código.
func (service tarifasSdlService) completarNombres(
	ctx context.Context,
	filas []SdlInputRow,
) ([]SdlInputRow, error) {
	codigos := make([]string, 0, len(filas))
	vistos := map[string]bool{}
	for _, fila := range filas {
		codigo := AgentCodeFor(fila.AgentCode)
		if codigo != "" && !vistos[codigo] {
			codigos = append(codigos, codigo)
			vistos[codigo] = true
		}
	}

	nombres, err := service.agentsRepository.NamesByAgentCode(ctx, codigos)
	if err != nil {
		return nil, fmt.Errorf("no se pudo consultar el catálogo de agentes: %w", err)
	}

	sinNombre := []string{}
	for i := range filas {
		codigo := AgentCodeFor(filas[i].AgentCode)
		nombre, hay := nombres[codigo]
		if !hay || nombre == "" {
			sinNombre = append(sinNombre, fmt.Sprintf("%s (agente %s)", filas[i].OperatorCode, codigo))
			continue
		}
		filas[i].OperatorName = nombre
	}

	if len(sinNombre) > 0 {
		sort.Strings(sinNombre)
		return nil, fmt.Errorf(
			"estos operadores no tienen un agente con actividad %q en el catálogo: %s",
			"OPERADOR DE RED", strings.Join(sinNombre, ", "))
	}

	return filas, nil
}

func (service tarifasSdlService) Preview(
	ctx context.Context,
	files []UploadedFile,
) (PreviewResult, error) {
	_, span := tracing.StartSpan(ctx, "services.tarifas_sdl.Preview")
	defer span.End()

	parseado := ParseInputs(files)
	resultado := PreviewResult{
		Rows:           []PreviewRow{},
		Warnings:       parseado.Warnings,
		CriticalErrors: parseado.CriticalErrors,
	}
	if len(parseado.CriticalErrors) > 0 {
		return resultado, nil
	}

	conNombre, err := service.completarNombres(ctx, parseado.Rows)
	if err != nil {
		// No se degrada con aviso como en el preview de Cargos STR: acá el nombre
		// distingue a los operadores que comparten razón social, así que sin él la
		// pantalla es ambigua.
		resultado.CriticalErrors = append(resultado.CriticalErrors, err.Error())
		return resultado, nil
	}

	for _, fila := range conNombre {
		componentes, err := ComponentesDe(fila)
		if err != nil {
			resultado.CriticalErrors = append(resultado.CriticalErrors, err.Error())
			continue
		}
		resultado.Rows = append(resultado.Rows, PreviewRow{
			SdlInputRow: fila,
			Rates:       aRatesJSON(Calcular(componentes)),
		})
	}

	if len(resultado.CriticalErrors) > 0 {
		resultado.Rows = []PreviewRow{}
	}

	return resultado, nil
}

// Confirm persiste el lote en las dos bases.
//
// No hay transacción que abarque ambas, así que el orden importa:
//
//  1. file-compiler (los componentes)
//  2. calculator-prices (las tarifas, que es lo que la pantalla muestra)
//
// Si la segunda falla, se borran las filas de la primera por load_id. Así nunca
// queda un insumo sin su tarifa ni una tarifa a medias visible. Como el modelo es
// append-only, reintentar con un load_id nuevo es seguro.
func (service tarifasSdlService) Confirm(
	ctx context.Context,
	period string,
	rows []PreviewRow,
	meta LoadMeta,
) (string, error) {
	ctx, span := tracing.StartSpan(ctx, "services.tarifas_sdl.Confirm")
	defer span.End()

	if strings.TrimSpace(period) == "" {
		return "", fmt.Errorf("falta el período del cargue")
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("el lote no tiene filas para guardar")
	}

	// Que estén los 21. Guardar menos dejaría operadores con la tarifa del período
	// anterior sin que nadie lo note, por el modelo append-only.
	if err := validarCobertura(rows); err != nil {
		return "", err
	}

	// El nombre se resuelve acá y NO se toma de lo que mandó el navegador: es lo
	// que va a ver Finanzas.
	filas := make([]SdlInputRow, 0, len(rows))
	for _, r := range rows {
		filas = append(filas, r.SdlInputRow)
	}
	conNombre, err := service.completarNombres(ctx, filas)
	if err != nil {
		return "", err
	}
	nombrePorOperador := map[string]string{}
	for _, f := range conNombre {
		nombrePorOperador[f.OperatorCode] = f.OperatorName
	}

	loadID := uuid.NewString()

	insumos := make([]models.LiquidationsSdlInput, 0, len(rows))
	tarifas := make([]models.LiquidationsSdlRate, 0, len(rows))

	for _, fila := range rows {
		// Las tarifas NO se toman de lo que mandó el navegador: se recalculan acá
		// desde los componentes. Es lo que garantiza que la tarifa guardada sea
		// consecuencia de los insumos guardados, que es toda la premisa de la
		// auditoría.
		componentes, err := ComponentesDe(fila.SdlInputRow)
		if err != nil {
			return "", err
		}
		calculadas := Calcular(componentes)

		var area *string
		if fila.DistributionArea != "" {
			valor := fila.DistributionArea
			area = &valor
		}

		insumos = append(insumos, models.LiquidationsSdlInput{
			LoadID:           loadID,
			Period:           period,
			OperatorCode:     fila.OperatorCode,
			OperatorName:     nombrePorOperador[fila.OperatorCode],
			AgentCode:        fila.AgentCode,
			Market:           fila.Market,
			DistributionArea: area,
			DT1Add:           fila.DT1Add,
			DT2Add:           fila.DT2Add,
			DT3Add:           fila.DT3Add,
			DT1:              fila.DT1,
			DT2:              fila.DT2,
			DT3:              fila.DT3,
			CDI:              fila.CDI,
			CDN4:             fila.CDN4,
			PR1:              fila.PR1,
			PR2:              fila.PR2,
			PR3:              fila.PR3,
			SourceFiles:      strings.Join(fila.SourceFiles, ", "),
			CreatedBy:        meta.CreatedBy,
			CreatedByID:      meta.CreatedByID,
		})

		tarifas = append(tarifas, models.LiquidationsSdlRate{
			LoadID:       loadID,
			Period:       period,
			OperatorCode: fila.OperatorCode,
			OperatorName: nombrePorOperador[fila.OperatorCode],
			AgentCode:    fila.AgentCode,
			Market:       fila.Market,

			ActiveLevel1Operator: calculadas.Activa.Nivel1Operador,
			ActiveLevel1Shared:   calculadas.Activa.Nivel1Compartido,
			ActiveLevel1User:     calculadas.Activa.Nivel1Usuario,
			ActiveLevel2User:     calculadas.Activa.Nivel2Usuario,
			ActiveLevel3User:     calculadas.Activa.Nivel3Usuario,

			ReactiveLevel1Operator: calculadas.Reactiva.Nivel1Operador,
			ReactiveLevel1Shared:   calculadas.Reactiva.Nivel1Compartido,
			ReactiveLevel1User:     calculadas.Reactiva.Nivel1Usuario,
			ReactiveLevel2User:     calculadas.Reactiva.Nivel2Usuario,
			ReactiveLevel3User:     calculadas.Reactiva.Nivel3Usuario,
		})
	}

	if err := service.repository.InsertInputs(ctx, insumos); err != nil {
		return "", fmt.Errorf("no se pudieron guardar los componentes: %w", err)
	}

	if err := service.repository.InsertRates(ctx, tarifas); err != nil {
		// Limpieza: el cargue no llegó a existir, no debe quedar insumo huérfano.
		if errBorrado := service.repository.DeleteInputsByLoad(ctx, loadID); errBorrado != nil {
			log.Printf("tarifas_sdl: falló el rollback de los componentes del cargue %s, hay que borrarlos a mano: %v",
				loadID, errBorrado)
		}
		return "", fmt.Errorf("no se pudieron guardar las tarifas: %w", err)
	}

	return loadID, nil
}

func validarCobertura(rows []PreviewRow) error {
	presentes := map[string]bool{}
	for _, r := range rows {
		presentes[r.OperatorCode] = true
	}

	faltan := []string{}
	for _, codigo := range OperatorCodes() {
		if !presentes[codigo] {
			faltan = append(faltan, codigo)
		}
	}
	if len(faltan) > 0 {
		sort.Strings(faltan)
		return fmt.Errorf(
			"el lote no cubre a todos los operadores de red; faltan %d: %s",
			len(faltan), strings.Join(faltan, ", "))
	}

	return nil
}

func (service tarifasSdlService) CurrentRates(
	ctx context.Context,
	filter repositories.SdlRateFilter,
) ([]repositories.SdlRate, error) {
	ctx, span := tracing.StartSpan(ctx, "services.tarifas_sdl.CurrentRates")
	defer span.End()

	return service.repository.CurrentRates(ctx, filter)
}

func (service tarifasSdlService) Periods(ctx context.Context) ([]string, error) {
	ctx, span := tracing.StartSpan(ctx, "services.tarifas_sdl.Periods")
	defer span.End()

	return service.repository.PeriodsWithRates(ctx)
}

func (service tarifasSdlService) Loads(
	ctx context.Context,
	periods []string,
) ([]repositories.SdlLoad, error) {
	ctx, span := tracing.StartSpan(ctx, "services.tarifas_sdl.Loads")
	defer span.End()

	return service.repository.Loads(ctx, periods)
}

// Audit lee los componentes guardados, recalcula y compara contra las tarifas
// guardadas.
//
// Es la comprobación que demuestra que las dos tablas están de verdad
// relacionadas y no solo dicen estarlo. Si alguna vez una tarifa se guardara mal
// —o alguien la editara a mano— esto lo encuentra.
//
// Tolerancia: 1e-9. Las dos cifras salen del mismo cálculo en float64, así que la
// diferencia esperable es del orden del épsilon de la máquina, no de redondeo.
func (service tarifasSdlService) Audit(ctx context.Context, periods []string) (AuditResult, error) {
	ctx, span := tracing.StartSpan(ctx, "services.tarifas_sdl.Audit")
	defer span.End()

	const tolerancia = 1e-9

	insumos, err := service.repository.CurrentInputs(ctx, periods)
	if err != nil {
		return AuditResult{}, fmt.Errorf("no se pudieron leer los componentes: %w", err)
	}

	tarifas, err := service.repository.CurrentRates(ctx, repositories.SdlRateFilter{Periods: periods})
	if err != nil {
		return AuditResult{}, fmt.Errorf("no se pudieron leer las tarifas: %w", err)
	}

	guardadas := map[string]repositories.SdlRate{}
	for _, t := range tarifas {
		guardadas[t.Period+"|"+t.OperatorCode] = t
	}

	res := AuditResult{Findings: []AuditFinding{}, Orphans: []string{}}
	vistas := map[string]bool{}

	for _, insumo := range insumos {
		clave := insumo.Period + "|" + insumo.OperatorCode
		vistas[clave] = true

		guardada, existe := guardadas[clave]
		if !existe {
			res.Orphans = append(res.Orphans, clave+" (componentes sin tarifa)")
			continue
		}

		fila := SdlInputRow{
			OperatorCode: insumo.OperatorCode,
			DT1Add:       insumo.DT1Add, DT2Add: insumo.DT2Add, DT3Add: insumo.DT3Add,
			DT1: insumo.DT1, DT2: insumo.DT2, DT3: insumo.DT3,
			CDI: insumo.CDI, CDN4: insumo.CDN4,
			PR1: insumo.PR1, PR2: insumo.PR2, PR3: insumo.PR3,
		}
		if insumo.DistributionArea != nil {
			fila.DistributionArea = *insumo.DistributionArea
		}

		componentes, err := ComponentesDe(fila)
		if err != nil {
			res.Findings = append(res.Findings, AuditFinding{
				Period: insumo.Period, OperatorCode: insumo.OperatorCode,
				Column: "componentes: " + err.Error(),
			})
			continue
		}

		recalculadas := Calcular(componentes)
		res.Checked++

		comparaciones := []struct {
			columna  string
			guardada float64
			nueva    float64
		}{
			{"active_level_1_operator", guardada.ActiveLevel1Operator, recalculadas.Activa.Nivel1Operador},
			{"active_level_1_shared", guardada.ActiveLevel1Shared, recalculadas.Activa.Nivel1Compartido},
			{"active_level_1_user", guardada.ActiveLevel1User, recalculadas.Activa.Nivel1Usuario},
			{"active_level_2_user", guardada.ActiveLevel2User, recalculadas.Activa.Nivel2Usuario},
			{"active_level_3_user", guardada.ActiveLevel3User, recalculadas.Activa.Nivel3Usuario},
			{"reactive_level_1_operator", guardada.ReactiveLevel1Operator, recalculadas.Reactiva.Nivel1Operador},
			{"reactive_level_1_shared", guardada.ReactiveLevel1Shared, recalculadas.Reactiva.Nivel1Compartido},
			{"reactive_level_1_user", guardada.ReactiveLevel1User, recalculadas.Reactiva.Nivel1Usuario},
			{"reactive_level_2_user", guardada.ReactiveLevel2User, recalculadas.Reactiva.Nivel2Usuario},
			{"reactive_level_3_user", guardada.ReactiveLevel3User, recalculadas.Reactiva.Nivel3Usuario},
		}

		for _, c := range comparaciones {
			if math.Abs(c.guardada-c.nueva) > tolerancia {
				res.Findings = append(res.Findings, AuditFinding{
					Period: insumo.Period, OperatorCode: insumo.OperatorCode,
					Column: c.columna, Stored: c.guardada, Recomputed: c.nueva,
				})
			}
		}
	}

	// Tarifas sin componentes: no se pueden verificar.
	for clave := range guardadas {
		if !vistas[clave] {
			res.Orphans = append(res.Orphans, clave+" (tarifa sin componentes)")
		}
	}
	sort.Strings(res.Orphans)

	return res, nil
}
