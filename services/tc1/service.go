package tc1

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

// Servicio de TC1.
//
// ⚠️ TRASVASE: archivo del paquete tc1. Ver docs/backend/migracion-a-go.md.
//
// ── Por qué este servicio no parsea nada ────────────────────────────────────
// El archivo de TC1 se parsea en el NAVEGADOR, no acá, y no es un capricho:
// el de CELSIA_VALLE pesa 98 MB con 743.530 filas y el de CHEC 82 MB. El parser
// filtra por ID_COMERCIALIZADOR = 62371 (BIA) y de esas 743.530 quedan 156, así
// que lo que llega al servidor es chico — pero solo porque el filtrado ocurre
// ANTES de subir. Mandar el archivo crudo sería empujar 98 MB por el gateway.
//
// Este servicio entonces recibe filas ya normalizadas: valida lo que no puede
// validar el navegador —que el período tenga forma, que el operador esté en el
// catálogo, que las fronteras no vengan vacías ni repetidas— y las guarda.

// Row es una frontera comercial ya normalizada por el parser del front.
//
// Los nombres JSON son las 33 columnas canónicas de TC1 tal como las produce el
// parser. Todo string: es lo que la tabla guarda y lo que el archivo trae.
type Row struct {
	OperatorCode string `json:"operator_code"`

	Niu                       string `json:"NIU"`
	CodigoDeConexion          string `json:"CODIGO_DE_CONEXION"`
	TipoDeConexion            string `json:"TIPO_DE_CONEXION"`
	NivelDeTension            string `json:"NIVEL_DE_TENSION"`
	NivelDeTensionPrimario    string `json:"NIVEL_DE_TENSION_PRIMARIO"`
	PorcPropiedadDelActivo    string `json:"PORC_PROPIEDAD_DEL_ACTIVO"`
	ConexionRed               string `json:"CONEXION_RED"`
	IDComercializador         string `json:"ID_COMERCIALIZADOR"`
	IDMercado                 string `json:"ID_MERCADO"`
	GrupoDeCalidad            string `json:"GRUPO_DE_CALIDAD"`
	CodFronteraComercial      string `json:"COD_FRONTERA_COMERCIAL"`
	CodigoCircuitoOLinea      string `json:"CODIGO_CIRCUITO_O_LINEA"`
	CodigoTransformador       string `json:"CODIGO_TRANSFORMADOR"`
	CodigoDaneNiu             string `json:"CODIGO_DANE_NIU"`
	Ubicacion                 string `json:"UBICACION"`
	Direccion                 string `json:"DIRECCION"`
	CondicionEspecial         string `json:"CONDICION_ESPECIAL"`
	TipoAreaEspecial          string `json:"TIPO_AREA_ESPECIAL"`
	CodigoAreaEspecial        string `json:"CODIGO_AREA_ESPECIAL"`
	EstratoID                 string `json:"ESTRATO_ID"`
	Altitud                   string `json:"ALTITUD"`
	Longitud                  string `json:"LONGITUD"`
	Latitud                   string `json:"LATITUD"`
	Autogenerador             string `json:"AUTOGENERADOR"`
	ExportaEnergia            string `json:"EXPORTA_ENERGIA"`
	Potencia                  string `json:"POTENCIA"`
	TipoGeneracion            string `json:"TIPO_GENERACION"`
	CodigoFronteraAutoGen     string `json:"CODIGO_FRONTERA_AUTO_GEN"`
	InicioOperacion           string `json:"INICIO_OPERACION"`
	ContratoRespaldo          string `json:"CONTRATO_RESPALDO"`
	CapacidadContratoRespaldo string `json:"CAPACIDAD_CONTRATO_RESPALDO"`
	Ciclo                     string `json:"CICLO"`
	Nodo                      string `json:"NODO"`
}

// Tc1Status dice cuántos operadores cargaron su TC1 en un período y cuáles
// faltan.
type Tc1Status struct {
	Period string `json:"period"`
	// Los que se esperan. Son los 21 operadores de red que reportan TC1.
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

type Tc1Service interface {
	// Confirm guarda las fronteras de un cargue y devuelve su id.
	Confirm(ctx context.Context, period, operatorCode string, rows []Row, meta LoadMeta) (string, error)
	// CurrentInputs devuelve las fronteras vigentes.
	CurrentInputs(ctx context.Context, filter repositories.Tc1Filter) ([]models.LiquidationsTc1Input, error)
	// Loads lista el historial de cargues.
	Loads(ctx context.Context, periods []string) ([]repositories.Tc1Load, error)
	// Periods lista los períodos con datos.
	Periods(ctx context.Context) ([]string, error)
	// Status dice qué operadores ya cargaron su TC1 del período y cuáles faltan.
	Status(ctx context.Context, period string) (Tc1Status, error)
	// Operators lista los operadores de red que reportan TC1, para que la pantalla
	// de Nueva carga sepa entre cuáles elegir.
	Operators() []string
}

type tc1Service struct {
	repository repositories.LiquidationsTc1Repository
	// Los operadores que se esperan por período. Entra como dato y no como
	// dependencia de otro paquete: el servicio no tiene por qué saber de dónde
	// sale el catálogo, solo cuál es.
	expected []string
}

func NewTc1Service(
	repository repositories.LiquidationsTc1Repository,
	expected []string,
) Tc1Service {
	return tc1Service{repository: repository, expected: expected}
}

// Operators son los operadores de red que reportan TC1.
//
// Existe porque el selector de operador de Nueva carga salía del backend
// TypeScript, que lee de Supabase y no está desplegado en desarrollo: la lista
// quedaba vacía y no se podía cargar un TC1. Ahora sale de acá.
func (service tc1Service) Operators() []string {
	return service.expected
}

// Status cruza los operadores esperados contra los que ya cargaron.
//
// Lo que se cuenta es OPERADORES, no cargues: si alguien vuelve a cargar el
// archivo de CENS, sigue siendo uno de los 21 y no dos.
func (service tc1Service) Status(ctx context.Context, period string) (Tc1Status, error) {
	ctx, span := tracing.StartSpan(ctx, "services.tc1.Status")
	defer span.End()

	period = strings.TrimSpace(period)
	if !formatoPeriodo.MatchString(period) {
		return Tc1Status{}, fmt.Errorf(
			"el período %q no tiene la forma AAAA-MM (por ejemplo 2026-02)", period)
	}

	cargues, err := service.repository.Loads(ctx, []string{period})
	if err != nil {
		return Tc1Status{}, err
	}

	yaCargaron := make(map[string]bool, len(cargues))
	for _, c := range cargues {
		yaCargaron[c.OperatorCode] = true
	}

	// Se recorre `expected` y no el mapa: el orden de un mapa en Go es aleatorio,
	// y una lista que cambia de orden en cada consulta es incómoda de leer en
	// pantalla. Misma razón por la que el catálogo de SDL está ordenado.
	estado := Tc1Status{
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

// El mismo formato que guardan Cargos STR y Tarifas SDL. La base tiene el mismo
// CHECK; acá se valida antes para dar un mensaje entendible en vez de un error
// de Postgres.
var formatoPeriodo = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

func (service tc1Service) Confirm(
	ctx context.Context,
	period, operatorCode string,
	rows []Row,
	meta LoadMeta,
) (string, error) {
	ctx, span := tracing.StartSpan(ctx, "services.tc1.Confirm")
	defer span.End()

	period = strings.TrimSpace(period)
	operatorCode = strings.ToUpper(strings.TrimSpace(operatorCode))

	if !formatoPeriodo.MatchString(period) {
		return "", fmt.Errorf(
			"el período %q no tiene la forma AAAA-MM (por ejemplo 2026-02)", period)
	}
	if operatorCode == "" {
		return "", fmt.Errorf("falta el operador de red del archivo")
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("el cargue no trae ninguna frontera")
	}

	// Fronteras vacías o repetidas. La frontera es la clave con la que después se
	// cruza contra Facturación: una repetida duplicaría el cruce y una vacía es
	// una fila que nadie va a poder conciliar. La base rechaza las vacías con un
	// CHECK, pero cortar acá dice CUÁL fila está mal en vez de tumbar el lote.
	vistas := make(map[string]int, len(rows))
	for i, r := range rows {
		frontera := strings.TrimSpace(r.CodFronteraComercial)
		if frontera == "" {
			return "", fmt.Errorf(
				"la fila %d no trae código de frontera comercial, que es con lo que se cruza "+
					"contra Facturación", i+1)
		}
		if previa, repetida := vistas[frontera]; repetida {
			return "", fmt.Errorf(
				"la frontera %q aparece en las filas %d y %d del archivo. Revisá que el archivo "+
					"sea el del operador %s y que no traiga filas duplicadas",
				frontera, previa+1, i+1, operatorCode)
		}
		vistas[frontera] = i
	}

	loadID := uuid.NewString()
	filas := make([]models.LiquidationsTc1Input, 0, len(rows))
	for _, r := range rows {
		filas = append(filas, aModelo(r, loadID, period, operatorCode, meta))
	}

	if err := service.repository.InsertInputs(ctx, filas); err != nil {
		return "", fmt.Errorf("no se pudieron guardar las fronteras: %w", err)
	}

	return loadID, nil
}

// aModelo pasa una fila del parser a la fila de la tabla.
//
// El operador y el período NO salen de la fila: el operador se elige en Nueva
// carga junto con el archivo, y el período también. Así todas las filas de un
// cargue quedan rotuladas igual aunque el archivo diga otra cosa.
func aModelo(r Row, loadID, period, operatorCode string, meta LoadMeta) models.LiquidationsTc1Input {
	return models.LiquidationsTc1Input{
		LoadID:       loadID,
		Period:       period,
		OperatorCode: operatorCode,

		Niu:                       r.Niu,
		CodigoDeConexion:          r.CodigoDeConexion,
		TipoDeConexion:            r.TipoDeConexion,
		NivelDeTension:            r.NivelDeTension,
		NivelDeTensionPrimario:    r.NivelDeTensionPrimario,
		PorcPropiedadDelActivo:    r.PorcPropiedadDelActivo,
		ConexionRed:               r.ConexionRed,
		IDComercializador:         r.IDComercializador,
		IDMercado:                 r.IDMercado,
		GrupoDeCalidad:            r.GrupoDeCalidad,
		CodFronteraComercial:      strings.TrimSpace(r.CodFronteraComercial),
		CodigoCircuitoOLinea:      r.CodigoCircuitoOLinea,
		CodigoTransformador:       r.CodigoTransformador,
		CodigoDaneNiu:             r.CodigoDaneNiu,
		Ubicacion:                 r.Ubicacion,
		Direccion:                 r.Direccion,
		CondicionEspecial:         r.CondicionEspecial,
		TipoAreaEspecial:          r.TipoAreaEspecial,
		CodigoAreaEspecial:        r.CodigoAreaEspecial,
		EstratoID:                 r.EstratoID,
		Altitud:                   r.Altitud,
		Longitud:                  r.Longitud,
		Latitud:                   r.Latitud,
		Autogenerador:             r.Autogenerador,
		ExportaEnergia:            r.ExportaEnergia,
		Potencia:                  r.Potencia,
		TipoGeneracion:            r.TipoGeneracion,
		CodigoFronteraAutoGen:     r.CodigoFronteraAutoGen,
		InicioOperacion:           r.InicioOperacion,
		ContratoRespaldo:          r.ContratoRespaldo,
		CapacidadContratoRespaldo: r.CapacidadContratoRespaldo,
		Ciclo:                     r.Ciclo,
		Nodo:                      r.Nodo,

		SourceFile:  meta.SourceFile,
		CreatedBy:   meta.CreatedBy,
		CreatedByID: meta.CreatedByID,
	}
}

func (service tc1Service) CurrentInputs(
	ctx context.Context,
	filter repositories.Tc1Filter,
) ([]models.LiquidationsTc1Input, error) {
	ctx, span := tracing.StartSpan(ctx, "services.tc1.CurrentInputs")
	defer span.End()

	return service.repository.CurrentInputs(ctx, filter)
}

func (service tc1Service) Loads(
	ctx context.Context,
	periods []string,
) ([]repositories.Tc1Load, error) {
	ctx, span := tracing.StartSpan(ctx, "services.tc1.Loads")
	defer span.End()

	return service.repository.Loads(ctx, periods)
}

func (service tc1Service) Periods(ctx context.Context) ([]string, error) {
	ctx, span := tracing.StartSpan(ctx, "services.tc1.Periods")
	defer span.End()

	return service.repository.Periods(ctx)
}
