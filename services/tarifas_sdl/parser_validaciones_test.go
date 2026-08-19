package tarifas_sdl_test

import (
	"fmt"
	"strings"
	"testing"

	"bia-bills/services/tarifas_sdl"

	"github.com/xuri/excelize/v2"
)

// Tests de las validaciones del parser, con archivos sintéticos.
//
// ⚠️ TRASVASE: archivo del paquete tarifas_sdl. Ver docs/backend/migracion-a-go.md.
//
// ── Por qué sintéticos y no los reales ───────────────────────────────────────
// Los 33 archivos de XM son válidos, así que con ellos una validación puede estar
// desactivada y todos los tests siguen pasando. Se comprobó por mutación: al
// quitar los controles de PR, de monotonía y de uniformidad, la suite no se
// enteraba.
//
// Acá se arma un lote VÁLIDO sintético y cada test rompe UNA cosa, para probar que
// el error se detecta. No se validan montos —eso es trabajo del test dorado contra
// los archivos reales— sino que lo que tiene que fallar, falle.

// ── Constructores ───────────────────────────────────────────────────────────

// archivoAdd arma un archivo ADD del área con el mismo cargo para todos sus
// operadores, que es la forma del dato real.
func archivoAdd(t *testing.T, area string, nivel int, valor float64) tarifas_sdl.UploadedFile {
	t.Helper()

	valores := make([]float64, len(mercadosPorArea[area]))
	for i := range valores {
		valores[i] = valor
	}

	return archivoAddNoUniforme(t, area, nivel, valores)
}

// archivoAddNoUniforme arma un archivo ADD con un cargo distinto por fila, para
// probar la validación de uniformidad. Los archivos reales nunca son así.
//
// Réplica de la estructura real: hoja "Cargos ADD <área> <nivel>", una fila de
// encabezado con "Operador de Red - Mercado" y "Cargo Único Transitorio", y las
// filas de datos debajo, cada una con su operador y su mercado.
func archivoAddNoUniforme(
	t *testing.T, area string, nivel int, valores []float64,
) tarifas_sdl.UploadedFile {
	t.Helper()

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	hoja := fmt.Sprintf("Cargos ADD %s %d", area, nivel)
	indice, err := f.NewSheet(hoja)
	if err != nil {
		t.Fatalf("NewSheet: %v", err)
	}
	f.SetActiveSheet(indice)

	poner := func(celda string, valor any) {
		if err := f.SetCellValue(hoja, celda, valor); err != nil {
			t.Fatalf("SetCellValue(%s): %v", celda, err)
		}
	}

	poner("B1", "Cargos ADD - Cargos Definitivos")
	poner("B2", fmt.Sprintf("ADD %s Nivel %d", area, nivel))
	poner("B4", "Operador de Red - Mercado")
	poner("C4", "Cargo Único Transitorio ($/kWh)")
	// La etiqueta lleva el mercado, que es de donde el parser saca a qué área
	// pertenece cada operador. Sin mercado no habría pertenencia y ningún operador
	// tendría cargos ADD.
	mercados := mercadosPorArea[area]
	for i, v := range valores {
		mercado := fmt.Sprintf("MERCADO AJENO %d", i+1)
		if i < len(mercados) {
			mercado = mercados[i]
		}
		poner(fmt.Sprintf("B%d", 5+i), fmt.Sprintf("OPERADOR %d Mercado de Comercialización %s", i+1, mercado))
		poner(fmt.Sprintf("C%d", 5+i), v)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}

	return tarifas_sdl.UploadedFile{
		Name:    fmt.Sprintf("LiquidacionDefinitivos%sNivel%d_202601.xlsx", area, nivel),
		Content: buf.Bytes(),
	}
}

// opcionesUso permite romper una cosa a la vez del archivo de uso de la red.
type opcionesUso struct {
	pr1        float64
	sinCDI     bool
	sinNivel4  bool
	sinPR2     bool
	sinNivel3  bool
	sinMonomio bool
}

// archivoUso arma un archivo de uso de la red completo.
func archivoUso(t *testing.T, codMercado, agente, mercado string, op opcionesUso) tarifas_sdl.UploadedFile {
	t.Helper()

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const hoja = "Cargos_Definitivos"
	indice, err := f.NewSheet(hoja)
	if err != nil {
		t.Fatalf("NewSheet: %v", err)
	}
	f.SetActiveSheet(indice)

	poner := func(celda string, valor any) {
		if err := f.SetCellValue(hoja, celda, valor); err != nil {
			t.Fatalf("SetCellValue(%s): %v", celda, err)
		}
	}

	poner("B1", fmt.Sprintf("%s - NOMBRE Mercado de Comercialización - %s", agente, mercado))

	// Encabezado de los cargos por nivel.
	if !op.sinMonomio {
		poner("F6", "Cargo monomio")
	}
	poner("C6", "Carga máxima")

	niveles := []int{1, 2, 3, 4}
	valores := []float64{300, 200, 100, 30}
	for i, n := range niveles {
		if n == 3 && op.sinNivel3 {
			continue
		}
		fila := 7 + i
		poner(fmt.Sprintf("B%d", fila), fmt.Sprintf("Cargo para cobrar el uso de la red Nivel %d (COP/kWh)", n))
		poner(fmt.Sprintf("F%d", fila), valores[i])
	}

	// Bloque de CDI y de nivel 4, con su marcador de fin.
	if !op.sinCDI {
		poner("C32", "CDI")
		poner("C33", 40.0)
		poner("C34", -0.00005) // notación científica al escribirse: el caso que se perdía
	}
	if !op.sinNivel4 {
		poner("G32", "Nivel4")
		poner("G33", 25.0)
	}
	poner("B36", "Cargo para el cobro de la remuneración de los planes de gestión")

	// Pérdidas reconocidas, como fracciones.
	pr1 := op.pr1
	if pr1 == 0 {
		pr1 = 0.12
	}
	poner("B48", "PR1")
	poner("C48", pr1)
	if !op.sinPR2 {
		poner("B49", "PR2")
		poner("C49", 0.04)
	}
	poner("B50", "PR3")
	poner("C50", 0.02)

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}

	return tarifas_sdl.UploadedFile{
		Name:    fmt.Sprintf("Cargo_Cobro_Uso_Red-Definitivo%s-202601.xlsx", codMercado),
		Content: buf.Bytes(),
	}
}

// Los 21 mercados con su agente, para armar el lote completo.
var mercadosDePrueba = []struct{ cod, agente, mercado string }{
	{"MARM", "CMMD", "CARIBE MAR"}, {"SOLM", "CSID", "CARIBE SOL"},
	{"NARM", "CDND", "NARIÑO"}, {"TOLM", "EPSD", "TOLIMA"},
	{"VACM", "EPSD", "VALLE DEL CAUCA"}, {"NSAM", "CNSD", "NORTE DE SANTANDER"},
	{"CAUM", "CEOD", "CAUCA"}, {"ULQM", "CETD", "TULUA"},
	{"CALM", "CHCD", "CALDAS"}, {"BOYM", "EBSD", "BOYACA"},
	{"QUIM", "EDQD", "QUINDIO"}, {"CRCM", "EEPD", "CARTAGO"},
	{"PEIM", "EEPD", "PEREIRA"}, {"HUIM", "HLAD", "HUILA"},
	{"CLOM", "EMID", "CALI - YUMBO"}, {"METM", "EMSD", "META"},
	{"CUNM", "ENDD", "BOGOTA"}, {"CASM", "CASD", "CASANARE"},
	{"ANTM", "EPMD", "ANTIOQUIA"}, {"SANM", "ESSD", "SANTANDER"},
	{"RUIM", "RTQD", "RUITOQUE"},
}

var areasDePrueba = []string{"Centro", "Occidente", "Oriente", "Sur"}

// mercadosPorArea reparte los mercados de prueba en las cuatro áreas, como lo hacen
// las hojas "Cargos ADD" reales: cada una lista los mercados de su área y de ahí
// sale la pertenencia de cada operador.
//
// CARIBE MAR y CARIBE SOL quedan afuera a propósito, igual que en los archivos de
// XM: no hay archivo de un área Caribe, así que esos dos operadores no tienen área.
var mercadosPorArea = map[string][]string{
	"Centro": {
		"NORTE DE SANTANDER", "CALDAS", "QUINDIO", "PEREIRA",
		"ANTIOQUIA", "SANTANDER", "RUITOQUE",
	},
	"Occidente": {"NARIÑO", "VALLE DEL CAUCA", "CAUCA", "TULUA", "CARTAGO", "CALI - YUMBO"},
	"Oriente":   {"TOLIMA", "BOYACA", "HUILA", "BOGOTA"},
	"Sur":       {"META", "CASANARE"},
}

// loteValido arma los 33 archivos de un lote que tiene que parsear sin errores.
func loteValido(t *testing.T) []tarifas_sdl.UploadedFile {
	t.Helper()

	archivos := []tarifas_sdl.UploadedFile{}
	for _, area := range areasDePrueba {
		// Decrecientes por nivel, como en los archivos reales, y el mismo valor
		// para todos los operadores del área.
		for nivel, valor := range map[int]float64{1: 300, 2: 200, 3: 100} {
			archivos = append(archivos, archivoAdd(t, area, nivel, valor))
		}
	}
	for _, m := range mercadosDePrueba {
		archivos = append(archivos, archivoUso(t, m.cod, m.agente, m.mercado, opcionesUso{}))
	}

	return archivos
}

// sinArchivo devuelve el lote sin el archivo cuyo nombre contiene el fragmento.
func sinArchivo(archivos []tarifas_sdl.UploadedFile, fragmento string) []tarifas_sdl.UploadedFile {
	out := []tarifas_sdl.UploadedFile{}
	for _, a := range archivos {
		if !strings.Contains(a.Name, fragmento) {
			out = append(out, a)
		}
	}
	return out
}

// reemplazar cambia el archivo cuyo nombre contiene el fragmento.
func reemplazar(archivos []tarifas_sdl.UploadedFile, fragmento string, nuevo tarifas_sdl.UploadedFile) []tarifas_sdl.UploadedFile {
	out := sinArchivo(archivos, fragmento)
	return append(out, nuevo)
}

// ── El lote válido parsea ───────────────────────────────────────────────────

// Si esto falla, los constructores no replican el formato y el resto de los tests
// no prueba nada.
func TestValidaciones_ElLoteSinteticoValidoParsea(t *testing.T) {
	res := tarifas_sdl.ParseInputs(loteValido(t))

	if len(res.CriticalErrors) > 0 {
		t.Fatalf("el lote sintético válido debería parsear: %v", res.CriticalErrors)
	}
	if len(res.Rows) != 21 {
		t.Fatalf("devolvió %d filas, se esperaban 21", len(res.Rows))
	}
}

// ── Las tres que la mutación descubrió sin cubrir ───────────────────────────

// Un PR como porcentaje haría que la fórmula divida por un número negativo y
// todas las tarifas activas salgan absurdas. Con la validación apagada, nada
// falla.
func TestValidaciones_PRComoPorcentajeCorta(t *testing.T) {
	roto := archivoUso(t, "CALM", "CHCD", "CALDAS", opcionesUso{pr1: 13.8011536})
	res := tarifas_sdl.ParseInputs(reemplazar(loteValido(t), "CALM", roto))

	if len(res.CriticalErrors) == 0 {
		t.Fatal("un PR de 13.8 tiene que cortar la carga")
	}
	juntos := strings.Join(res.CriticalErrors, " ")
	if !strings.Contains(juntos, "PR1") || !strings.Contains(juntos, "fracción") {
		t.Errorf("el error no explica el problema: %v", res.CriticalErrors)
	}
}

// Menor nivel de tensión, mayor cargo. Si se rompe, se leyó otra columna u otra
// hoja — o los archivos son de períodos distintos.
func TestValidaciones_MonotoniaDeLosCargosADD(t *testing.T) {
	// Nivel 2 con un cargo MAYOR que el de nivel 1.
	roto := archivoAdd(t, "Centro", 2, 999)
	res := tarifas_sdl.ParseInputs(reemplazar(loteValido(t), "CentroNivel2", roto))

	if len(res.CriticalErrors) == 0 {
		t.Fatal("los cargos que no decrecen por nivel tienen que cortar la carga")
	}
	if !strings.Contains(strings.Join(res.CriticalErrors, " "), "decrecen") {
		t.Errorf("el error no explica el problema: %v", res.CriticalErrors)
	}
}

// El cargo ADD es del ÁREA, así que es el mismo para todos sus operadores. Es lo
// que distingue la hoja correcta: en "Cargos Dt" y "Cargos Transitorios" del mismo
// libro el valor varía por operador.
func TestValidaciones_UniformidadDelCargoADD(t *testing.T) {
	roto := archivoAddNoUniforme(t, "Centro", 1, []float64{300, 310, 320})
	res := tarifas_sdl.ParseInputs(reemplazar(loteValido(t), "CentroNivel1", roto))

	if len(res.CriticalErrors) == 0 {
		t.Fatal("valores distintos por operador tienen que cortar la carga")
	}
	juntos := strings.Join(res.CriticalErrors, " ")
	if !strings.Contains(juntos, "valores distintos") {
		t.Errorf("el error no explica el problema: %v", res.CriticalErrors)
	}
}

// ── Lo que no se encuentra corta, y no queda en cero ────────────────────────

// Cada uno de estos, antes, dejaba el valor en 0. Y cero es plausible: sin CDI no
// hay descuento, sin CDN4 la activa iguala a la reactiva, sin PR el divisor es 1.
// Tarifas creíbles y equivocadas.
func TestValidaciones_LoQueFaltaCortaEnVezDeQuedarEnCero(t *testing.T) {
	casos := map[string]struct {
		op      opcionesUso
		mensaje string
	}{
		"sin el marcador CDI":        {opcionesUso{sinCDI: true}, "CDI"},
		"sin el marcador de nivel 4": {opcionesUso{sinNivel4: true}, "NIVEL4"},
		"sin la fila de PR2":         {opcionesUso{sinPR2: true}, "PR2"},
		"sin el cargo de nivel 3":    {opcionesUso{sinNivel3: true}, "nivel 3"},
		"sin la columna del cargo":   {opcionesUso{sinMonomio: true}, "CARGO MONOMIO"},
	}

	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			roto := archivoUso(t, "CALM", "CHCD", "CALDAS", caso.op)
			res := tarifas_sdl.ParseInputs(reemplazar(loteValido(t), "CALM", roto))

			if len(res.CriticalErrors) == 0 {
				t.Fatal("tiene que cortar la carga en vez de dejar el valor en cero")
			}
			if !strings.Contains(strings.Join(res.CriticalErrors, " "), caso.mensaje) {
				t.Errorf("el error no menciona %q: %v", caso.mensaje, res.CriticalErrors)
			}
		})
	}
}

// ── Cobertura del lote ──────────────────────────────────────────────────────

func TestValidaciones_FaltarUnArchivoCorta(t *testing.T) {
	casos := map[string]struct {
		fragmento string
		mensaje   string
	}{
		// Sin el ADD de un área, sus operadores no tienen NT.
		"falta un ADD": {"CentroNivel2", "Falta el archivo ADD"},
		// Sin el archivo de un operador, ese operador se quedaría con la tarifa del
		// período anterior por el modelo append-only, sin que nadie lo note.
		"falta un operador": {"CALM", "Faltan los archivos"},
	}

	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			res := tarifas_sdl.ParseInputs(sinArchivo(loteValido(t), caso.fragmento))

			if len(res.CriticalErrors) == 0 {
				t.Fatal("tiene que cortar la carga")
			}
			if !strings.Contains(strings.Join(res.CriticalErrors, " "), caso.mensaje) {
				t.Errorf("el error no menciona %q: %v", caso.mensaje, res.CriticalErrors)
			}
		})
	}
}

// Dos archivos del mismo área y nivel: antes el segundo sobrescribía al primero en
// silencio, y basta con que quede un archivo viejo en la carpeta.
func TestValidaciones_ArchivoAddDuplicadoCorta(t *testing.T) {
	archivos := loteValido(t)
	duplicado := archivoAdd(t, "Centro", 1, 300)
	duplicado.Name = "LiquidacionDefinitivosCentroNivel1_202512.xlsx" // otro período
	archivos = append(archivos, duplicado)

	res := tarifas_sdl.ParseInputs(archivos)

	if len(res.CriticalErrors) == 0 {
		t.Fatal("dos archivos del mismo área y nivel tienen que cortar la carga")
	}
	if !strings.Contains(strings.Join(res.CriticalErrors, " "), "dos archivos ADD") {
		t.Errorf("el error no explica el problema: %v", res.CriticalErrors)
	}
}

// Un mercado que no está en la tabla: antes se deducía del título y podía dar un
// operador inventado que después quedaba descartado sin explicación.
func TestValidaciones_MercadoDesconocidoCorta(t *testing.T) {
	desconocido := archivoUso(t, "XXXM", "ZZZZ", "MERCADO NUEVO", opcionesUso{})
	res := tarifas_sdl.ParseInputs(append(loteValido(t), desconocido))

	if len(res.CriticalErrors) == 0 {
		t.Fatal("un mercado fuera de la tabla tiene que cortar la carga")
	}
	juntos := strings.Join(res.CriticalErrors, " ")
	if !strings.Contains(juntos, "XXXM") || !strings.Contains(juntos, "no está en la tabla de mercados") {
		t.Errorf("el error no identifica el mercado: %v", res.CriticalErrors)
	}
}

// ── La hoja y la columna, por nombre ────────────────────────────────────────

// El archivo ADD real tiene DIEZ hojas y dos de ellas traen cargos distintos para
// el mismo operador. Si la hoja no está, hay que cortar y no leer otra.
func TestValidaciones_HojaAddConOtroNombreCorta(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	// Una hoja con datos plausibles pero con OTRO nombre.
	indice, _ := f.NewSheet("Cargos Dt Centro 1")
	f.SetActiveSheet(indice)
	_ = f.SetCellValue("Cargos Dt Centro 1", "B4", "Operador de Red - Mercado")
	_ = f.SetCellValue("Cargos Dt Centro 1", "C4", "Cargo Único Transitorio ($/kWh)")
	_ = f.SetCellValue("Cargos Dt Centro 1", "C5", 288.75)

	buf, _ := f.WriteToBuffer()
	roto := tarifas_sdl.UploadedFile{
		Name:    "LiquidacionDefinitivosCentroNivel1_202601.xlsx",
		Content: buf.Bytes(),
	}

	res := tarifas_sdl.ParseInputs(reemplazar(loteValido(t), "CentroNivel1", roto))

	if len(res.CriticalErrors) == 0 {
		t.Fatal("sin la hoja Cargos ADD tiene que cortar, no leer otra hoja")
	}
	juntos := strings.Join(res.CriticalErrors, " ")
	if !strings.Contains(juntos, "CARGOS ADD") {
		t.Errorf("el error no dice qué hoja falta: %v", res.CriticalErrors)
	}
	// Y tiene que listar las hojas que sí hay, para poder diagnosticar.
	if !strings.Contains(juntos, "Cargos Dt Centro 1") {
		t.Errorf("el error no lista las hojas encontradas: %v", res.CriticalErrors)
	}
}

func TestValidaciones_ColumnaAddRenombradaCorta(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	const hoja = "Cargos ADD Centro 1"
	indice, _ := f.NewSheet(hoja)
	f.SetActiveSheet(indice)
	_ = f.SetCellValue(hoja, "B4", "Operador de Red - Mercado")
	_ = f.SetCellValue(hoja, "C4", "Cargo Definitivo") // el nombre cambió
	_ = f.SetCellValue(hoja, "C5", 300.0)

	buf, _ := f.WriteToBuffer()
	roto := tarifas_sdl.UploadedFile{
		Name:    "LiquidacionDefinitivosCentroNivel1_202601.xlsx",
		Content: buf.Bytes(),
	}

	res := tarifas_sdl.ParseInputs(reemplazar(loteValido(t), "CentroNivel1", roto))

	if len(res.CriticalErrors) == 0 {
		t.Fatal("con la columna renombrada hay que cortar, no tomar el primer número")
	}
	juntos := strings.Join(res.CriticalErrors, " ")
	if !strings.Contains(juntos, "Cargo Definitivo") {
		t.Errorf("el error no lista los encabezados encontrados: %v", res.CriticalErrors)
	}
}

// Un archivo que no es Excel avisa y no tumba el request.
func TestValidaciones_ArchivoIlegible(t *testing.T) {
	roto := tarifas_sdl.UploadedFile{
		Name:    "LiquidacionDefinitivosCentroNivel1_202601.xlsx",
		Content: []byte("esto no es un xlsx"),
	}

	res := tarifas_sdl.ParseInputs(reemplazar(loteValido(t), "CentroNivel1", roto))

	if len(res.CriticalErrors) == 0 {
		t.Fatal("un archivo ilegible tiene que reportarse")
	}
	if !strings.Contains(strings.Join(res.CriticalErrors, " "), "no se pudo leer como Excel") {
		t.Errorf("el error no explica que el archivo no se pudo abrir: %v", res.CriticalErrors)
	}
}

// Un archivo que no es ni ADD ni de uso de la red se omite con aviso: puede ser
// algo que quedó en la carpeta y no debe tumbar la carga.
func TestValidaciones_ArchivoAjenoSoloAvisa(t *testing.T) {
	ajeno := tarifas_sdl.UploadedFile{Name: "notas_del_mes.xlsx", Content: []byte("x")}
	res := tarifas_sdl.ParseInputs(append(loteValido(t), ajeno))

	if len(res.CriticalErrors) > 0 {
		t.Fatalf("un archivo ajeno no debería cortar la carga: %v", res.CriticalErrors)
	}
	if len(res.Warnings) == 0 || !strings.Contains(strings.Join(res.Warnings, " "), "notas_del_mes") {
		t.Errorf("debería avisar que lo omitió: %v", res.Warnings)
	}
	if len(res.Rows) != 21 {
		t.Errorf("devolvió %d filas", len(res.Rows))
	}
}
