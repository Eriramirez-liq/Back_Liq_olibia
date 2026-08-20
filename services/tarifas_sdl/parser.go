package tarifas_sdl

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Parser de los insumos de Tarifas SDL: los archivos "Cargos ADD" y los de
// "Uso de la red" que publica XM.
//
// ⚠️ TRASVASE: archivo del paquete tarifas_sdl. Ver docs/backend/migracion-a-go.md.
//
// ── Todo se ancla por NOMBRE, nada por posición ──────────────────────────────
// La versión TypeScript leía la primera hoja del libro y tomaba "el primer número
// puro mayor que 1". Funcionaba, pero por casualidad: el archivo ADD tiene DIEZ
// hojas, y otras dos de ellas traen valores distintos para el mismo operador
// (288.75 y 283.89 donde el correcto es 315.70). Una reordenación de hojas
// bastaba para leer el número equivocado sin que nada fallara.
//
// Acá la hoja, la columna y la fila se ubican por su nombre, y si alguno no está
// se corta con error crítico en vez de adivinar.
//
// ── Y nada devuelve cero cuando no encuentra algo ────────────────────────────
// El otro problema de la versión anterior: cuando un marcador no aparecía, el
// valor quedaba en 0. Cero es plausible, no imposible, y produce tarifas
// creíbles y equivocadas — sin CDI no hay descuento, sin CDN4 la activa iguala a
// la reactiva. Todo eso ahora es error crítico.

// Nombres del formato de XM. Si XM los cambia, el parser falla de forma visible
// en vez de leer otro número: es un contrato explícito con el proveedor.
const (
	hojaAddPrefijo   = "CARGOS ADD"
	hojaUso          = "Cargos_Definitivos"
	columnaAdd       = "CARGO UNICO TRANSITORIO"
	columnaAddOtra   = "CARGO ÚNICO TRANSITORIO" // por si llega sin normalizar
	columnaUso       = "CARGO MONOMIO"
	encabezadoOperad = "OPERADOR"
	marcadorCDI      = "CDI"
	marcadorNivel4   = "NIVEL4"
)

// Marcadores de fin del bloque que se suma para CDI y CDN4.
var marcadoresFin = []string{"PLANES DE GESTION", "COBRO DE LA REMUNERACION"}

// UploadedFile es un archivo del lote.
type UploadedFile struct {
	Name    string
	Content []byte
}

// SdlInputRow es una fila de la tabla de insumos: un operador de red con todo lo
// que alimenta su cálculo.
//
// Guarda los DOS juegos de DT —los del ADD de su área y los de su propio archivo
// de uso de la red— aunque el cálculo solo use uno según el tipo de operador. Es
// a propósito: permite auditar por qué una tarifa salió como salió.
type SdlInputRow struct {
	OperatorCode string `json:"operator_code"`

	// Código de agente que trae el archivo y mercado que atiende. El agente
	// resuelve el nombre legal contra public.agents; el mercado distingue a los dos
	// operadores que comparten razón social (Celsia Valle/Tolima y EEP
	// Pereira/Cartago), que sin él parecerían filas duplicadas.
	AgentCode string `json:"agent_code"`
	Market    string `json:"market"`
	// Nombre legal, lo completa el servicio contra public.agents.
	OperatorName string `json:"operator_name,omitempty"`

	DistributionArea string `json:"distribution_area"` // vacío en los tipo USO

	// Del ADD del área. nil en los operadores tipo USO, que no tienen área.
	DT1Add *float64 `json:"dt1_add"`
	DT2Add *float64 `json:"dt2_add"`
	DT3Add *float64 `json:"dt3_add"`

	// Del archivo de uso de la red del propio operador.
	DT1  float64 `json:"dt1"`
	DT2  float64 `json:"dt2"`
	DT3  float64 `json:"dt3"`
	CDI  float64 `json:"cdi"`
	CDN4 float64 `json:"cdn4"`

	// Fracciones, no porcentajes. Ver Componentes.
	PR1 float64 `json:"pr1"`
	PR2 float64 `json:"pr2"`
	PR3 float64 `json:"pr3"`

	// Los archivos que produjeron esta fila: el de uso de la red del operador y
	// los del ADD de su área. Para poder rastrear una tarifa hasta su origen.
	SourceFiles []string `json:"source_files"`
}

// ParseResult acompaña las filas con lo que hay que contarle al usuario.
type ParseResult struct {
	Rows           []SdlInputRow `json:"rows"`
	Warnings       []string      `json:"warnings"`
	CriticalErrors []string      `json:"critical_errors"`
}

// datosAdd es el cargo de un área y nivel.
type datosAdd struct {
	valor   float64
	archivo string
}

// addDelArchivo es lo que se saca de un archivo ADD: el cargo del área-nivel y
// los mercados que la hoja lista, que son los que definen qué operadores
// pertenecen al área.
type addDelArchivo struct {
	area     AreaDistribucion
	nivel    int
	valor    float64
	mercados []string
}

// datosUso son los componentes del archivo de uso de la red de un operador.
type datosUso struct {
	dt1, dt2, dt3 float64
	cdi, cdn4     float64
	pr1, pr2, pr3 float64
	archivo       string
	agente        string // código de agente que trae el archivo
	mercado       string // mercado de comercialización
}

// periodoDelNombreUso saca el período de un archivo de uso de la red, que lo trae
// al final del nombre: "Cargo_Cobro_Uso_Red-DefinitivoCALM-202604.xlsx" → "2026-04".
//
// Devuelve "" si el nombre no lo trae en ese formato.
func periodoDelNombreUso(nombre string) string {
	m := periodoEnNombreUso.FindStringSubmatch(nombre)
	if m == nil {
		return ""
	}

	mes, err := strconv.Atoi(m[2])
	if err != nil || mes < 1 || mes > 12 {
		return ""
	}

	return fmt.Sprintf("%s-%02d", m[1], mes)
}

var periodoEnNombreUso = regexp.MustCompile(`-(20\d{2})(\d{2})\b`)

// validarPeriodoDeUso corta la carga si algún archivo de uso de la red no es del
// período elegido.
//
// Los archivos de uso de la red SÍ tienen que ser del mes que se está liquidando.
// Los ADD no: van sistemáticamente dos meses atrás, y por eso no se validan —era
// justo la parte de "unos archivos son de meses anteriores".
//
// CORTA y no avisa. Por el modelo append-only, un cargue con los archivos del mes
// equivocado no se deshace desde la pantalla: quedan las tarifas de un mes
// guardadas bajo otro, y el error recién aparece cuando alguien liquida. En los
// cargues reales ya hay uno así: período 2026-07 con los archivos de junio.
//
// Si no llega el período —un front viejo contra un backend nuevo— no se valida,
// pero se dice: callarlo dejaría pasar justo lo que esto viene a evitar.
func validarPeriodoDeUso(period string, files []UploadedFile) (errores, avisos []string) {
	if strings.TrimSpace(period) == "" {
		return nil, []string{
			"No llegó el período al preview, así que no se pudo verificar contra los archivos de " +
				"uso de la red. Recargá la página; si sigue igual, el front está desactualizado.",
		}
	}

	desajustes := []string{}
	for _, f := range files {
		if clasificar(f.Name) != "USO" {
			continue
		}
		delArchivo := periodoDelNombreUso(f.Name)
		if delArchivo == "" {
			avisos = append(avisos, fmt.Sprintf(
				"[%s] no dice en su nombre de qué período es, así que no se pudo verificar contra "+
					"el seleccionado (%s).", f.Name, period))
			continue
		}
		if delArchivo != period {
			desajustes = append(desajustes, fmt.Sprintf("%q es de %s", f.Name, delArchivo))
		}
	}

	if len(desajustes) > 0 {
		// Se listan como mucho tres: son 21 archivos y si el mes está mal suelen
		// estarlo todos. La lista completa no agrega nada y tapa el mensaje.
		muestra := desajustes
		sufijo := ""
		if len(muestra) > 3 {
			muestra, sufijo = muestra[:3], fmt.Sprintf(" y %d más", len(desajustes)-3)
		}
		errores = append(errores, fmt.Sprintf(
			"El período seleccionado es %s y los archivos de uso de la red no son de ese mes: %s%s. "+
				"Si los archivos son los que corresponden, corregí el período en Nueva carga; si no, "+
				"cargá los de %s.",
			period, strings.Join(muestra, ", "), sufijo, period))
	}

	return errores, avisos
}

// ParseInputs arma una fila por operador de red a partir del lote.
//
// El período lo elige la persona en Nueva carga. Sirve para dos cosas: rotula el
// cargue, y se contrasta contra los archivos de uso de la red, que tienen que ser
// de ese mes.
func ParseInputs(files []UploadedFile, period string) ParseResult {
	res := ParseResult{Rows: []SdlInputRow{}, Warnings: []string{}, CriticalErrors: []string{}}

	errPeriodo, avisosPeriodo := validarPeriodoDeUso(period, files)
	res.Warnings = append(res.Warnings, avisosPeriodo...)
	if len(errPeriodo) > 0 {
		res.CriticalErrors = append(res.CriticalErrors, errPeriodo...)
		return res
	}

	addPorArea := map[AreaDistribucion]map[int]datosAdd{}
	usoPorOperador := map[string]datosUso{}
	// Mercado (normalizado) → área. Se arma con lo que listan las hojas "Cargos
	// ADD", que es la única fuente de la pertenencia de un operador a un área.
	areaPorMercado := map[string]AreaDistribucion{}

	for _, f := range files {
		switch clasificar(f.Name) {
		case "ADD":
			add, err := parsearAdd(f)
			if err != nil {
				res.CriticalErrors = append(res.CriticalErrors, err.Error())
				continue
			}
			area, nivel := add.area, add.nivel
			if addPorArea[area] == nil {
				addPorArea[area] = map[int]datosAdd{}
			}
			// Duplicado: dos archivos del mismo área y nivel. Antes el segundo
			// sobrescribía al primero en silencio, y basta con que quede un
			// archivo viejo en la carpeta.
			if previo, existe := addPorArea[area][nivel]; existe {
				res.CriticalErrors = append(res.CriticalErrors, fmt.Sprintf(
					"Hay dos archivos ADD para %s nivel %d: %q y %q. Dejá solo el del período que estás cargando.",
					area, nivel, previo.archivo, f.Name))
				continue
			}
			addPorArea[area][nivel] = datosAdd{valor: add.valor, archivo: f.Name}

			// Un mercado en dos áreas distintas no puede ser: significa que hay
			// archivos de períodos distintos, o que XM movió un operador y llegó
			// el lote a medio actualizar.
			for _, mercado := range add.mercados {
				clave := normalizar(mercado)
				if previa, existe := areaPorMercado[clave]; existe && previa != area {
					res.CriticalErrors = append(res.CriticalErrors, fmt.Sprintf(
						"El mercado %q aparece en dos áreas: %s y %s. Revisá que los archivos ADD sean todos del mismo período.",
						mercado, previa, area))
					continue
				}
				areaPorMercado[clave] = area
			}

		case "USO":
			operador, datos, err := parsearUso(f)
			if err != nil {
				res.CriticalErrors = append(res.CriticalErrors, err.Error())
				continue
			}
			if _, conocido := TipoDeOperador(operador); !conocido {
				res.CriticalErrors = append(res.CriticalErrors, fmt.Sprintf(
					"El archivo %q corresponde al operador %q, que no está en el catálogo de operadores de red.",
					f.Name, operador))
				continue
			}
			if previo, existe := usoPorOperador[operador]; existe {
				res.CriticalErrors = append(res.CriticalErrors, fmt.Sprintf(
					"Dos archivos de uso de la red resuelven al operador %s: %q y %q.",
					operador, previo.archivo, f.Name))
				continue
			}
			usoPorOperador[operador] = datos

		default:
			res.Warnings = append(res.Warnings, fmt.Sprintf(
				"Archivo omitido, no es ADD ni de uso de la red: %s", f.Name))
		}
	}

	// ── Cobertura y coherencia ──────────────────────────────────────────────
	res.CriticalErrors = append(res.CriticalErrors, validarAdd(addPorArea)...)
	res.CriticalErrors = append(res.CriticalErrors, validarCoberturaUso(usoPorOperador)...)
	res.CriticalErrors = append(res.CriticalErrors,
		validarAreaDeTipoAdd(usoPorOperador, areaPorMercado)...)

	if len(res.CriticalErrors) > 0 {
		return res
	}

	res.Warnings = append(res.Warnings, avisarSinArea(usoPorOperador, areaPorMercado)...)

	// ── Una fila por operador ───────────────────────────────────────────────
	for _, codigo := range OperatorCodes() {
		uso := usoPorOperador[codigo]
		fila := SdlInputRow{
			OperatorCode: codigo,
			AgentCode:    uso.agente,
			Market:       uso.mercado,
			DT1:          uso.dt1, DT2: uso.dt2, DT3: uso.dt3,
			CDI: uso.cdi, CDN4: uso.cdn4,
			PR1: uso.pr1, PR2: uso.pr2, PR3: uso.pr3,
			SourceFiles: []string{uso.archivo},
		}

		// El área y los cargos del ADD se guardan para TODO operador que figure en
		// una hoja "Cargos ADD", sin mirar su tipo. Son un dato del operador, no un
		// insumo exclusivo del cálculo: los tipo USO también pertenecen a un área y
		// tienen cargos ADD, aunque su tarifa se calcule con los DT de su propio
		// archivo de uso de la red. Quién entra al cálculo lo decide ComponentesDe.
		//
		// La pertenencia se resuelve por nombre de mercado, que es lo único que
		// distingue a los operadores que comparten razón social: EEP figura como
		// "EEP Mercado de Comercialización PEREIRA" en Centro y como "…CARTAGO" en
		// Occidente, y Celsia como VALLE DEL CAUCA en Occidente y TOLIMA en Oriente.
		// El nombre del operador no alcanzaría: los dos dicen "EEP".
		if area, enUnArea := areaPorMercado[normalizar(uso.mercado)]; enUnArea {
			fila.DistributionArea = string(area)

			porNivel := addPorArea[area]
			dt1, dt2, dt3 := porNivel[1].valor, porNivel[2].valor, porNivel[3].valor
			fila.DT1Add, fila.DT2Add, fila.DT3Add = &dt1, &dt2, &dt3
			fila.SourceFiles = append(fila.SourceFiles,
				porNivel[1].archivo, porNivel[2].archivo, porNivel[3].archivo)
		}

		res.Rows = append(res.Rows, fila)
	}

	return res
}

// ComponentesDe arma las entradas del cálculo de una fila de insumos.
//
// Acá vive la regla de qué archivo manda: el NT sale del ADD del área para los
// operadores tipo ADD, y del propio archivo de uso de la red para los tipo USO.
// El CDI, el CDN4 y los PR salen SIEMPRE del archivo de uso de la red.
func ComponentesDe(fila SdlInputRow) (Componentes, error) {
	c := Componentes{
		CDI: fila.CDI, CDN4: fila.CDN4,
		PR1: fila.PR1, PR2: fila.PR2, PR3: fila.PR3,
	}

	tipo, conocido := TipoDeOperador(fila.OperatorCode)
	if !conocido {
		return c, fmt.Errorf("el operador %q no está en el catálogo", fila.OperatorCode)
	}

	if tipo == InsumoADD {
		if fila.DT1Add == nil || fila.DT2Add == nil || fila.DT3Add == nil {
			return c, fmt.Errorf(
				"el operador %s es tipo ADD y le faltan los DT del ADD de su área", fila.OperatorCode)
		}
		c.NT1, c.NT2, c.NT3 = *fila.DT1Add, *fila.DT2Add, *fila.DT3Add
		return c, nil
	}

	c.NT1, c.NT2, c.NT3 = fila.DT1, fila.DT2, fila.DT3
	return c, nil
}

// ── Validaciones ────────────────────────────────────────────────────────────

func validarAdd(addPorArea map[AreaDistribucion]map[int]datosAdd) []string {
	errores := []string{}
	areas := []AreaDistribucion{AreaCentro, AreaOccidente, AreaOriente, AreaSur}

	for _, area := range areas {
		porNivel := addPorArea[area]

		faltan := []string{}
		for _, nivel := range []int{1, 2, 3} {
			if _, ok := porNivel[nivel]; !ok {
				faltan = append(faltan, strconv.Itoa(nivel))
			}
		}
		if len(faltan) > 0 {
			errores = append(errores, fmt.Sprintf(
				"Falta el archivo ADD de %s para el nivel %s. Sin él, los operadores de esa área quedan sin tarifa.",
				area, strings.Join(faltan, ", ")))
			continue
		}

		// Menor nivel de tensión, mayor cargo por kWh. Se cumple en los cuatro
		// áreas de los archivos reales; si se rompe, se leyó la columna o la hoja
		// equivocada.
		dt1, dt2, dt3 := porNivel[1].valor, porNivel[2].valor, porNivel[3].valor
		if !(dt1 > dt2 && dt2 > dt3) {
			errores = append(errores, fmt.Sprintf(
				"Los cargos ADD de %s no decrecen por nivel (%.6f, %.6f, %.6f). "+
					"Revisá que los archivos sean del mismo período y que la columna leída sea la correcta.",
				area, dt1, dt2, dt3))
		}
	}

	return errores
}

// validarAreaDeTipoAdd verifica que los operadores cuyo NT sale del ADD figuren en
// alguna hoja "Cargos ADD". Antes esto lo garantizaba un mapa fijo en el código;
// ahora la pertenencia sale de los archivos, así que hay que comprobarla.
//
// Es error crítico y no aviso: sin área no tienen de dónde tomar el NT y la fila
// saldría con las diez tarifas en cero, que es peor que no cargar.
func validarAreaDeTipoAdd(
	usoPorOperador map[string]datosUso,
	areaPorMercado map[string]AreaDistribucion,
) []string {
	faltan := []string{}
	for _, codigo := range OperatorCodes() {
		if tipo, _ := TipoDeOperador(codigo); tipo != InsumoADD {
			continue
		}
		uso, tieneUso := usoPorOperador[codigo]
		if !tieneUso {
			continue // ya lo reporta validarCoberturaUso
		}
		if _, enArea := areaPorMercado[normalizar(uso.mercado)]; !enArea {
			faltan = append(faltan, fmt.Sprintf("%s (mercado %q)", codigo, uso.mercado))
		}
	}
	if len(faltan) == 0 {
		return nil
	}

	return []string{fmt.Sprintf(
		"Estos operadores toman su cargo de red del ADD pero no figuran en ninguna hoja \"Cargos ADD\" "+
			"del lote: %s. Verificá que estén los 12 archivos del período y que el mercado del archivo "+
			"de uso de la red coincida con el del ADD.", strings.Join(faltan, ", "))}
}

// avisarSinArea deja constancia de los operadores que no figuran en ningún archivo
// ADD del lote.
//
// No es error: son tipo USO, calculan su tarifa con los DT de su propio archivo y
// no necesitan el ADD para nada. Pero el área y los cargos ADD quedan vacíos en la
// tabla de insumos, y eso se lee como un dato que falta. El aviso lo hace explícito
// en el preview, antes de guardar.
func avisarSinArea(
	usoPorOperador map[string]datosUso,
	areaPorMercado map[string]AreaDistribucion,
) []string {
	sinArea := []string{}
	for _, codigo := range OperatorCodes() {
		uso, tieneUso := usoPorOperador[codigo]
		if !tieneUso {
			continue
		}
		if _, enArea := areaPorMercado[normalizar(uso.mercado)]; !enArea {
			sinArea = append(sinArea, fmt.Sprintf("%s (mercado %q)", codigo, uso.mercado))
		}
	}
	if len(sinArea) == 0 {
		return nil
	}

	return []string{fmt.Sprintf(
		"Sin área de distribución, porque no figuran en ninguna hoja \"Cargos ADD\" del lote: %s. "+
			"Su tarifa se calcula igual, con los cargos de su propio archivo de uso de la red.",
		strings.Join(sinArea, ", "))}
}

func validarCoberturaUso(usoPorOperador map[string]datosUso) []string {
	faltan := []string{}
	for _, codigo := range OperatorCodes() {
		if _, ok := usoPorOperador[codigo]; !ok {
			faltan = append(faltan, codigo)
		}
	}
	if len(faltan) == 0 {
		return nil
	}

	// Faltar un operador NO es una advertencia: ese operador se quedaría sin
	// tarifa nueva y, por el modelo append-only, seguiría mostrando la del
	// período anterior sin que nadie lo note.
	return []string{fmt.Sprintf(
		"Faltan los archivos de uso de la red de %d operadores: %s.",
		len(faltan), strings.Join(faltan, ", "))}
}

// ── Lectura de los archivos ─────────────────────────────────────────────────

func clasificar(nombre string) string {
	n := normalizar(nombre)
	switch {
	case strings.Contains(n, "LIQUIDACIONDEFINITIVO"):
		return "ADD"
	case strings.Contains(n, "CARGO_COBRO_USO_RED"), strings.Contains(n, "CARGOCOBROUSORED"):
		return "USO"
	default:
		return ""
	}
}

func areaDelNombre(nombre string) (AreaDistribucion, bool) {
	n := normalizar(nombre)
	for _, area := range []AreaDistribucion{AreaCentro, AreaOccidente, AreaOriente, AreaSur} {
		if strings.Contains(n, string(area)) {
			return area, true
		}
	}
	return "", false
}

func nivelDelNombre(nombre string) (int, bool) {
	n := normalizar(nombre)
	for _, nivel := range []int{1, 2, 3} {
		if strings.Contains(n, fmt.Sprintf("NIVEL%d", nivel)) {
			return nivel, true
		}
	}
	return 0, false
}

// parsearAdd lee el cargo de un archivo ADD.
//
// El valor es el mismo para todos los operadores del archivo —es un cargo de
// área, no de operador— y eso se verifica. Es lo que distingue la hoja correcta:
// en "Cargos Dt" y "Cargos Transitorios" del mismo libro el valor VARÍA por
// operador, así que si alguien apunta a la hoja equivocada, esta comprobación lo
// detecta.
func parsearAdd(f UploadedFile) (addDelArchivo, error) {
	vacio := addDelArchivo{}

	area, okArea := areaDelNombre(f.Name)
	nivel, okNivel := nivelDelNombre(f.Name)
	if !okArea || !okNivel {
		return vacio, fmt.Errorf(
			"del nombre del archivo ADD %q no se pudo deducir el área y el nivel", f.Name)
	}

	libro, err := abrir(f)
	if err != nil {
		return vacio, err
	}
	defer func() { _ = libro.Close() }()

	hoja, err := hojaPorPrefijo(libro, hojaAddPrefijo, f.Name)
	if err != nil {
		return vacio, err
	}

	filas, err := libro.GetRows(hoja, excelize.Options{RawCellValue: true})
	if err != nil {
		return vacio, fmt.Errorf("no se pudo leer la hoja %q de %q: %v", hoja, f.Name, err)
	}

	// Fila de encabezado y columnas, todas por nombre.
	filaHdr := -1
	colValor := -1
	colOperador := -1
	for i, fila := range filas {
		if !contieneNormalizado(fila, encabezadoOperad) {
			continue
		}
		filaHdr = i
		for c, celda := range fila {
			n := normalizar(celda)
			if colValor < 0 && (strings.Contains(n, columnaAdd) || strings.Contains(n, normalizar(columnaAddOtra))) {
				colValor = c
			}
			if colOperador < 0 && strings.Contains(n, encabezadoOperad) {
				colOperador = c
			}
		}
		break
	}
	if filaHdr < 0 {
		return vacio, fmt.Errorf(
			"en %q, hoja %q, no se encontró la fila de encabezado (ninguna celda dice %q)",
			f.Name, hoja, encabezadoOperad)
	}
	if colValor < 0 {
		return vacio, fmt.Errorf(
			"en %q, hoja %q, no se encontró la columna %q. Encabezados encontrados: %s",
			f.Name, hoja, columnaAddOtra, strings.Join(noVacias(filas[filaHdr]), " | "))
	}

	// Cada fila de datos trae el operador con su mercado y el cargo del área. El
	// mercado es lo que después resuelve a qué operador pertenece la fila.
	valores := []float64{}
	mercados := []string{}
	for i := filaHdr + 1; i < len(filas); i++ {
		v, ok := numero(celda(filas, i, colValor))
		if !ok {
			continue
		}
		valores = append(valores, v)

		if mercado := mercadoDeEtiqueta(celda(filas, i, colOperador)); mercado != "" {
			mercados = append(mercados, mercado)
		}
	}
	if len(valores) == 0 {
		return vacio, fmt.Errorf(
			"en %q, hoja %q, la columna %q no tiene ningún valor numérico", f.Name, hoja, columnaAddOtra)
	}

	// Si la hoja lista operadores pero no se les pudo leer el mercado, el área
	// quedaría sin operadores y nadie tendría cargos ADD. Falla explícito: es un
	// cambio de formato en la etiqueta del operador.
	if len(mercados) == 0 {
		return vacio, fmt.Errorf(
			"en %q, hoja %q, no se pudo leer el mercado de ningún operador. La etiqueta esperada es "+
				"algo como \"CENS Mercado de Comercialización NORTE DE SANTANDER\" y se leyó %q",
			f.Name, hoja, celda(filas, filaHdr+1, colOperador))
	}

	distintos := map[string]bool{}
	for _, v := range valores {
		distintos[strconv.FormatFloat(v, 'f', 9, 64)] = true
	}
	if len(distintos) > 1 {
		return vacio, fmt.Errorf(
			"en %q, hoja %q, la columna %q trae %d valores distintos y debería ser el mismo cargo para "+
				"todos los operadores del área. Probablemente se está leyendo la hoja o la columna equivocada.",
			f.Name, hoja, columnaAddOtra, len(distintos))
	}

	return addDelArchivo{area: area, nivel: nivel, valor: valores[0], mercados: mercados}, nil
}

// Mercado de comercialización → operador de red.
//
// Cada archivo de uso de la red corresponde a UN mercado, y su código está en el
// nombre del archivo. Los 21 son únicos, así que esta tabla resuelve el operador
// sin ambigüedad.
//
// ── Por qué explícito y no deducido del título ───────────────────────────────
// La versión anterior recortaba la celda B1 con separadores. Eso falla en casos
// reales: para Air-e, partir "CSID - AIR-E Mercado…" por el guion da "AIR_E" y no
// "AIRE". Andaba solo porque los casos raros estaban en un mapa como este, pero
// más chico — o sea que la deducción nunca fue la que resolvía de verdad.
//
// ── Dos agentes con dos mercados cada uno ────────────────────────────────────
// EEPD atiende Pereira (PEIM) y Cartago (CRCM); EPSD atiende Valle (VACM) y
// Tolima (TOLM). Son la MISMA razón social operando dos mercados, y el negocio los
// quiere separados: son cuatro filas, no dos. Por eso la clave es el mercado y no
// el agente.
//
// Los códigos coinciden con los de Cargos STR para los 17 operadores que ambos
// módulos comparten, así que las dos tablas hablan el mismo idioma.
var mercadoOperador = map[string]string{
	"MARM": "AFINIA",
	"SOLM": "AIRE",
	"NARM": "CEDENAR",
	"TOLM": "CELSIA_TOLIMA",
	"VACM": "CELSIA_VALLE",
	"NSAM": "CENS",
	"CAUM": "CEO",
	"ULQM": "CETSA",
	"CALM": "CHEC",
	"BOYM": "EBSA",
	"QUIM": "EDEQ",
	"CRCM": "EEP_CARTAGO",
	"PEIM": "EEP_PEREIRA",
	"HUIM": "ELECTROHUILA",
	"CLOM": "EMCALI",
	"METM": "EMSA",
	"CUNM": "ENEL",
	"CASM": "ENERCA",
	"ANTM": "EPM",
	"SANM": "ESSA",
	"RUIM": "RUITOQUE",
}

// Alias de código de agente para resolver el nombre.
//
// El archivo de Air-e viene con CSID, pero en public.agents ese código está como
// actividad DISTRIBUCIÓN y con el nombre "AIR- E S.A.S. E.S.P. - INTERVENIDO". El
// operador de red vigente es CSSD, con el nombre limpio.
//
// Sin este alias, Tarifas SDL mostraría "AIR- E S.A.S. E.S.P. - INTERVENIDO" y
// Cargos STR "AIR- E S.A.S. E.S.P." para el mismo operador. Cargos STR ya resuelve
// esto filtrando por actividad; acá hace falta el alias porque el código llega
// desde el archivo.
var aliasAgente = map[string]string{
	"CSID": "CSSD",
}

// AgentCodeFor devuelve el código de agente con el que se resuelve el nombre.
func AgentCodeFor(codigoDelArchivo string) string {
	if alias, hay := aliasAgente[codigoDelArchivo]; hay {
		return alias
	}
	return codigoDelArchivo
}

// parsearUso lee los componentes del archivo de uso de la red de un operador.
func parsearUso(f UploadedFile) (string, datosUso, error) {
	var vacio datosUso

	libro, err := abrir(f)
	if err != nil {
		return "", vacio, err
	}
	defer func() { _ = libro.Close() }()

	filas, err := libro.GetRows(hojaUso, excelize.Options{RawCellValue: true})
	if err != nil || len(filas) == 0 {
		return "", vacio, fmt.Errorf(
			"en %q no se pudo leer la hoja %q, que es la que trae los cargos definitivos", f.Name, hojaUso)
	}

	operador, agente, mercado, err := identidadDelUso(f.Name, filas)
	if err != nil {
		return "", vacio, err
	}

	datos := datosUso{archivo: f.Name, agente: agente, mercado: mercado}

	// Columna del cargo, por encabezado.
	colCargo := -1
	for _, fila := range filas {
		for c, celda := range fila {
			if strings.Contains(normalizar(celda), columnaUso) {
				colCargo = c
				break
			}
		}
		if colCargo >= 0 {
			break
		}
	}
	if colCargo < 0 {
		return "", vacio, fmt.Errorf(
			"en %q no se encontró la columna %q", f.Name, columnaUso)
	}

	// DT por nivel, por la etiqueta de la fila y no por su posición.
	for _, nivel := range []int{1, 2, 3} {
		etiqueta := fmt.Sprintf("NIVEL %d", nivel)
		valor, ok := porEtiqueta(filas, etiqueta, colCargo)
		if !ok {
			return "", vacio, fmt.Errorf(
				"en %q no se encontró la fila del cargo de uso de la red del nivel %d", f.Name, nivel)
		}
		switch nivel {
		case 1:
			datos.dt1 = valor
		case 2:
			datos.dt2 = valor
		case 3:
			datos.dt3 = valor
		}
	}

	// CDI y CDN4: se suma la columna del propio marcador, desde debajo del
	// marcador hasta el fin del bloque.
	//
	// Antes, si el marcador no aparecía el valor quedaba en 0 — y un CDI en cero
	// significa "sin descuento", que es una tarifa creíble y equivocada.
	cdi, err := sumarBloque(filas, marcadorCDI, f.Name)
	if err != nil {
		return "", vacio, err
	}
	datos.cdi = cdi

	cdn4, err := sumarBloque(filas, marcadorNivel4, f.Name)
	if err != nil {
		return "", vacio, err
	}
	datos.cdn4 = cdn4

	// PR: fracciones. Si faltan, antes quedaban en 0 y CDN4/(1-0) daba un número
	// creíble.
	for i, etq := range []string{"PR1", "PR2", "PR3"} {
		valor, ok := porEtiquetaExacta(filas, etq)
		if !ok {
			return "", vacio, fmt.Errorf("en %q no se encontró la fila %s", f.Name, etq)
		}
		// El guardián contra el peor error posible: si el archivo trajera el PR
		// como porcentaje (13.8 en vez de 0.138), la fórmula CDN4/(1-PR) daría un
		// divisor negativo y todas las tarifas activas saldrían absurdas.
		if valor < 0 || valor >= 1 {
			return "", vacio, fmt.Errorf(
				"en %q, %s vale %.6f y debería ser una fracción entre 0 y 1 "+
					"(13,8%% se guarda como 0.138, no como 13.8)", f.Name, etq, valor)
		}
		switch i {
		case 0:
			datos.pr1 = valor
		case 1:
			datos.pr2 = valor
		case 2:
			datos.pr3 = valor
		}
	}

	return operador, datos, nil
}

// identidadDelUso resuelve quién es el archivo: operador, código de agente y
// mercado.
//
// El operador sale del código de MERCADO del nombre del archivo, que es único. El
// código de agente y el mercado salen de la celda del título, que los trae los
// dos:
//
//	"EEPD - EEP Mercado de Comercialización - CARTAGO"
//	 ^^^^ agente                             ^^^^^^^ mercado
//
// El agente es lo que resuelve el nombre legal contra public.agents, igual que en
// Cargos STR. Así los nombres son idénticos en las dos tablas por construcción y
// no por coincidencia.
func identidadDelUso(nombre string, filas [][]string) (operador, agente, mercado string, err error) {
	cod := codigoDeMercado(nombre)
	if cod == "" {
		return "", "", "", fmt.Errorf(
			"del nombre del archivo %q no se pudo extraer el código de mercado "+
				"(se esperaba algo como Cargo_Cobro_Uso_Red-DefinitivoCALM-202604.xlsx)", nombre)
	}

	operador, conocido := mercadoOperador[cod]
	if !conocido {
		// Antes esto se deducía de la celda del título y podía dar un operador
		// inventado, que después quedaba fuera del catálogo sin explicación.
		return "", "", "", fmt.Errorf(
			"el archivo %q corresponde al mercado %q, que no está en la tabla de mercados. "+
				"Si es un mercado nuevo hay que agregarlo al catálogo.", nombre, cod)
	}

	titulo := celda(filas, 0, 1)

	// El agente es lo que va antes del primer guion.
	if partes := strings.SplitN(titulo, "-", 2); len(partes) == 2 {
		agente = strings.ToUpper(strings.TrimSpace(partes[0]))
	}
	if agente == "" {
		return "", "", "", fmt.Errorf(
			"en %q no se pudo leer el código de agente: la celda del título dice %q", nombre, titulo)
	}

	return operador, agente, mercadoDeEtiqueta(titulo), nil
}

// mercadoDeEtiqueta saca el mercado de una etiqueta "… Mercado de Comercialización
// [-] MERCADO".
//
// Los dos formatos de archivo traen el mercado en la misma frase pero con
// puntuación distinta, así que el guion es opcional:
//
//	uso de la red: "EEPD - EEP Mercado de Comercialización - CARTAGO"
//	ADD:           "EEP Mercado de Comercialización CARTAGO"
//
// Se ancla en la frase y no en el último guion porque hay mercados con guiones
// adentro: "CALI - YUMBO - PUERTO TEJADA" quedaría reducido a "PUERTO TEJADA". Y
// tampoco se puede partir por el primer guion contando posiciones: hay nombres con
// guion, como "AIR-E".
//
// Se busca "COMERCIALIZACI" —sin la vocal acentuada— porque el índice se usa para
// cortar el texto ORIGINAL. Buscar la frase completa en el texto normalizado daba
// un índice corrido: la "ó" ocupa dos bytes y la "o" que la reemplaza ocupa uno,
// así que el corte caía un byte antes y el mercado salía como "n - CARIBE MAR".
func mercadoDeEtiqueta(etiqueta string) string {
	const ancla = "COMERCIALIZACI"
	k := strings.Index(strings.ToUpper(etiqueta), ancla)
	if k < 0 {
		return ""
	}

	// Lo que sigue al ancla es el final de la palabra ("ón" u "on"), que nunca
	// tiene espacios: se salta hasta el primer espacio y de ahí sale el mercado.
	resto := etiqueta[k+len(ancla):]
	i := strings.IndexAny(resto, " \t")
	if i < 0 {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(resto[i:]), "-"))
}

func codigoDeMercado(nombre string) string {
	n := normalizar(nombre)
	i := strings.Index(n, "DEFINITIVO")
	if i < 0 {
		return ""
	}
	resto := n[i+len("DEFINITIVO"):]
	j := strings.Index(resto, "-")
	if j <= 0 {
		return ""
	}
	return resto[:j]
}

// sumarBloque ubica el marcador y suma su MISMA columna hacia abajo, hasta el
// marcador de fin.
func sumarBloque(filas [][]string, marcador, archivo string) (float64, error) {
	filaMarcador, colMarcador := -1, -1
	for i, fila := range filas {
		for c, celda := range fila {
			if normalizar(celda) == marcador {
				filaMarcador, colMarcador = i, c
				break
			}
		}
		if filaMarcador >= 0 {
			break
		}
	}
	if filaMarcador < 0 {
		return 0, fmt.Errorf(
			"en %q no se encontró el marcador %q, así que no se puede calcular ese componente",
			archivo, marcador)
	}

	fin := len(filas)
	for i := filaMarcador + 1; i < len(filas); i++ {
		n := normalizar(celda(filas, i, 1))
		for _, m := range marcadoresFin {
			if strings.Contains(n, m) {
				fin = i
				break
			}
		}
		if fin != len(filas) {
			break
		}
	}

	suma := 0.0
	for i := filaMarcador + 1; i < fin; i++ {
		if v, ok := numero(celda(filas, i, colMarcador)); ok {
			suma += v
		}
	}

	return suma, nil
}

// porEtiqueta busca la fila cuya columna de etiquetas contiene el texto y
// devuelve el valor de la columna indicada.
func porEtiqueta(filas [][]string, etiqueta string, col int) (float64, bool) {
	for i, fila := range filas {
		for _, c := range fila {
			if strings.Contains(normalizar(c), etiqueta) {
				if v, ok := numero(celda(filas, i, col)); ok {
					return v, true
				}
			}
		}
	}
	return 0, false
}

// porEtiquetaExacta busca una fila cuya celda sea exactamente la etiqueta y
// devuelve el primer número que aparezca a su derecha.
func porEtiquetaExacta(filas [][]string, etiqueta string) (float64, bool) {
	for i, fila := range filas {
		for c, celdaTexto := range fila {
			if normalizar(celdaTexto) != etiqueta {
				continue
			}
			for j := c + 1; j < len(fila); j++ {
				if v, ok := numero(celda(filas, i, j)); ok {
					return v, true
				}
			}
		}
	}
	return 0, false
}

// ── Utilidades ──────────────────────────────────────────────────────────────

func abrir(f UploadedFile) (*excelize.File, error) {
	libro, err := excelize.OpenReader(strings.NewReader(string(f.Content)))
	if err != nil {
		return nil, fmt.Errorf("el archivo %q no se pudo leer como Excel: %v", f.Name, err)
	}
	return libro, nil
}

func hojaPorPrefijo(libro *excelize.File, prefijo, archivo string) (string, error) {
	hojas := libro.GetSheetList()
	for _, h := range hojas {
		if strings.HasPrefix(normalizar(h), prefijo) {
			return h, nil
		}
	}
	// El archivo ADD trae diez hojas y varias tienen cargos distintos para el
	// mismo operador; leer otra daría un número plausible y equivocado.
	sort.Strings(hojas)
	return "", fmt.Errorf(
		"en %q no hay ninguna hoja que empiece con %q. Hojas del archivo: %s",
		archivo, prefijo, strings.Join(hojas, " | "))
}

func celda(filas [][]string, fila, col int) string {
	if fila < 0 || fila >= len(filas) {
		return ""
	}
	if col < 0 || col >= len(filas[fila]) {
		return ""
	}
	return filas[fila][col]
}

func contieneNormalizado(fila []string, texto string) bool {
	for _, c := range fila {
		if strings.Contains(normalizar(c), texto) {
			return true
		}
	}
	return false
}

func noVacias(fila []string) []string {
	out := []string{}
	for _, c := range fila {
		if strings.TrimSpace(c) != "" {
			out = append(out, strings.TrimSpace(c))
		}
	}
	return out
}

// numero interpreta una celda.
//
// Se intenta primero tal cual, porque los valores crudos de Excel vienen en
// notación de máquina — incluida la CIENTÍFICA. Limpiar antes rompía justamente
// esos: de "-5.0623e-05" se borraba la "e" y quedaba "-5.0623-05", que no parsea,
// así que el valor se descartaba en silencio y desaparecía de la suma del CDI o
// del CDN4. Con un CDI de 87 y un sumando de 0,00005, la tarifa salía creíble y
// distinta.
//
// Solo si no parsea se trata como texto con formato humano (símbolo de moneda,
// separador de miles). Ahí la coma es de miles y el punto es decimal, igual que
// en Cargos STR.
func numero(v string) (float64, bool) {
	s := strings.TrimSpace(v)
	if s == "" {
		return 0, false
	}

	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return n, true
	}

	limpio := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' || r == '-' {
			return r
		}
		return -1
	}, s)
	limpio = strings.ReplaceAll(limpio, ",", "")
	if limpio == "" || limpio == "-" {
		return 0, false
	}

	n, err := strconv.ParseFloat(limpio, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

var acentos = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U", "Ñ", "N",
)

func normalizar(s string) string {
	return strings.ToUpper(strings.TrimSpace(acentos.Replace(s)))
}
