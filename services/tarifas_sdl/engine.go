// Package tarifas_sdl calcula las tarifas SDL a partir de los insumos que
// publica XM: los archivos "Cargos ADD" y los de "Uso de la red".
//
// ⚠️ TRASVASE: paquete propio, no colisiona con nada de bia-bills. Ver
// docs/backend/migracion-a-go.md.
package tarifas_sdl

// Motor de cálculo. Port del lib/engine/tarifas-sdl.ts de este mismo repo.
//
// Por cada operador de red y período se arman los componentes:
//
//   - NT1, NT2, NT3: cargo de uso de la red por nivel de tensión.
//     Operador tipo ADD → del archivo ADD, el DT de su ÁREA de distribución.
//     Operador tipo USO → del archivo de uso de la red del propio operador.
//   - CDI, CDN4, PR1, PR2, PR3: SIEMPRE del archivo de uso de la red.
//
// Y salen 10 tarifas por operador: 5 activas y 5 reactivas, una por cada
// combinación de nivel y propiedad de activos que existe (nivel 1 tiene las
// tres propiedades; niveles 2 y 3 solo usuario).

// TipoInsumo indica de dónde sale el NT de un operador.
type TipoInsumo string

const (
	InsumoADD TipoInsumo = "ADD"
	InsumoUSO TipoInsumo = "USO"
)

// AreaDistribucion es el área de los archivos ADD: hay un archivo por área y
// nivel, y los operadores tipo ADD toman el DT del área que les corresponde.
type AreaDistribucion string

const (
	AreaCentro    AreaDistribucion = "CENTRO"
	AreaOccidente AreaDistribucion = "OCCIDENTE"
	AreaOriente   AreaDistribucion = "ORIENTE"
	AreaSur       AreaDistribucion = "SUR"
)

// orTipo mapea cada operador al origen de su NT. Son los 21 del negocio.
//
// No es configuración: es una regla regulatoria sobre qué archivo publica el
// cargo de red de cada operador. Cambia cuando lo cambia XM, no por ambiente.
var orTipo = map[string]TipoInsumo{
	"AIRE": InsumoUSO, "AFINIA": InsumoUSO, "EEP_PEREIRA": InsumoUSO,
	"EEP_CARTAGO": InsumoUSO, "EMCALI": InsumoUSO, "ENEL": InsumoUSO,
	"EPM": InsumoUSO,

	"CEDENAR": InsumoADD, "CELSIA_VALLE": InsumoADD, "CELSIA_TOLIMA": InsumoADD,
	"CENS": InsumoADD, "CEO": InsumoADD, "CETSA": InsumoADD, "CHEC": InsumoADD,
	"EBSA": InsumoADD, "EDEQ": InsumoADD, "ELECTROHUILA": InsumoADD,
	"EMSA": InsumoADD, "ENERCA": InsumoADD, "ESSA": InsumoADD,
	"RUITOQUE": InsumoADD,
}

// orAreaADD es el área de distribución de cada operador tipo ADD.
var orAreaADD = map[string]AreaDistribucion{
	"CENS": AreaCentro, "CHEC": AreaCentro, "EDEQ": AreaCentro,
	"ESSA": AreaCentro, "RUITOQUE": AreaCentro,

	"CEDENAR": AreaOccidente, "CELSIA_VALLE": AreaOccidente,
	"CEO": AreaOccidente, "CETSA": AreaOccidente,

	"CELSIA_TOLIMA": AreaOriente, "EBSA": AreaOriente, "ELECTROHUILA": AreaOriente,

	"EMSA": AreaSur, "ENERCA": AreaSur,
}

// TipoDeOperador devuelve de dónde sale el NT de un operador, y si está en el
// catálogo.
func TipoDeOperador(codigo string) (TipoInsumo, bool) {
	tipo, ok := orTipo[codigo]
	return tipo, ok
}

// AreaDeOperador devuelve el área ADD de un operador tipo ADD. Para los tipo USO
// devuelve false: no tienen área porque no usan los archivos ADD.
func AreaDeOperador(codigo string) (AreaDistribucion, bool) {
	area, ok := orAreaADD[codigo]
	return area, ok
}

// OperatorCodes devuelve los códigos del catálogo, ordenados.
//
// Ordenados a propósito: el orden de iteración de un mapa en Go es aleatorio, y
// en Cargos STR eso ya produjo un resultado distinto en cada corrida.
func OperatorCodes() []string {
	return append([]string(nil), codigosOrdenados...)
}

// Componentes son las entradas del cálculo de un operador.
type Componentes struct {
	NT1, NT2, NT3 float64
	CDI, CDN4     float64

	// FRACCIONES, no porcentajes: 0.1255 es 12,55 %.
	//
	// Importa porque la fórmula hace CDN4/(1-PR). Con 12.55 en vez de 0.1255 el
	// divisor queda negativo y todas las tarifas activas salen absurdas, sin que
	// nada falle. En los archivos de XM la celda vale la fracción y el "%" es
	// solo formato de Excel.
	PR1, PR2, PR3 float64
}

// Tarifas son las 10 tarifas de un operador.
type Tarifas struct {
	Activa   TarifasPorNivel
	Reactiva TarifasPorNivel
}

// TarifasPorNivel: el nivel 1 se abre por propiedad de activos; los niveles 2 y
// 3 solo tienen tarifa de usuario.
type TarifasPorNivel struct {
	Nivel1Operador   float64
	Nivel1Compartido float64
	Nivel1Usuario    float64
	Nivel2Usuario    float64
	Nivel3Usuario    float64
}

// Calcular aplica las fórmulas del negocio.
//
//	ACTIVA
//	  nivel 1 operador    = NT1 - CDN4/(1-PR1)
//	  nivel 1 compartido  = (NT1 - CDN4/(1-PR1)) - CDI*0.5
//	  nivel 1 usuario     = (NT1 - CDN4/(1-PR1)) - CDI
//	  nivel 2 usuario     = NT2 - CDN4/(1-PR2)
//	  nivel 3 usuario     = NT3 - CDN4/(1-PR3)
//
//	REACTIVA
//	  nivel 1 operador    = NT1
//	  nivel 1 compartido  = NT1 - CDI*0.5
//	  nivel 1 usuario     = NT1 - CDI
//	  nivel 2 usuario     = NT2
//	  nivel 3 usuario     = NT3
//
// El orden de las operaciones se mantiene igual al del TypeScript: en punto
// flotante, reordenar cambia los últimos decimales, y estas tarifas alimentan la
// facturación.
func Calcular(c Componentes) Tarifas {
	activaNivel1Operador := c.NT1 - c.CDN4/(1-c.PR1)

	return Tarifas{
		Activa: TarifasPorNivel{
			Nivel1Operador:   activaNivel1Operador,
			Nivel1Compartido: activaNivel1Operador - c.CDI*0.5,
			Nivel1Usuario:    activaNivel1Operador - c.CDI,
			Nivel2Usuario:    c.NT2 - c.CDN4/(1-c.PR2),
			Nivel3Usuario:    c.NT3 - c.CDN4/(1-c.PR3),
		},
		Reactiva: TarifasPorNivel{
			Nivel1Operador:   c.NT1,
			Nivel1Compartido: c.NT1 - c.CDI*0.5,
			Nivel1Usuario:    c.NT1 - c.CDI,
			Nivel2Usuario:    c.NT2,
			Nivel3Usuario:    c.NT3,
		},
	}
}
