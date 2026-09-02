package models

import "time"

// Tabla de SDL por Operador (preliquidación).
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.

// LiquidationsSdlPreliquidation es una frontera de la preliquidación que un
// operador de red envía, ya normalizada, en la base file-compiler.
//
// ── Acá no se calcula nada ──────────────────────────────────────────────────
// Se guarda lo que el operador reporta y nada más. El valor en pesos NO va en
// esta tabla: sale de energía × tarifa, y como hay operadores que no mandan la
// tarifa, en esos casos se busca en `liquidations_sdl_rates` (calculator-prices)
// por operador, período, nivel de tensión y propiedad de activos. Ese cálculo es
// de quien lo consume —hoy la caja de valor SDL del Dashboard—; guardarlo acá
// congelaría un número que depende de una tarifa que puede recalcularse.
//
// ── Casi todo opcional, a propósito ─────────────────────────────────────────
// Son veintiún operadores y diecinueve formatos. No todos reportan reactiva,
// factor M, contador M ni tarifas. Exigir esas columnas rechazaría cargues
// legítimos, así que lo obligatorio es solo lo que identifica la fila.
//
// ── APPEND-ONLY, con la regla de TC1 y no la de STR ─────────────────────────
// Nada se pisa ni se borra. Y lo vigente es el ÚLTIMO CARGUE, no la última fila:
// hay muchas filas por período y operador —una por frontera—, así que quedarse
// con la más reciente de cada una mezclaría fronteras de dos cargues distintos.
// Esa regla vive en el repositorio.
//
// Las etiquetas `json` importan tanto como las `gorm`: el endpoint de lectura
// devuelve este struct tal cual, y sin ellas Go serializa con los nombres de
// campo de Go —LoadID, ActiveEnergy— que el front no sabe leer.
type LiquidationsSdlPreliquidation struct {
	ID           string `json:"id"            gorm:"column:id;primaryKey"`
	LoadID       string `json:"load_id"       gorm:"column:load_id;not null"`
	Period       string `json:"period"        gorm:"column:period;not null"` // "YYYY-MM"
	OperatorCode string `json:"operator_code" gorm:"column:operator_code;not null"`

	// El período que venía escrito DENTRO del archivo, cuando lo trae. Contra él
	// se contrasta el elegido en el módulo: distinto bloquea el cargue, ausente
	// queda en aviso. Se guarda para poder auditar con qué venía el archivo.
	FilePeriod *string `json:"file_period" gorm:"column:file_period"`

	Sic  string  `json:"sic"  gorm:"column:sic;not null"`
	Name *string `json:"name" gorm:"column:name"`

	ActiveEnergy *float64 `json:"active_energy" gorm:"column:active_energy"`

	InductiveTotal      *float64 `json:"inductive_total"      gorm:"column:inductive_total"`
	InductivePenalized  *float64 `json:"inductive_penalized"  gorm:"column:inductive_penalized"`
	CapacitiveTotal     *float64 `json:"capacitive_total"     gorm:"column:capacitive_total"`
	CapacitivePenalized *float64 `json:"capacitive_penalized" gorm:"column:capacitive_penalized"`

	FactorM  *float64 `json:"factor_m"  gorm:"column:factor_m"`
	CounterM *float64 `json:"counter_m" gorm:"column:counter_m"`

	TensionLevel   *string `json:"tension_level"   gorm:"column:tension_level"`
	AssetOwnership *string `json:"asset_ownership" gorm:"column:asset_ownership"`

	ActiveRate   *float64 `json:"active_rate"   gorm:"column:active_rate"`
	ReactiveRate *float64 `json:"reactive_rate" gorm:"column:reactive_rate"`

	// La frontera venía repetida dentro del mismo archivo. No se rechaza, se
	// marca: el reporte de Congruencia excluye las marcadas.
	IsDuplicate bool `json:"is_duplicate" gorm:"column:is_duplicate;not null"`

	SourceFile  *string   `json:"source_file"   gorm:"column:source_file"`
	CreatedBy   *string   `json:"created_by"    gorm:"column:created_by"`
	CreatedByID *string   `json:"created_by_id" gorm:"column:created_by_id"`
	CreatedAt   time.Time `json:"created_at"    gorm:"column:created_at"`
}

func (LiquidationsSdlPreliquidation) TableName() string {
	return "public.liquidations_sdl_preliquidations"
}
