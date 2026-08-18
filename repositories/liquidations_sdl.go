package repositories

import (
	"context"
	"fmt"
	"time"

	"bia-bills/models"
	"bia-bills/providers/postgres"
)

// Acceso a datos de Tarifas SDL.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// ── La regla de "lo vigente", en un solo lugar ───────────────────────────────
// Las dos tablas son append-only: guardan lo vigente Y el histórico. Recargar un
// período no pisa lo anterior. Por eso toda lectura tiene que quedarse con el
// registro más reciente de cada (period, operator_code).
//
// Esa regla se escribe UNA vez, en `vigentesSdl`, y todas las consultas pasan por
// ahí. Una consulta que fuera directo a la tabla sin aplicarla mezclaría cargas
// viejas con nuevas y devolvería tarifas de dos meses distintos como si fueran
// del mismo — y las tarifas alimentan la facturación.

// SdlRate es una fila del resultado vigente.
//
// ⚠️ Los `gorm:"column:..."` son OBLIGATORIOS acá, no decorativos. GORM deriva el
// nombre de columna del nombre del campo y `ActiveLevel1Operator` le da
// `active_level1_operator` —sin guion antes del dígito—, que no existe en la
// tabla. Sin los tags el Scan no falla: deja las diez tarifas en CERO, y la
// pantalla muestra ceros como si el período no tuviera datos.
type SdlRate struct {
	LoadID       string `gorm:"column:load_id" json:"load_id"`
	Period       string `gorm:"column:period" json:"period"`
	OperatorCode string `gorm:"column:operator_code" json:"operator_code"`
	OperatorName string `gorm:"column:operator_name" json:"operator_name"`
	AgentCode    string `gorm:"column:agent_code" json:"agent_code"`
	Market       string `gorm:"column:market" json:"market"`

	ActiveLevel1Operator float64 `gorm:"column:active_level_1_operator" json:"active_level_1_operator"`
	ActiveLevel1Shared   float64 `gorm:"column:active_level_1_shared" json:"active_level_1_shared"`
	ActiveLevel1User     float64 `gorm:"column:active_level_1_user" json:"active_level_1_user"`
	ActiveLevel2User     float64 `gorm:"column:active_level_2_user" json:"active_level_2_user"`
	ActiveLevel3User     float64 `gorm:"column:active_level_3_user" json:"active_level_3_user"`

	ReactiveLevel1Operator float64 `gorm:"column:reactive_level_1_operator" json:"reactive_level_1_operator"`
	ReactiveLevel1Shared   float64 `gorm:"column:reactive_level_1_shared" json:"reactive_level_1_shared"`
	ReactiveLevel1User     float64 `gorm:"column:reactive_level_1_user" json:"reactive_level_1_user"`
	ReactiveLevel2User     float64 `gorm:"column:reactive_level_2_user" json:"reactive_level_2_user"`
	ReactiveLevel3User     float64 `gorm:"column:reactive_level_3_user" json:"reactive_level_3_user"`
}

// SdlRateFilter acota la lectura. Los campos vacíos no filtran.
type SdlRateFilter struct {
	Periods       []string
	OperatorCodes []string
}

// SdlLoad es un cargue del historial. Sale de agrupar los insumos por load_id,
// igual que en Cargos STR: no hay tabla de cargas.
type SdlLoad struct {
	LoadID      string    `json:"load_id"`
	Period      string    `json:"period"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
	CreatedByID string    `json:"created_by_id"`
	SourceFiles string    `json:"source_files"`
	Operators   int       `json:"operators"`
}

type LiquidationsSdlRepository interface {
	// InsertInputs guarda los componentes en file-compiler.
	InsertInputs(ctx context.Context, rows []models.LiquidationsSdlInput) error
	// InsertRates guarda las tarifas en calculator-prices.
	InsertRates(ctx context.Context, rows []models.LiquidationsSdlRate) error
	// DeleteInputsByLoad limpia un cargue que no llegó a completarse.
	DeleteInputsByLoad(ctx context.Context, loadID string) error

	// CurrentRates devuelve las tarifas vigentes por (período, operador).
	CurrentRates(ctx context.Context, filter SdlRateFilter) ([]SdlRate, error)
	// CurrentInputs devuelve los componentes vigentes. Sirve para auditar: se
	// recalcula desde acá y tiene que dar las tarifas guardadas.
	CurrentInputs(ctx context.Context, periods []string) ([]models.LiquidationsSdlInput, error)
	// PeriodsWithRates lista los períodos con datos, más reciente primero.
	PeriodsWithRates(ctx context.Context) ([]string, error)
	// Loads devuelve el historial de cargues, más reciente primero.
	Loads(ctx context.Context, periods []string) ([]SdlLoad, error)
}

type liquidationsSdlRepository struct {
	db postgres.LiquidationsDB
}

func NewLiquidationsSdlRepository(db postgres.LiquidationsDB) LiquidationsSdlRepository {
	return liquidationsSdlRepository{db: db}
}

// vigentesSdl arma el subquery que resuelve "el más reciente por (período,
// operador)". El desempate por id hace el resultado estable si dos cargues
// cayeran en el mismo instante.
func vigentesSdl(tabla, columnas string) string {
	return fmt.Sprintf(`
		SELECT DISTINCT ON (period, operator_code) %s
		  FROM %s
		 ORDER BY period, operator_code, created_at DESC, id DESC`, columnas, tabla)
}

func (repository liquidationsSdlRepository) InsertInputs(ctx context.Context, rows []models.LiquidationsSdlInput) error {
	if len(rows) == 0 {
		return nil
	}

	return repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Create(&rows).Error
}

func (repository liquidationsSdlRepository) InsertRates(ctx context.Context, rows []models.LiquidationsSdlRate) error {
	if len(rows) == 0 {
		return nil
	}

	return repository.db.Connection(postgres.LiqDBCalculatorPrices).
		WithContext(ctx).
		Create(&rows).Error
}

func (repository liquidationsSdlRepository) DeleteInputsByLoad(ctx context.Context, loadID string) error {
	return repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Where("load_id = ?", loadID).
		Delete(&models.LiquidationsSdlInput{}).Error
}

const columnasSdlRate = `load_id, period, operator_code, operator_name, agent_code, market,
	active_level_1_operator, active_level_1_shared, active_level_1_user,
	active_level_2_user, active_level_3_user,
	reactive_level_1_operator, reactive_level_1_shared, reactive_level_1_user,
	reactive_level_2_user, reactive_level_3_user`

func (repository liquidationsSdlRepository) CurrentRates(ctx context.Context, filter SdlRateFilter) ([]SdlRate, error) {
	condiciones := ""
	args := []any{}

	// IN (?) y no = ANY(?): GORM no expande slices en ANY y Postgres devuelve un
	// error de sintaxis. Ya pasó en Cargos STR, en cuatro consultas.
	if len(filter.Periods) > 0 {
		condiciones += " AND period IN (?)"
		args = append(args, filter.Periods)
	}
	if len(filter.OperatorCodes) > 0 {
		condiciones += " AND operator_code IN (?)"
		args = append(args, filter.OperatorCodes)
	}

	query := fmt.Sprintf(`
		SELECT %s
		  FROM (%s) AS vigentes
		 WHERE 1 = 1 %s
		 ORDER BY period, operator_code`,
		columnasSdlRate,
		vigentesSdl("public.liquidations_sdl_rates", columnasSdlRate+", created_at, id"),
		condiciones)

	res := []SdlRate{}
	err := repository.db.Connection(postgres.LiqDBCalculatorPrices).
		WithContext(ctx).
		Raw(query, args...).Scan(&res).Error

	return res, err
}

func (repository liquidationsSdlRepository) CurrentInputs(
	ctx context.Context,
	periods []string,
) ([]models.LiquidationsSdlInput, error) {
	condicion := ""
	args := []any{}
	if len(periods) > 0 {
		condicion = " WHERE period IN (?)"
		args = append(args, periods)
	}

	columnas := `id, load_id, period, operator_code, operator_name, agent_code, market,
		distribution_area,
		dt1_add, dt2_add, dt3_add, dt1, dt2, dt3, cdi, cdn4, pr1, pr2, pr3,
		source_files, created_by, created_by_id, created_at`

	query := fmt.Sprintf(`
		SELECT %s FROM (%s) AS vigentes%s
		 ORDER BY period, operator_code`,
		columnas,
		vigentesSdl("public.liquidations_sdl_inputs", columnas),
		condicion)

	res := []models.LiquidationsSdlInput{}
	err := repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Raw(query, args...).Scan(&res).Error

	return res, err
}

func (repository liquidationsSdlRepository) PeriodsWithRates(ctx context.Context) ([]string, error) {
	periodos := []string{}

	err := repository.db.Connection(postgres.LiqDBCalculatorPrices).
		WithContext(ctx).
		Raw(`SELECT DISTINCT period
		       FROM public.liquidations_sdl_rates
		      ORDER BY period DESC`).
		Scan(&periodos).Error

	return periodos, err
}

// Loads reconstruye el historial agrupando los insumos por cargue. Lee de
// file-compiler: ahí está el insumo con sus metadatos.
func (repository liquidationsSdlRepository) Loads(ctx context.Context, periods []string) ([]SdlLoad, error) {
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
		       MIN(created_at)    AS created_at,
		       MAX(created_by)    AS created_by,
		       MAX(created_by_id) AS created_by_id,
		       MAX(source_files)  AS source_files,
		       COUNT(*)           AS operators
		  FROM public.liquidations_sdl_inputs%s
		 GROUP BY load_id, period
		 ORDER BY MIN(created_at) DESC`, condicion)

	res := []SdlLoad{}
	err := repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Raw(query, args...).Scan(&res).Error

	return res, err
}
