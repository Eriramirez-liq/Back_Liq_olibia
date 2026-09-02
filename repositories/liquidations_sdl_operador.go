package repositories

import (
	"context"
	"fmt"

	"bia-bills/models"
	"bia-bills/providers/postgres"
)

// Repositorio de SDL por Operador (preliquidación).
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// Una sola base: file-compiler. La preliquidación no produce nada derivado que
// haya que guardar, así que no hay tabla espejo en calculator-prices como en
// Cargos STR y Tarifas SDL.

// SdlPreliqLoad es una línea del historial de cargues.
type SdlPreliqLoad struct {
	LoadID       string `json:"load_id"       gorm:"column:load_id"`
	Period       string `json:"period"        gorm:"column:period"`
	OperatorCode string `json:"operator_code" gorm:"column:operator_code"`
	CreatedAt    string `json:"created_at"    gorm:"column:created_at"`
	CreatedBy    string `json:"created_by"    gorm:"column:created_by"`
	CreatedByID  string `json:"created_by_id" gorm:"column:created_by_id"`
	SourceFile   string `json:"source_file"   gorm:"column:source_file"`
	Borders      int    `json:"borders"       gorm:"column:borders"`
}

// SdlPreliqFilter acota la lectura de fronteras vigentes.
type SdlPreliqFilter struct {
	Periods       []string
	OperatorCodes []string
}

type LiquidationsSdlPreliqRepository interface {
	// InsertRows guarda las fronteras de un cargue.
	InsertRows(ctx context.Context, rows []models.LiquidationsSdlPreliquidation) error
	// CurrentRows devuelve las fronteras vigentes: las del último cargue de cada
	// período y operador.
	CurrentRows(ctx context.Context, filter SdlPreliqFilter) ([]models.LiquidationsSdlPreliquidation, error)
	// Loads lista el historial de cargues, del más reciente al más viejo.
	Loads(ctx context.Context, periods []string) ([]SdlPreliqLoad, error)
	// Periods lista los períodos con datos, más reciente primero.
	Periods(ctx context.Context) ([]string, error)
}

type liquidationsSdlPreliqRepository struct {
	db postgres.LiquidationsDB
}

func NewLiquidationsSdlPreliqRepository(db postgres.LiquidationsDB) LiquidationsSdlPreliqRepository {
	return liquidationsSdlPreliqRepository{db: db}
}

// cargasVigentesSdlPreliq elige qué cargue manda en cada período y operador.
//
// Es la regla de TC1 y no la de Cargos STR: allá hay UNA fila por período y
// operador, así que lo vigente es la fila más reciente. Acá hay MUCHAS —una por
// frontera— y desempatar frontera por frontera mezclaría un cargue viejo con uno
// nuevo, dando una foto que nunca existió. Lo vigente es el CARGUE completo.
//
// El desempate por load_id hace estable el resultado si dos cargues cayeran en
// el mismo instante.
const cargasVigentesSdlPreliq = `
	SELECT DISTINCT ON (period, operator_code) load_id
	  FROM public.liquidations_sdl_preliquidations
	 ORDER BY period, operator_code, created_at DESC, load_id DESC`

func (repository liquidationsSdlPreliqRepository) InsertRows(
	ctx context.Context,
	rows []models.LiquidationsSdlPreliquidation,
) error {
	if len(rows) == 0 {
		return nil
	}

	// En lotes: un operador grande manda miles de fronteras y un INSERT único
	// con todas superaría el límite de parámetros de Postgres (65535) con 23
	// columnas por fila.
	const porLote = 500

	return repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		CreateInBatches(&rows, porLote).Error
}

func (repository liquidationsSdlPreliqRepository) CurrentRows(
	ctx context.Context,
	filter SdlPreliqFilter,
) ([]models.LiquidationsSdlPreliquidation, error) {
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
		  FROM public.liquidations_sdl_preliquidations
		 WHERE load_id IN (%s)%s
		 ORDER BY period, operator_code, sic`,
		cargasVigentesSdlPreliq, condiciones)

	res := []models.LiquidationsSdlPreliquidation{}
	err := repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Raw(query, args...).Scan(&res).Error

	return res, err
}

// Loads reconstruye el historial agrupando las fronteras por cargue.
//
// Un cargue es de UN operador —cada OR manda su archivo— así que el grupo es
// (load_id, period, operator_code) y lo que se cuenta son fronteras.
func (repository liquidationsSdlPreliqRepository) Loads(
	ctx context.Context,
	periods []string,
) ([]SdlPreliqLoad, error) {
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
		  FROM public.liquidations_sdl_preliquidations%s
		 GROUP BY load_id, period, operator_code
		 ORDER BY MIN(created_at) DESC`, condicion)

	res := []SdlPreliqLoad{}
	err := repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Raw(query, args...).Scan(&res).Error

	return res, err
}

func (repository liquidationsSdlPreliqRepository) Periods(ctx context.Context) ([]string, error) {
	res := []string{}
	err := repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Raw(`SELECT DISTINCT period
		       FROM public.liquidations_sdl_preliquidations
		      ORDER BY period DESC`).
		Scan(&res).Error

	return res, err
}
