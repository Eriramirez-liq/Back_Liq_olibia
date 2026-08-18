package models

import "time"

// Tablas de Tarifas SDL.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.
//
// Las dos son APPEND-ONLY, igual que las de Cargos STR: cada cargue inserta
// filas nuevas con su LoadID y su CreatedAt, y nada se pisa. De ahí que no haya
// UpdatedAt y que la unicidad sea (load_id, operator_code).
//
// Toda lectura tiene que quedarse con el registro más reciente de cada
// (period, operator_code); esa regla vive en el repositorio.

// LiquidationsSdlInput es todo lo que alimenta el cálculo de un operador, en la
// base file-compiler.
//
// Guarda los DOS juegos de DT —el del ADD de su área y el de su propio archivo de
// uso de la red— aunque el cálculo use solo uno según el tipo de operador. Es a
// propósito: es lo que permite auditar por qué una tarifa salió como salió.
type LiquidationsSdlInput struct {
	ID           string `gorm:"column:id;primaryKey;default:gen_random_uuid()::text"`
	LoadID       string `gorm:"column:load_id;not null"`
	Period       string `gorm:"column:period;not null"` // "YYYY-MM"
	OperatorCode string `gorm:"column:operator_code;not null"`

	// Identidad del operador. El código de agente lo trae el propio archivo y es
	// con lo que se resuelve el nombre legal contra public.agents, con el mismo
	// filtro de actividad que usa Cargos STR — así los dos módulos muestran el
	// mismo nombre para el mismo operador.
	//
	// El mercado es imprescindible, no decorativo: dos agentes atienden dos
	// mercados cada uno (EEPD → Pereira y Cartago; EPSD → Valle y Tolima), o sea
	// misma razón social en dos filas. Sin el mercado se ven como duplicados.
	OperatorName string `gorm:"column:operator_name"`
	AgentCode    string `gorm:"column:agent_code"`
	Market       string `gorm:"column:market"`

	// Área cuyo cargo ADD alimenta el NT. Vacío en los operadores que toman el NT
	// de su propio archivo.
	DistributionArea *string `gorm:"column:distribution_area"`

	// Del ADD del área. nil cuando no aplica. La base tiene un CHECK que exige
	// que el área y los tres cargos estén o falten juntos: una fila a medias
	// significa que el parser perdió algo en el camino.
	DT1Add *float64 `gorm:"column:dt1_add"`
	DT2Add *float64 `gorm:"column:dt2_add"`
	DT3Add *float64 `gorm:"column:dt3_add"`

	// Del archivo de uso de la red del propio operador.
	DT1  float64 `gorm:"column:dt1;not null"`
	DT2  float64 `gorm:"column:dt2;not null"`
	DT3  float64 `gorm:"column:dt3;not null"`
	CDI  float64 `gorm:"column:cdi;not null"`
	CDN4 float64 `gorm:"column:cdn4;not null"`

	// Pérdidas reconocidas, como FRACCIONES (0.1255 = 12,55 %).
	//
	// La base tiene un CHECK que exige [0, 1). El cálculo divide por (1 - PR), así
	// que un porcentaje acá haría el divisor negativo y todas las tarifas activas
	// saldrían absurdas sin que nada falle. El cero sí es válido y aparece en los
	// datos reales.
	PR1 float64 `gorm:"column:pr1;not null"`
	PR2 float64 `gorm:"column:pr2;not null"`
	PR3 float64 `gorm:"column:pr3;not null"`

	// Archivos que produjeron la fila, separados por coma.
	SourceFiles string `gorm:"column:source_files"`
	CreatedBy   string `gorm:"column:created_by"`
	CreatedByID string `gorm:"column:created_by_id"`

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (LiquidationsSdlInput) TableName() string {
	return "public.liquidations_sdl_inputs"
}

// LiquidationsSdlRate son las diez tarifas de un operador en un período, en la
// base calculator-prices.
//
// Diez columnas y no filas porque la forma es fija y conocida: el nivel 1 se abre
// por propiedad de activos y los niveles 2 y 3 solo tienen tarifa de usuario.
// Existen cinco combinaciones, no nueve.
//
// Se guardan con toda la precisión a propósito. La pantalla muestra dos
// decimales, pero redondear acá rompería la auditoría: recalcular desde los
// insumos ya no reproduciría la tarifa guardada.
type LiquidationsSdlRate struct {
	ID           string `gorm:"column:id;primaryKey;default:gen_random_uuid()::text"`
	LoadID       string `gorm:"column:load_id;not null"`
	Period       string `gorm:"column:period;not null"`
	OperatorCode string `gorm:"column:operator_code;not null"`

	// Nombre legal y mercado, guardados y no consultados: esta tabla vive en
	// calculator-prices y public.agents en file-compiler, así que un join entre las
	// dos no es posible. Es la misma decisión que en las tablas de Cargos STR.
	OperatorName string `gorm:"column:operator_name"`
	AgentCode    string `gorm:"column:agent_code"`
	Market       string `gorm:"column:market"`

	ActiveLevel1Operator float64 `gorm:"column:active_level_1_operator;not null"`
	ActiveLevel1Shared   float64 `gorm:"column:active_level_1_shared;not null"`
	ActiveLevel1User     float64 `gorm:"column:active_level_1_user;not null"`
	ActiveLevel2User     float64 `gorm:"column:active_level_2_user;not null"`
	ActiveLevel3User     float64 `gorm:"column:active_level_3_user;not null"`

	ReactiveLevel1Operator float64 `gorm:"column:reactive_level_1_operator;not null"`
	ReactiveLevel1Shared   float64 `gorm:"column:reactive_level_1_shared;not null"`
	ReactiveLevel1User     float64 `gorm:"column:reactive_level_1_user;not null"`
	ReactiveLevel2User     float64 `gorm:"column:reactive_level_2_user;not null"`
	ReactiveLevel3User     float64 `gorm:"column:reactive_level_3_user;not null"`

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (LiquidationsSdlRate) TableName() string {
	return "public.liquidations_sdl_rates"
}
