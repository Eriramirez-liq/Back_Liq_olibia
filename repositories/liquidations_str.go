package repositories

import (
	"context"
	"fmt"

	"bia-bills/models"
	"bia-bills/providers/postgres"
)

// Acceso a datos de Cargos STR.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// ── La regla de "lo vigente", en un solo lugar ───────────────────────────────
// Las dos tablas son append-only: guardan lo vigente Y el histórico. Recargar un
// período no pisa lo anterior, quedan las dos cargas. Por eso toda lectura tiene
// que quedarse con el registro más reciente de cada (period, operator_code).
//
// Esa regla se escribe UNA vez, en `vigentes`, y todas las consultas pasan por
// ahí. Una consulta que fuera directo a la tabla sin aplicarla sumaría cargas
// viejas con nuevas y DUPLICARÍA los montos — con cifras de mil millones, el
// error no se nota a simple vista.

// StrCharge es una fila del resultado vigente.
type StrCharge struct {
	LoadID        string  `json:"load_id"`
	Period        string  `json:"period"`
	OperatorCode  string  `json:"operator_code"`
	OperatorName  string  `json:"operator_name"`
	AmountPayable float64 `json:"amount_payable"`
}

// StrChargeFilter acota la lectura. Los campos vacíos no filtran.
type StrChargeFilter struct {
	Periods       []string
	OperatorCodes []string
}

type LiquidationsStrRepository interface {
	// InsertInputs guarda el desglose crudo en file-compiler.
	InsertInputs(ctx context.Context, rows []models.LiquidationsStrInput) error
	// InsertCharges guarda el valor a pagar en calculator-prices.
	InsertCharges(ctx context.Context, rows []models.LiquidationsStrCharge) error
	// DeleteInputsByLoad limpia un cargue que no llegó a completarse. No es
	// "reemplazar historial" —el modelo no lo permite— sino borrar una carga que
	// nunca existió, cuando la escritura del resultado falló después del insumo.
	DeleteInputsByLoad(ctx context.Context, loadID string) error

	// CurrentCharges devuelve el valor a pagar vigente por (período, operador).
	CurrentCharges(ctx context.Context, filter StrChargeFilter) ([]StrCharge, error)
	// TotalsByPeriod suma lo vigente de cada período. Los períodos sin datos no
	// aparecen en el mapa, para poder distinguir "cero" de "no cargado".
	TotalsByPeriod(ctx context.Context, periods []string) (map[string]float64, error)
	// PeriodsWithCharges lista los períodos que ya tienen datos, más reciente
	// primero.
	PeriodsWithCharges(ctx context.Context) ([]string, error)
}

type liquidationsStrRepository struct {
	db postgres.LiquidationsDB
}

func NewLiquidationsStrRepository(db postgres.LiquidationsDB) LiquidationsStrRepository {
	return liquidationsStrRepository{db: db}
}

// vigentes arma el subquery que resuelve "el más reciente por (período,
// operador)". DISTINCT ON se queda con la primera fila de cada grupo según el
// ORDER BY; el desempate por id hace el resultado estable si dos cargas cayeran
// en el mismo instante.
func vigentes(columnas string) string {
	return fmt.Sprintf(`
		SELECT DISTINCT ON (period, operator_code) %s
		  FROM public.liquidations_str_charges
		 ORDER BY period, operator_code, created_at DESC, id DESC`, columnas)
}

func (repository liquidationsStrRepository) InsertInputs(ctx context.Context, rows []models.LiquidationsStrInput) error {
	if len(rows) == 0 {
		return nil
	}

	return repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Create(&rows).Error
}

func (repository liquidationsStrRepository) InsertCharges(ctx context.Context, rows []models.LiquidationsStrCharge) error {
	if len(rows) == 0 {
		return nil
	}

	return repository.db.Connection(postgres.LiqDBCalculatorPrices).
		WithContext(ctx).
		Create(&rows).Error
}

func (repository liquidationsStrRepository) DeleteInputsByLoad(ctx context.Context, loadID string) error {
	return repository.db.Connection(postgres.LiqDBFileCompiler).
		WithContext(ctx).
		Where("load_id = ?", loadID).
		Delete(&models.LiquidationsStrInput{}).Error
}

func (repository liquidationsStrRepository) CurrentCharges(ctx context.Context, filter StrChargeFilter) ([]StrCharge, error) {
	condiciones := ""
	args := []any{}

	if len(filter.Periods) > 0 {
		condiciones += " AND period = ANY(?)"
		args = append(args, filter.Periods)
	}
	if len(filter.OperatorCodes) > 0 {
		condiciones += " AND operator_code = ANY(?)"
		args = append(args, filter.OperatorCodes)
	}

	query := fmt.Sprintf(`
		SELECT load_id, period, operator_code, operator_name, amount_payable
		  FROM (%s) AS vigentes
		 WHERE 1 = 1 %s
		 ORDER BY period, operator_name`,
		vigentes("load_id, period, operator_code, operator_name, amount_payable, created_at, id"),
		condiciones)

	res := []StrCharge{}
	err := repository.db.Connection(postgres.LiqDBCalculatorPrices).
		WithContext(ctx).
		Raw(query, args...).Scan(&res).Error

	return res, err
}

func (repository liquidationsStrRepository) TotalsByPeriod(ctx context.Context, periods []string) (map[string]float64, error) {
	totales := map[string]float64{}
	if len(periods) == 0 {
		return totales, nil
	}

	query := fmt.Sprintf(`
		SELECT period, COALESCE(SUM(amount_payable), 0) AS total
		  FROM (%s) AS vigentes
		 WHERE period = ANY(?)
		 GROUP BY period`,
		vigentes("period, operator_code, amount_payable, created_at, id"))

	filas := []struct {
		Period string
		Total  float64
	}{}

	err := repository.db.Connection(postgres.LiqDBCalculatorPrices).
		WithContext(ctx).
		Raw(query, periods).Scan(&filas).Error
	if err != nil {
		return nil, err
	}

	for _, f := range filas {
		totales[f.Period] = f.Total
	}

	return totales, nil
}

func (repository liquidationsStrRepository) PeriodsWithCharges(ctx context.Context) ([]string, error) {
	periodos := []string{}

	err := repository.db.Connection(postgres.LiqDBCalculatorPrices).
		WithContext(ctx).
		Raw(`SELECT DISTINCT period
		       FROM public.liquidations_str_charges
		      ORDER BY period DESC`).
		Scan(&periodos).Error

	return periodos, err
}
