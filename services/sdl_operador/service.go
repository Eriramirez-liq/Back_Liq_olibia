// Package sdl_operador resuelve la preliquidación SDL que cada operador de red
// envía por período.
//
// ⚠️ TRASVASE: paquete propio bajo services/. Ver docs/backend/migracion-a-go.md.
package sdl_operador

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"bia-bills/models"
	"bia-bills/repositories"

	"github.com/biaenergy/bia-commons-go/tracing"
	"github.com/google/uuid"
)

// ── Por qué este servicio no parsea nada ────────────────────────────────────
// El archivo se parsea en el NAVEGADOR, igual que en TC1. Acá no es por el peso
// —los de SDL son chicos— sino por el parser: son 2.000 líneas con una rareza
// por operador, ya escritas y probadas en TypeScript. Portarlas a Go sería
// reescribirlas, y ahí es donde viven los errores. Se copian y el test dorado
// contra los archivos reales prueba que dan lo mismo.
//
// Este servicio entonces recibe filas ya normalizadas: valida lo que el
// navegador no puede validar solo —que el período tenga forma, que el operador
// esté en el catálogo, que el período del archivo cuadre con el elegido— y las
// guarda.

// Row es una frontera ya normalizada por el parser del front. Los nombres JSON
// son los mismos que las columnas de la tabla: lo que viaja y lo que se guarda
// se llaman igual, sin traducción de por medio.
type Row struct {
	Sic        string  `json:"sic"`
	Name       *string `json:"name"`
	FilePeriod *string `json:"file_period"`

	ActiveEnergy *float64 `json:"active_energy"`

	InductiveTotal      *float64 `json:"inductive_total"`
	InductivePenalized  *float64 `json:"inductive_penalized"`
	CapacitiveTotal     *float64 `json:"capacitive_total"`
	CapacitivePenalized *float64 `json:"capacitive_penalized"`

	FactorM  *float64 `json:"factor_m"`
	CounterM *float64 `json:"counter_m"`

	TensionLevel   *string `json:"tension_level"`
	AssetOwnership *string `json:"asset_ownership"`

	ActiveRate   *float64 `json:"active_rate"`
	ReactiveRate *float64 `json:"reactive_rate"`

	IsDuplicate bool `json:"is_duplicate"`
}

// Status dice cuántos operadores ya cargaron su preliquidación en un período y
// cuáles faltan. Es lo que alimenta el chip "N/21" del estado del período.
type Status struct {
	Period   string   `json:"period"`
	Expected []string `json:"expected"`
	Loaded   []string `json:"loaded"`
	Pending  []string `json:"pending"`
}

// LoadMeta es la traza del cargue.
type LoadMeta struct {
	CreatedBy   string
	CreatedByID string
	SourceFile  string
}

type SdlOperadorService interface {
	// Confirm guarda las fronteras de un cargue y devuelve su id.
	Confirm(ctx context.Context, period, operatorCode string, rows []Row, meta LoadMeta) (string, error)
	// CurrentRows devuelve las fronteras vigentes.
	CurrentRows(ctx context.Context, filter repositories.SdlPreliqFilter) ([]models.LiquidationsSdlPreliquidation, error)
	// Loads lista el historial de cargues.
	Loads(ctx context.Context, periods []string) ([]repositories.SdlPreliqLoad, error)
	// Periods lista los períodos con datos.
	Periods(ctx context.Context) ([]string, error)
	// Status dice qué operadores ya cargaron y cuáles faltan.
	Status(ctx context.Context, period string) (Status, error)
	// Operators lista los operadores que reportan preliquidación SDL.
	Operators() []string
}

type sdlOperadorService struct {
	repository repositories.LiquidationsSdlPreliqRepository
	// Los operadores que se esperan por período. Entra como dato y no como
	// dependencia de otro paquete: el servicio no tiene por qué saber de dónde
	// sale el catálogo, solo cuál es. Son los mismos que reportan TC1.
	expected []string
}

func NewSdlOperadorService(
	repository repositories.LiquidationsSdlPreliqRepository,
	expected []string,
) SdlOperadorService {
	return sdlOperadorService{repository: repository, expected: expected}
}

// El mismo formato que guardan Cargos STR, Tarifas SDL y TC1. La base tiene el
// mismo CHECK; acá se valida antes para dar un mensaje entendible en vez de un
// error de Postgres.
var formatoPeriodo = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

func (service sdlOperadorService) Operators() []string {
	return service.expected
}

func (service sdlOperadorService) Confirm(
	ctx context.Context,
	period, operatorCode string,
	rows []Row,
	meta LoadMeta,
) (string, error) {
	ctx, span := tracing.StartSpan(ctx, "services.sdl_operador.Confirm")
	defer span.End()

	period = strings.TrimSpace(period)
	operatorCode = strings.ToUpper(strings.TrimSpace(operatorCode))

	if !formatoPeriodo.MatchString(period) {
		return "", fmt.Errorf(
			"el período %q no tiene la forma AAAA-MM (por ejemplo 2026-07)", period)
	}
	if operatorCode == "" {
		return "", fmt.Errorf("falta el operador de red del archivo")
	}
	if !service.esperado(operatorCode) {
		return "", fmt.Errorf(
			"el operador %q no está entre los que reportan preliquidación SDL", operatorCode)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("el cargue no trae ninguna frontera")
	}

	// El período que trae el archivo tiene que cuadrar con el elegido.
	//
	// Misma regla que en Cargos STR e Insumos Tarifas SDL: si no coincide se
	// BLOQUEA, y si el archivo no lo trae se deja pasar —hay operadores que no lo
	// reportan, CHEC entre ellos—. Cargar un mes en el período de otro es el error
	// más caro de esta pantalla: no falla nada y la conciliación de dos meses
	// queda mal.
	if err := validarPeriodoDelArchivo(period, rows); err != nil {
		return "", err
	}

	// La frontera vacía sí se rechaza: es la clave con la que después se cruza
	// contra Facturación y XM, y una fila sin ella no la va a poder conciliar
	// nadie. La base la rechaza con un CHECK, pero cortar acá dice CUÁL fila está
	// mal en vez de tumbar el lote entero.
	//
	// La frontera REPETIDA, en cambio, no se rechaza: el archivo puede traerla
	// dos veces y eso se marca en `is_duplicate` para que Congruencia la excluya.
	// Es la diferencia con TC1, donde una repetida sí corta el cargue.
	for i, r := range rows {
		if strings.TrimSpace(r.Sic) == "" {
			return "", fmt.Errorf(
				"la fila %d no trae código de frontera, que es con lo que se cruza contra "+
					"Facturación y XM", i+1)
		}
	}

	loadID := uuid.NewString()
	filas := make([]models.LiquidationsSdlPreliquidation, 0, len(rows))
	for _, r := range rows {
		filas = append(filas, aModelo(r, loadID, period, operatorCode, meta))
	}

	if err := service.repository.InsertRows(ctx, filas); err != nil {
		return "", fmt.Errorf("no se pudieron guardar las fronteras: %w", err)
	}
	return loadID, nil
}

// validarPeriodoDelArchivo corta si el archivo dice de otro mes.
//
// Se listan hasta tres fronteras y se resume el resto: con miles de filas, un
// mensaje con todas no se lee, y con ninguna no se sabe dónde mirar.
func validarPeriodoDelArchivo(period string, rows []Row) error {
	distintos := map[string][]string{}
	for _, r := range rows {
		if r.FilePeriod == nil {
			continue
		}
		p := strings.TrimSpace(*r.FilePeriod)
		if p == "" || p == period {
			continue
		}
		distintos[p] = append(distintos[p], r.Sic)
	}
	if len(distintos) == 0 {
		return nil
	}

	partes := make([]string, 0, len(distintos))
	for p, fronteras := range distintos {
		muestra := fronteras
		resto := 0
		if len(muestra) > 3 {
			resto = len(muestra) - 3
			muestra = muestra[:3]
		}
		detalle := strings.Join(muestra, ", ")
		if resto > 0 {
			detalle = fmt.Sprintf("%s y %d más", detalle, resto)
		}
		partes = append(partes, fmt.Sprintf("%s (%s)", p, detalle))
	}

	return fmt.Errorf(
		"el archivo dice ser del período %s, y el seleccionado es %s. "+
			"Revisá el período del módulo o el archivo antes de cargar",
		strings.Join(partes, "; "), period)
}

func (service sdlOperadorService) esperado(codigo string) bool {
	for _, e := range service.expected {
		if e == codigo {
			return true
		}
	}
	return false
}

func (service sdlOperadorService) CurrentRows(
	ctx context.Context,
	filter repositories.SdlPreliqFilter,
) ([]models.LiquidationsSdlPreliquidation, error) {
	ctx, span := tracing.StartSpan(ctx, "services.sdl_operador.CurrentRows")
	defer span.End()
	return service.repository.CurrentRows(ctx, filter)
}

func (service sdlOperadorService) Loads(
	ctx context.Context,
	periods []string,
) ([]repositories.SdlPreliqLoad, error) {
	ctx, span := tracing.StartSpan(ctx, "services.sdl_operador.Loads")
	defer span.End()
	return service.repository.Loads(ctx, periods)
}

func (service sdlOperadorService) Periods(ctx context.Context) ([]string, error) {
	ctx, span := tracing.StartSpan(ctx, "services.sdl_operador.Periods")
	defer span.End()
	return service.repository.Periods(ctx)
}

// Status cruza los operadores esperados contra los que ya cargaron.
//
// Lo que se cuenta son OPERADORES, no cargues: si alguien vuelve a cargar el
// archivo de CHEC, sigue siendo uno de los 21 y no dos.
func (service sdlOperadorService) Status(ctx context.Context, period string) (Status, error) {
	ctx, span := tracing.StartSpan(ctx, "services.sdl_operador.Status")
	defer span.End()

	period = strings.TrimSpace(period)
	if !formatoPeriodo.MatchString(period) {
		return Status{}, fmt.Errorf(
			"el período %q no tiene la forma AAAA-MM (por ejemplo 2026-07)", period)
	}

	cargues, err := service.repository.Loads(ctx, []string{period})
	if err != nil {
		return Status{}, err
	}

	yaCargaron := make(map[string]bool, len(cargues))
	for _, c := range cargues {
		yaCargaron[c.OperatorCode] = true
	}

	// Se recorre `expected` y no el mapa: el orden de un mapa en Go es aleatorio,
	// y una lista que cambia de orden en cada consulta es incómoda de leer.
	estado := Status{
		Period:   period,
		Expected: service.expected,
		Loaded:   []string{},
		Pending:  []string{},
	}
	for _, operador := range service.expected {
		if yaCargaron[operador] {
			estado.Loaded = append(estado.Loaded, operador)
		} else {
			estado.Pending = append(estado.Pending, operador)
		}
	}
	return estado, nil
}

func aModelo(
	r Row,
	loadID, period, operatorCode string,
	meta LoadMeta,
) models.LiquidationsSdlPreliquidation {
	opcional := func(s string) *string {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		return &s
	}

	return models.LiquidationsSdlPreliquidation{
		// La tabla no tiene default para el id: lo pone el servicio.
		ID:           uuid.NewString(),
		LoadID:       loadID,
		Period:       period,
		FilePeriod:   r.FilePeriod,
		OperatorCode: operatorCode,
		Sic:          strings.TrimSpace(r.Sic),
		Name:         r.Name,

		ActiveEnergy: r.ActiveEnergy,

		InductiveTotal:      r.InductiveTotal,
		InductivePenalized:  r.InductivePenalized,
		CapacitiveTotal:     r.CapacitiveTotal,
		CapacitivePenalized: r.CapacitivePenalized,

		FactorM:  r.FactorM,
		CounterM: r.CounterM,

		TensionLevel:   r.TensionLevel,
		AssetOwnership: r.AssetOwnership,

		ActiveRate:   r.ActiveRate,
		ReactiveRate: r.ReactiveRate,

		IsDuplicate: r.IsDuplicate,

		SourceFile:  opcional(meta.SourceFile),
		CreatedBy:   opcional(meta.CreatedBy),
		CreatedByID: opcional(meta.CreatedByID),
	}
}
