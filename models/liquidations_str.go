package models

import "time"

// Tablas de Cargos STR.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// Las dos son APPEND-ONLY: cada cargue inserta filas nuevas con su LoadID y su
// CreatedAt, y nada se pisa. De ahí que no haya UpdatedAt —una fila insertada
// nunca se modifica— y que la unicidad sea (load_id, operator_code) en vez de
// (period, operator_code).
//
// Toda lectura tiene que quedarse con el registro más reciente de cada
// (period, operator_code); esa regla vive en el repositorio.

// LiquidationsStrInput es el insumo crudo, en la base file-compiler.
//
// No tiene columna de valor a pagar a propósito: la factura y cada ajuste viajan
// por separado. La suma vive en la otra base, vinculada por LoadID.
type LiquidationsStrInput struct {
	ID           string `gorm:"column:id;primaryKey;default:gen_random_uuid()::text"`
	LoadID       string `gorm:"column:load_id;not null"`
	Period       string `gorm:"column:period;not null"` // "YYYY-MM"
	OperatorCode string `gorm:"column:operator_code;not null"`

	InvoiceAmount float64 `gorm:"column:invoice_amount;not null"`
	// Punteros porque NULL y 0 significan cosas distintas: NULL es "ese archivo
	// de ajuste no vino en el lote", 0 es "vino y el operador tenía cero".
	Reinvoice1Amount *float64 `gorm:"column:reinvoice_1_amount"`
	Reinvoice2Amount *float64 `gorm:"column:reinvoice_2_amount"`
	Reinvoice3Amount *float64 `gorm:"column:reinvoice_3_amount"`

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (LiquidationsStrInput) TableName() string {
	return "public.liquidations_str_inputs"
}

// LiquidationsStrCharge es el resultado, en la base calculator-prices: lo que se
// le paga a cada operador en un período. Alimenta la orden de compra de NetSuite.
type LiquidationsStrCharge struct {
	ID           string `gorm:"column:id;primaryKey;default:gen_random_uuid()::text"`
	LoadID       string `gorm:"column:load_id;not null"`
	Period       string `gorm:"column:period;not null"`
	OperatorCode string `gorm:"column:operator_code;not null"`
	// Nombre legal, resuelto contra public.agents de file-compiler. Se guarda ya
	// resuelto porque ese catálogo vive en la otra base.
	OperatorName string `gorm:"column:operator_name;not null"`
	// Factura del mes más todos los ajustes del lote. Puede ser negativo si los
	// ajustes superan la factura.
	AmountPayable float64 `gorm:"column:amount_payable;not null"`

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (LiquidationsStrCharge) TableName() string {
	return "public.liquidations_str_charges"
}
