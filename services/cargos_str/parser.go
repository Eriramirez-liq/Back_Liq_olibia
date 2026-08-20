package cargos_str

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"bia-bills/entities"

	"github.com/xuri/excelize/v2"
)

// Extracción de los archivos BalanceSTR*.xlsx.
//
// Port del parser TypeScript (lib/parsers/insumos-str.ts), validado contra
// archivos reales. La lógica es deliberadamente idéntica, incluida la detección
// automática de la fila de encabezados y la búsqueda flexible de la fila
// BIAC-BIAE: son las que hacen que el parser tolere las variaciones de formato
// con las que XM publica estos archivos.
//
// Los montos son float64, igual que en la versión TypeScript. Los valores son
// enteros de ~1e9, muy dentro del rango exacto de float64 (2^53), así que la
// aritmética reproduce los mismos números peso por peso. En base se guardan como
// NUMERIC(18,2).

// UploadedFile es un archivo del lote, tal como llega en el multipart.
type UploadedFile struct {
	Name    string
	Content []byte
}

// StrRow es una fila del resultado: UN operador de red, no un archivo.
//
// Misma forma que la tabla liquidations_str_inputs, así lo que el usuario valida
// en pantalla es exactamente lo que queda guardado.
type StrRow struct {
	OperatorCode  string  `json:"operator_code"`
	OperatorName  string  `json:"operator_name,omitempty"` // se resuelve contra public.agents
	Period        string  `json:"period"`
	InvoiceAmount float64 `json:"invoice_amount"`
	// nil = ese archivo de ajuste no vino en el lote.
	// 0   = vino y el operador tenía cero. Son cosas distintas.
	Reinvoice1Amount *float64 `json:"reinvoice_1_amount"`
	Reinvoice2Amount *float64 `json:"reinvoice_2_amount"`
	Reinvoice3Amount *float64 `json:"reinvoice_3_amount"`
	AmountPayable    float64  `json:"amount_payable"`
}

// ParseResult acompaña las filas con lo que hay que contarle al usuario.
type ParseResult struct {
	Rows           []StrRow `json:"rows"`
	Warnings       []string `json:"warnings"`
	CriticalErrors []string `json:"critical_errors"`
}

// homologation traduce el código de columna del Excel al operador de red.
//
// Las columnas traen códigos de agente de XM. Los mismos códigos existen en
// public.agents (base file-compiler) con su nombre legal, y de ahí sale
// OperatorName. Lo que este mapa agrega, y agents no puede saber, es CUÁLES
// agentes son operadores del negocio y CÓMO SE AGRUPAN: AIRE llega en dos
// columnas (CSID y CSSD) que se suman, y esos dos agentes ni siquiera comparten
// NIT en el catálogo. Por eso vive en código.
var homologation = map[string]string{
	"CMMD": "AFINIA",
	"CSID": "AIRE", // mismo OR que CSSD: se suman
	"CSSD": "AIRE",
	"ENID": "ENELAR",
	"CHCD": "CHEC",
	"CDND": "CEDENAR",
	"CNSD": "CENS",
	"ESSD": "ESSA",
	"CQTD": "ELECTROCAQUETA",
	"HLAD": "ELECTROHUILA",
	"EMSD": "EMSA",
	"EBSD": "EBSA",
	"CASD": "ENERCA",
	"EEPD": "EEP_PEREIRA",
	"EBPD": "BAJO_PUTUMAYO",
	"EPSD": "CELSIA_VALLE",
	"EPTD": "PUTUMAYO",
	"EDQD": "EDEQ",
	"EGVD": "ENERGUAVIARE",
	"EDPD": "DISPAC",
	"EPMD": "EPM",
	"EMID": "EMCALI",
	"CEOD": "CEO",
	"ENDD": "ENEL",
}

// codigosOrdenados es el orden en que se buscan los códigos en un encabezado.
//
// Tiene que ser DETERMINÍSTICO: el orden de iteración de un mapa en Go es
// aleatorio, así que recorrer `homologation` directamente hacía que, si una celda
// de encabezado contuviera dos códigos, cuál gana cambiara entre ejecuciones — y
// el monto quedaría atribuido a un operador distinto cada vez.
//
// No es hipotético: estos archivos traen celdas descriptivas con varios códigos
// juntos ("Reporte ... CMMD CSID CSSD"), y de hecho detectHeaderRow existe para
// no elegir una de esas como encabezado. Pero si alguna se colara, el resultado
// tiene que ser reproducible.
var codigosOrdenados = func() []string {
	codes := make([]string, 0, len(homologation))
	for code := range homologation {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}()

// Homologation expone el mapa para resolver nombres contra public.agents.
func Homologation() map[string]string {
	copia := make(map[string]string, len(homologation))
	for k, v := range homologation {
		copia[k] = v
	}
	return copia
}

// monthsInFilename: las claves largas van primero para que "enero" no quede
// capturado por "ene". Ya NO define el período de la carga —ese sale del filtro—
// pero sí ordena los archivos de ajuste del más viejo al más nuevo.
var monthsInFilename = []struct {
	key   string
	month int
}{
	{"enero", 1}, {"ene", 1},
	{"febrero", 2}, {"feb", 2},
	{"marzo", 3}, {"mar", 3},
	{"abril", 4}, {"abr", 4},
	{"mayo", 5}, {"may", 5},
	{"junio", 6}, {"jun", 6},
	{"julio", 7}, {"jul", 7},
	{"agosto", 8}, {"ago", 8},
	{"septiembre", 9}, {"sep", 9},
	{"octubre", 10}, {"oct", 10},
	{"noviembre", 11}, {"nov", 11},
	{"diciembre", 12}, {"dic", 12},
}

var (
	yearInFilename = regexp.MustCompile(`(20\d{2})`)
	numericJunk    = regexp.MustCompile(`[^0-9.,\-]`)
	multiSpace     = regexp.MustCompile(`\s+`)
	dashNormalizer = strings.NewReplacer("–", "-", "—", "-")
)

// adjustmentOrder devuelve la clave de orden de un archivo de ajuste según su
// nombre: "BalanceSTRTipoReFactu2025-NOV-2.xlsx" → 202511.
//
// Si el nombre no trae mes o año reconocible devuelve el máximo, para que quede
// al final; el desempate final es alfabético, así el orden es reproducible.
// periodoDelNombre saca el año y el mes del nombre de un archivo BalanceSTR, que
// los trae como "BalanceSTRTipoFactu2026-MAY.xlsx".
//
// ok=false cuando el nombre no dice mes o no dice año. No es un error por sí
// solo: quien llama decide qué hacer con eso.
func periodoDelNombre(filename string) (anio, mes int, ok bool) {
	lower := strings.ToLower(filename)

	for _, m := range monthsInFilename {
		if strings.Contains(lower, "-"+m.key) || strings.Contains(lower, "_"+m.key) {
			mes = m.month
			break
		}
	}
	if mes == 0 {
		return 0, 0, false
	}

	year := yearInFilename.FindStringSubmatch(lower)
	if year == nil {
		return 0, 0, false
	}

	n, err := strconv.Atoi(year[1])
	if err != nil {
		return 0, 0, false
	}

	return n, mes, true
}

func adjustmentOrder(filename string) int {
	anio, mes, ok := periodoDelNombre(filename)
	if !ok {
		return 1 << 30
	}

	return anio*100 + mes
}

// toNum pasa el valor de una celda a número. Devuelve ok=false cuando la celda
// está vacía o no tiene nada numérico.
//
// Se intenta primero tal cual, porque con RawCellValue los valores vienen en
// notación de máquina — incluida la CIENTÍFICA. Limpiar antes rompía justamente
// esos: de "-5.0623e-05" se borraba la "e", quedaba "-5.0623-05", no parseaba, y
// el valor se descartaba EN SILENCIO como si la celda estuviera vacía.
//
// Acá eso significaría que un ajuste de refactura chico no se resta, y el valor a
// pagar sale creíble y equivocado. Se encontró portando Tarifas SDL, donde el
// mismo error se comía sumandos del CDI.
//
// Solo si no parsea se trata como texto con formato humano: símbolo de moneda,
// separador de miles.
func toNum(raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}

	if valor, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
		return valor, true
	}

	limpio := strings.TrimSpace(numericJunk.ReplaceAllString(raw, ""))
	if limpio == "" {
		return 0, false
	}

	// La coma es separador de miles en estos archivos, no decimal.
	valor, err := strconv.ParseFloat(strings.ReplaceAll(limpio, ",", ""), 64)
	if err != nil {
		return 0, false
	}

	return valor, true
}

// detectHeaderRow elige la fila con MÁS celdas individuales que contienen un
// código de operador.
//
// Evita el falso positivo donde una fila descriptiva con todos los códigos en una
// sola celda (ej. "Reporte ... CMMD CSID CSSD") ganaba sobre el encabezado real,
// donde cada código está en su propia celda.
func detectHeaderRow(matrix [][]string) int {
	best, bestScore := 6, 0 // 6 = el default del script original (header=6)

	limite := len(matrix)
	if limite > 20 {
		limite = 20
	}

	for i := 0; i < limite; i++ {
		conCodigo := 0
		for _, celda := range matrix[i] {
			texto := strings.ToUpper(celda)
			for _, code := range codigosOrdenados {
				if strings.Contains(texto, code) {
					conCodigo++
					break
				}
			}
		}
		if conCodigo > bestScore {
			best, bestScore = i, conCodigo
		}
	}

	return best
}

// findSheet busca una pestaña tolerando diferencias de espacios, guiones bajos y
// mayúsculas: "BalSTR01_Ajuste" ≡ "BalSTR01 Ajuste".
func findSheet(f *excelize.File, nombre string) string {
	normalizar := func(s string) string {
		return strings.ToUpper(strings.NewReplacer(" ", "", "_", "").Replace(s))
	}

	objetivo := normalizar(nombre)
	for _, hoja := range f.GetSheetList() {
		if normalizar(hoja) == objetivo {
			return hoja
		}
	}

	return ""
}

// findBiacRow ubica la fila "BIAC - BIAE", que es la que trae los valores.
//
// Búsqueda flexible a propósito: revisa las primeras cuatro columnas, normaliza
// espacios y cualquier tipo de guión, y solo exige que el texto contenga BIAC y
// BIAE. Los archivos de XM varían en todo eso.
func findBiacRow(matrix [][]string, desde int) int {
	for i := desde; i < len(matrix); i++ {
		fila := matrix[i]
		for col := 0; col <= 3 && col < len(fila); col++ {
			texto := strings.ToUpper(strings.TrimSpace(
				multiSpace.ReplaceAllString(dashNormalizer.Replace(fila[col]), " "),
			))
			if strings.Contains(texto, "BIAC") && strings.Contains(texto, "BIAE") {
				return i
			}
		}
	}

	return -1
}

// extractByOperator saca los valores de UN archivo, sumando las columnas que
// homologan al mismo operador (AIRE = CSID + CSSD).
func extractByOperator(file UploadedFile, sheets []string, warnings *[]string) map[string]float64 {
	valores := make(map[string]float64)

	f, err := excelize.OpenReader(bytes.NewReader(file.Content))
	if err != nil {
		*warnings = append(*warnings, fmt.Sprintf("[%s] no se pudo leer como Excel: %v", file.Name, err))
		return nil
	}
	defer func() { _ = f.Close() }()

	var diagnosticos []string

	for _, nombreHoja := range sheets {
		hoja := findSheet(f, nombreHoja)
		if hoja == "" {
			continue // la pestaña no existe: silencioso, igual que en TS
		}

		// RawCellValue evita que el formato de celda redondee o meta separadores:
		// necesitamos el valor almacenado, no el que se muestra.
		matrix, err := f.GetRows(hoja, excelize.Options{RawCellValue: true})
		if err != nil || len(matrix) == 0 {
			diagnosticos = append(diagnosticos, fmt.Sprintf("pestaña %q vacía o ilegible", nombreHoja))
			continue
		}

		filaHeader := detectHeaderRow(matrix)
		headers := matrix[filaHeader]

		filaBiac := findBiacRow(matrix, filaHeader+1)
		if filaBiac < 0 {
			diagnosticos = append(diagnosticos, fmt.Sprintf(
				"pestaña %q: BIAC-BIAE no encontrado (encabezado detectado en la fila %d)",
				nombreHoja, filaHeader+1))
			continue
		}
		biacRow := matrix[filaBiac]

		for j, header := range headers {
			if j >= len(biacRow) {
				break
			}
			headerUpper := strings.ToUpper(strings.TrimSpace(header))

			for _, code := range codigosOrdenados {
				if !strings.Contains(headerUpper, code) {
					continue
				}
				if valor, ok := toNum(biacRow[j]); ok {
					valores[homologation[code]] += valor
				}
				break
			}
		}
	}

	if len(valores) == 0 {
		detalle := ""
		if len(diagnosticos) > 0 {
			detalle = " — " + strings.Join(diagnosticos, " | ")
		}
		*warnings = append(*warnings, fmt.Sprintf(
			"[%s] no se encontró la fila \"BIAC - BIAE\" o no hubo valores para los operadores%s.",
			file.Name, detalle))
		return nil
	}

	return valores
}

// ParseStrInputs procesa el lote de archivos BalanceSTR y devuelve UNA FILA POR
// OPERADOR, con la factura y cada ajuste en su columna.
//
// El período sale de year/month —lo que el usuario eligió en el filtro de Nueva
// carga— y se aplica a TODOS los archivos, sin importar qué mes diga el nombre de
// un archivo de refactura.
func ParseStrInputs(files []UploadedFile, year, month int) ParseResult {
	resultado := ParseResult{
		Rows:           []StrRow{},
		Warnings:       []string{},
		CriticalErrors: []string{},
	}

	if len(files) == 0 {
		resultado.CriticalErrors = append(resultado.CriticalErrors, "No se recibió ningún archivo.")
		return resultado
	}

	period := fmt.Sprintf("%04d-%02d", year, month)
	resultado.Warnings = append(resultado.Warnings,
		fmt.Sprintf("Período de liquidación: %s (seleccionado en Nueva carga).", period))

	// ── Clasificar y ordenar ────────────────────────────────────────────────
	var invoices, adjustments []UploadedFile
	for _, f := range files {
		lower := strings.ToLower(f.Name)
		switch {
		case strings.Contains(lower, "tiporefactu"):
			adjustments = append(adjustments, f)
		case strings.Contains(lower, "tipofactu"):
			invoices = append(invoices, f)
		default:
			resultado.Warnings = append(resultado.Warnings, fmt.Sprintf(
				"[%s] omitido — el nombre no contiene \"tipofactu\" ni \"tiporefactu\".", f.Name))
		}
	}

	sort.SliceStable(adjustments, func(i, j int) bool {
		oi, oj := adjustmentOrder(adjustments[i].Name), adjustmentOrder(adjustments[j].Name)
		if oi != oj {
			return oi < oj
		}
		return adjustments[i].Name < adjustments[j].Name
	})

	// El ancho de la tabla destino es fijo. Un ajuste de más NO se descarta en
	// silencio: se corta la carga para que nadie liquide de menos sin enterarse.
	if len(adjustments) > entities.LiqStrMaxReinvoices {
		nombres := make([]string, 0, len(adjustments))
		for _, a := range adjustments {
			nombres = append(nombres, a.Name)
		}
		resultado.CriticalErrors = append(resultado.CriticalErrors, fmt.Sprintf(
			"El lote trae %d archivos de refactura y el máximo admitido es %d. Archivos: %s. "+
				"Cargalos en dos veces o pedí ampliar las columnas de ajuste.",
			len(adjustments), entities.LiqStrMaxReinvoices, strings.Join(nombres, ", ")))
		return resultado
	}

	if len(invoices) == 0 {
		resultado.Warnings = append(resultado.Warnings,
			"El lote no trae archivo de factura (tipofactu): solo se registrarán ajustes.")
	}

	// ── El archivo de factura tiene que ser del período elegido ─────────────
	//
	// El período de la carga lo fija la persona en Nueva carga. El archivo de
	// factura dice en su nombre de qué mes es —"BalanceSTRTipoFactu2026-MAY"— y si
	// no coinciden, se eligió mal el mes o se arrastró el archivo de otro. Los
	// cargues reales lo confirman: hay uno de 2026-05 hecho con el archivo de abril.
	//
	// CORTA la carga. Por el modelo append-only, un cargue con el archivo del mes
	// equivocado no se deshace desde la pantalla: quedan cifras de un mes guardadas
	// bajo otro, y el error solo aparece cuando alguien concilia. Es más barato
	// frenarlo acá.
	//
	// Si el nombre no dice el mes, en cambio, solo se avisa: no hay contra qué
	// comparar, y bloquear por eso dejaría a nadie cargar un archivo renombrado.
	//
	// Los archivos de refactura NO se validan: que sean de otros meses es
	// exactamente lo que son, ajustes de períodos anteriores.
	desajustes := []string{}
	for _, f := range invoices {
		anioArchivo, mesArchivo, ok := periodoDelNombre(f.Name)
		if !ok {
			resultado.Warnings = append(resultado.Warnings, fmt.Sprintf(
				"[%s] no dice en su nombre de qué mes es, así que no se pudo verificar contra el "+
					"período seleccionado (%s). Revisá que sea el archivo del mes correcto.",
				f.Name, period))
			continue
		}
		if anioArchivo != year || mesArchivo != month {
			desajustes = append(desajustes, fmt.Sprintf(
				"%q es de %04d-%02d", f.Name, anioArchivo, mesArchivo))
		}
	}
	if len(desajustes) > 0 {
		resultado.CriticalErrors = append(resultado.CriticalErrors, fmt.Sprintf(
			"El período seleccionado es %s y el archivo de factura no es de ese mes: %s. "+
				"Si el archivo es el que corresponde, corregí el período en Nueva carga; si no, "+
				"cargá el archivo de %s.",
			period, strings.Join(desajustes, ", "), period))
		return resultado
	}

	// ── Extraer ─────────────────────────────────────────────────────────────
	invoiceValues := make(map[string]float64)
	for _, f := range invoices {
		valores := extractByOperator(f, []string{"BalSTR01", "BalSTR02"}, &resultado.Warnings)
		for operador, valor := range valores {
			invoiceValues[operador] += valor
		}
	}

	adjustmentValues := make([]map[string]float64, 0, len(adjustments))
	for _, f := range adjustments {
		valores := extractByOperator(f, []string{"BalSTR01_Ajuste", "BalSTR02_Ajuste"}, &resultado.Warnings)
		if valores != nil {
			adjustmentValues = append(adjustmentValues, valores)
		}
	}

	// ── Pivotar a una fila por operador ─────────────────────────────────────
	operadores := make(map[string]struct{}, len(invoiceValues))
	for operador := range invoiceValues {
		operadores[operador] = struct{}{}
	}
	for _, valores := range adjustmentValues {
		for operador := range valores {
			operadores[operador] = struct{}{}
		}
	}

	ordenados := make([]string, 0, len(operadores))
	for operador := range operadores {
		ordenados = append(ordenados, operador)
	}
	sort.Strings(ordenados)

	for _, operador := range ordenados {
		fila := StrRow{
			OperatorCode:  operador,
			Period:        period,
			InvoiceAmount: invoiceValues[operador],
		}

		total := fila.InvoiceAmount
		destinos := []**float64{
			&fila.Reinvoice1Amount,
			&fila.Reinvoice2Amount,
			&fila.Reinvoice3Amount,
		}
		for i, valores := range adjustmentValues {
			if i >= len(destinos) {
				break
			}
			valor := valores[operador] // 0 si el archivo vino y el operador no estaba
			*destinos[i] = &valor
			total += valor
		}

		fila.AmountPayable = total
		resultado.Rows = append(resultado.Rows, fila)
	}

	if len(resultado.Rows) == 0 && len(resultado.CriticalErrors) == 0 {
		resultado.Warnings = append(resultado.Warnings,
			"No se generaron registros — revisá los archivos cargados.")
	}

	return resultado
}
