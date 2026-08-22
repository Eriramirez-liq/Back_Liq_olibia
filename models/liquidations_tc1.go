package models

import "time"

// Tabla de TC1.
//
// ⚠️ TRASVASE: archivo con prefijo `liquidations_`. Ver docs/backend/migracion-a-go.md.

// LiquidationsTc1Input es una frontera comercial tal como la reporta su operador
// de red, en la base file-compiler.
//
// ── A diferencia de STR y SDL, acá no se calcula nada ────────────────────────
// TC1 no produce valores derivados: no hay tabla espejo en calculator-prices. El
// archivo se normaliza y se guarda. Quien lo consume después —la conciliación
// contra Facturación, la congruencia— lo cruza por CodFronteraComercial.
//
// ── Todo string, a propósito ────────────────────────────────────────────────
// El layout de la CREG es una secuencia fija de campos, no un esquema tipado, y
// los archivos traen valores que solo parecen numéricos: la latitud llega como
// "35003989" en microgrados y varios operadores dejan campos vacíos en vez de
// cero. Convertir acá sería el único lugar donde se puede perder el valor
// original, y guardar lo que llegó es justamente para lo que existe esta tabla.
//
// ── APPEND-ONLY, con una diferencia respecto de STR y SDL ────────────────────
// Nada se pisa ni se borra. Pero acá lo vigente es el ÚLTIMO CARGUE, no la
// última fila: hay muchas filas por período y operador —una por frontera—, así
// que quedarse con la más reciente de cada una mezclaría fronteras de dos
// cargues distintos. Esa regla vive en el repositorio.
// Las etiquetas `json` importan tanto como las `gorm`: el endpoint de lectura
// devuelve este struct tal cual, y sin ellas Go serializa con los nombres de campo
// de Go —LoadID, CodFronteraComercial— que el front no sabe leer. Se usan los
// mismos nombres que las columnas, así lo que viaja y lo que se guarda se llaman
// igual y no hay traducción de por medio.
type LiquidationsTc1Input struct {
	ID           string `json:"id" gorm:"column:id;primaryKey;default:gen_random_uuid()::text"`
	LoadID       string `json:"load_id" gorm:"column:load_id;not null"`
	Period       string `json:"period" gorm:"column:period;not null"` // "YYYY-MM"
	OperatorCode string `json:"operator_code" gorm:"column:operator_code;not null"`

	// ── Las 33 columnas canónicas de TC1, en el orden de la CREG ───────────
	//
	// El orden importa y no es decoración: el parser mapea las columnas del
	// archivo a estas POR POSICIÓN, porque cada operador nombra sus encabezados
	// distinto. Reordenar esta lista rompe el mapeo sin que nada falle.
	//
	// Las etiquetas `gorm:"column:..."` van explícitas en todas. En Tarifas SDL
	// costó una sesión descubrir que sin ellas GORM deriva nombres que no
	// existen y el Scan deja el campo en su valor cero SIN fallar.
	Niu                       string `json:"niu" gorm:"column:niu"`
	CodigoDeConexion          string `json:"codigo_de_conexion" gorm:"column:codigo_de_conexion"`
	TipoDeConexion            string `json:"tipo_de_conexion" gorm:"column:tipo_de_conexion"`
	NivelDeTension            string `json:"nivel_de_tension" gorm:"column:nivel_de_tension"`
	NivelDeTensionPrimario    string `json:"nivel_de_tension_primario" gorm:"column:nivel_de_tension_primario"`
	PorcPropiedadDelActivo    string `json:"porc_propiedad_del_activo" gorm:"column:porc_propiedad_del_activo"`
	ConexionRed               string `json:"conexion_red" gorm:"column:conexion_red"`
	IDComercializador         string `json:"id_comercializador" gorm:"column:id_comercializador"`
	IDMercado                 string `json:"id_mercado" gorm:"column:id_mercado"`
	GrupoDeCalidad            string `json:"grupo_de_calidad" gorm:"column:grupo_de_calidad"`
	CodFronteraComercial      string `json:"cod_frontera_comercial" gorm:"column:cod_frontera_comercial;not null"`
	CodigoCircuitoOLinea      string `json:"codigo_circuito_o_linea" gorm:"column:codigo_circuito_o_linea"`
	CodigoTransformador       string `json:"codigo_transformador" gorm:"column:codigo_transformador"`
	CodigoDaneNiu             string `json:"codigo_dane_niu" gorm:"column:codigo_dane_niu"`
	Ubicacion                 string `json:"ubicacion" gorm:"column:ubicacion"`
	Direccion                 string `json:"direccion" gorm:"column:direccion"`
	CondicionEspecial         string `json:"condicion_especial" gorm:"column:condicion_especial"`
	TipoAreaEspecial          string `json:"tipo_area_especial" gorm:"column:tipo_area_especial"`
	CodigoAreaEspecial        string `json:"codigo_area_especial" gorm:"column:codigo_area_especial"`
	EstratoID                 string `json:"estrato_id" gorm:"column:estrato_id"`
	Altitud                   string `json:"altitud" gorm:"column:altitud"`
	Longitud                  string `json:"longitud" gorm:"column:longitud"`
	Latitud                   string `json:"latitud" gorm:"column:latitud"`
	Autogenerador             string `json:"autogenerador" gorm:"column:autogenerador"`
	ExportaEnergia            string `json:"exporta_energia" gorm:"column:exporta_energia"`
	Potencia                  string `json:"potencia" gorm:"column:potencia"`
	TipoGeneracion            string `json:"tipo_generacion" gorm:"column:tipo_generacion"`
	CodigoFronteraAutoGen     string `json:"codigo_frontera_auto_gen" gorm:"column:codigo_frontera_auto_gen"`
	InicioOperacion           string `json:"inicio_operacion" gorm:"column:inicio_operacion"`
	ContratoRespaldo          string `json:"contrato_respaldo" gorm:"column:contrato_respaldo"`
	CapacidadContratoRespaldo string `json:"capacidad_contrato_respaldo" gorm:"column:capacidad_contrato_respaldo"`
	Ciclo                     string `json:"ciclo" gorm:"column:ciclo"`
	Nodo                      string `json:"nodo" gorm:"column:nodo"`

	SourceFile  string    `json:"source_file" gorm:"column:source_file"`
	CreatedBy   string    `json:"created_by" gorm:"column:created_by"`
	CreatedByID string    `json:"created_by_id" gorm:"column:created_by_id"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
}

func (LiquidationsTc1Input) TableName() string {
	return "public.liquidations_tc1_inputs"
}
