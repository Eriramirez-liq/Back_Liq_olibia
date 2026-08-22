package repositories

import (
	"context"
	"fmt"

	"bia-bills/models"
	"bia-bills/providers/postgres"
)

// Repositorio de TC1.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// Una sola base: file-compiler. TC1 no produce nada derivado, así que no hay
// tabla espejo en calculator-prices como en Cargos STR y Tarifas SDL.

// Tc1Load es una línea del historial de cargues.
type Tc1Load struct {
	LoadID       string `json:"load_id"        gorm:"column:load_id"`
	Period       string `json:"period"         gorm:"column:period"`
	OperatorCode string `json:"operator_code"  gorm:"column:operator_code"`
	CreatedAt    string `json:"created_at"     gorm:"column:created_at"`
	CreatedBy    string `json:"created_by"     gorm:"column:created_by"`
	CreatedByID  string `json:"created_by_id"  gorm:"column:created_by_id"`
	SourceFile   string `json:"source_file"    gorm:"column:source_file"`
	Borders      int    `json:"borders"        gorm:"column:borders"`
}

// Tc1Filter acota la lectura de fronteras vigentes.
type Tc1Filter struct {
	Periods       []string
	OperatorCodes []string
}

type LiquidationsTc1Repository interface {
	// InsertInputs guarda las fronteras de un cargue.
	InsertInputs(ctx context.Context, rows []models.LiquidationsTc1Input) error
	// CurrentInputs devuelve las fronteras vigentes: las del último cargue de
	// cada período y operador.
	CurrentInputs(ctx context.Context, filter Tc1Filter) ([]models.LiquidationsTc1Input, error)
	// Loads lista el historial de cargues, del más reciente al más viejo.
	Loads(ctx context.Context, periods []string) ([]Tc1Load, error)
	// Periods lista los períodos con datos, más reciente primero.
	Periods(ctx context.Context) ([]string, error)
}

type liquidationsTc1Repository struct {
	db postgres.LiquidationsDB
}

func NewLiquidationsTc1Repository(db postgres.LiquidationsDB) LiquidationsTc1Repository {
	return liquidationsTc1Repository{db: db}
}

// cargasVigentesTc1 es el subquery que resuelve qué cargue manda en cada
// período y operador.
//
// ── Por qué no es el DISTINCT ON de STR y SDL ───────────────────────────────
// Allá hay UNA fila por período y operador, así que lo vigente es la fila más
// reciente. Acá hay MUCHAS —una por frontera, 156 en el operador más grande—, y
// quedarse con la más reciente de cada (período, operador) devolvería una sola
// frontera. Peor todavía sería desempatar frontera por frontera: mezclaría un
// cargue viejo con uno nuevo y daría una foto que nunca existió.
//
// Lo vigente es el CARGUE completo: se elige su load_id y después se traen todas
// sus filas. El desempate por load_id hace estable el resultado si dos cargues
// cayeran en el mismo instante.
const cargasVigentesTc1 = `
	SELECT DISTINCT ON (period, operator_code) load_id
	  FROM public.liquidations_tc1_inputs
	 ORDER BY period, operator_code, created_at DESC, load_id DESC`

func (repository liquidationsTc1Repository) InsertInputs(
	ctx context.Context,
	rows []models.LiquidationsTc1Input,
) error {
	if len(rows) == 0 {
		return nil
	}

	// En lotes: un período completo son 21 operadores y unos pocos miles de
	// fronteras, y un INSERT único con todas superaría el límite de parámetros
	// de Postgres (65535) con 37 columnas por fila.
	const porLote = 500

	return repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		CreateInBatches(&rows, porLote).Error
}

func (repository liquidationsTc1Repository) CurrentInputs(
	ctx context.Context,
	filter Tc1Filter,
) ([]models.LiquidationsTc1Input, error) {
	condiciones := ""
	args := []any{}

	// IN (?) y NO = ANY(?): GORM no expande un slice dentro de ANY y Postgres
	// responde "syntax error at or near ",". Ya costó una sesión en Cargos STR.
	if len(filter.Periods) > 0 {
		condiciones += " AND period IN (?)"
		args = append(args, filter.Periods)
	}
	if len(filter.OperatorCodes) > 0 {
		condiciones += " AND operator_code IN (?)"
		args = append(args, filter.OperatorCodes)
	}

	query := fmt.Sprintf(`
		SELECT *
		  FROM public.liquidations_tc1_inputs
		 WHERE load_id IN (%s)%s
		 ORDER BY period, operator_code, cod_frontera_comercial`,
		cargasVigentesTc1, condiciones)

	res := []models.LiquidationsTc1Input{}
	err := repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Raw(query, args...).Scan(&res).Error

	return res, err
}

// Loads reconstruye el historial agrupando las fronteras por cargue.
//
// Un cargue de TC1 es de UN operador —cada OR manda su archivo— así que el grupo
// es (load_id, period, operator_code) y lo que se cuenta son fronteras, no
// operadores como en Tarifas SDL.
func (repository liquidationsTc1Repository) Loads(
	ctx context.Context,
	periods []string,
) ([]Tc1Load, error) {
	condicion := ""
	args := []any{}
	if len(periods) > 0 {
		condicion = " WHERE period IN (?)"
		args = append(args, periods)
	}

	// Los MAX() sobre los metadatos no son una agregación de verdad: todas las
	// filas de un cargue traen el mismo valor. Es la forma de traerlos con GROUP BY.
	query := fmt.Sprintf(`
		SELECT load_id,
		       period,
		       operator_code,
		       MIN(created_at)    AS created_at,
		       MAX(created_by)    AS created_by,
		       MAX(created_by_id) AS created_by_id,
		       MAX(source_file)   AS source_file,
		       COUNT(*)           AS borders
		  FROM public.liquidations_tc1_inputs%s
		 GROUP BY load_id, period, operator_code
		 ORDER BY MIN(created_at) DESC`, condicion)

	res := []Tc1Load{}
	err := repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Raw(query, args...).Scan(&res).Error

	return res, err
}

func (repository liquidationsTc1Repository) Periods(ctx context.Context) ([]string, error) {
	res := []string{}
	err := repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Raw(`SELECT DISTINCT period
		       FROM public.liquidations_tc1_inputs
		      ORDER BY period DESC`).
		Scan(&res).Error

	return res, err
}
