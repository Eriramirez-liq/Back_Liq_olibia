package tarifas_sdl_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bia-bills/models"
	"bia-bills/repositories"
	"bia-bills/services/tarifas_sdl"
)

// Tests del servicio con dobles a mano.
//
// Un "doble" es un objeto falso que reemplaza al repositorio real, para que los
// tests no necesiten base de datos: devuelven lo que el test decide y anotan con
// qué los llamaron.
//
// Lo que más interesa: el Confirm escribe en DOS bases sin transacción que las
// abarque, y la auditoría es lo que demuestra que las dos tablas están de verdad
// relacionadas.

// ── Dobles ──────────────────────────────────────────────────────────────────

type sdlRepoFake struct {
	insumosInsertados []models.LiquidationsSdlInput
	tarifasInsertadas []models.LiquidationsSdlRate
	borradoDeCarga    string

	insumosGuardados []models.LiquidationsSdlInput
	tarifasGuardadas []repositories.SdlRate
	periodos         []string
	cargues          []repositories.SdlLoad

	errAlInsertarInsumos error
	errAlInsertarTarifas error
	errAlBorrar          error
	errDeLectura         error

	periodosRecibidos []string
	filtroRecibido    repositories.SdlRateFilter
}

func (f *sdlRepoFake) InsertInputs(_ context.Context, rows []models.LiquidationsSdlInput) error {
	if f.errAlInsertarInsumos != nil {
		return f.errAlInsertarInsumos
	}
	f.insumosInsertados = append(f.insumosInsertados, rows...)
	return nil
}

func (f *sdlRepoFake) InsertRates(_ context.Context, rows []models.LiquidationsSdlRate) error {
	if f.errAlInsertarTarifas != nil {
		return f.errAlInsertarTarifas
	}
	f.tarifasInsertadas = append(f.tarifasInsertadas, rows...)
	return nil
}

func (f *sdlRepoFake) DeleteInputsByLoad(_ context.Context, loadID string) error {
	f.borradoDeCarga = loadID
	return f.errAlBorrar
}

func (f *sdlRepoFake) CurrentRates(_ context.Context, filtro repositories.SdlRateFilter) ([]repositories.SdlRate, error) {
	f.filtroRecibido = filtro
	return f.tarifasGuardadas, f.errDeLectura
}

func (f *sdlRepoFake) CurrentInputs(_ context.Context, periods []string) ([]models.LiquidationsSdlInput, error) {
	f.periodosRecibidos = periods
	return f.insumosGuardados, f.errDeLectura
}

func (f *sdlRepoFake) PeriodsWithRates(context.Context) ([]string, error) {
	return f.periodos, f.errDeLectura
}

func (f *sdlRepoFake) Loads(_ context.Context, periods []string) ([]repositories.SdlLoad, error) {
	f.periodosRecibidos = periods
	return f.cargues, f.errDeLectura
}

type agentesFake struct {
	nombres    map[string]string
	err        error
	consultado []string
}

func (f *agentesFake) NamesByOperator(context.Context, map[string]string) (map[string]string, error) {
	return nil, nil
}

func (f *agentesFake) NamesByAgentCode(_ context.Context, codes []string) (map[string]string, error) {
	f.consultado = codes
	return f.nombres, f.err
}

// catálogo con los nombres de los 19 códigos de agente que traen los archivos.
func agentesDePrueba() *agentesFake {
	return &agentesFake{nombres: map[string]string{
		"CMMD": "CARIBEMAR DE LA COSTA S.A.S. E.S.P.",
		"CSSD": "AIR- E S.A.S. E.S.P.",
		"CDND": "CENTRALES ELECTRICAS DE NARIÑO S.A. E.S.P.",
		"EPSD": "CELSIA COLOMBIA S.A. E.S.P.",
		"CNSD": "CENTRALES ELECTRICAS DEL NORTE DE SANTANDER S.A. E.S.P.",
		"CEOD": "COMPANIA ENERGETICA DE OCCIDENTE S.A.S. E.S.P.",
		"CETD": "COMPANIA DE ELECTRICIDAD DE TULUA S.A. E.S.P.",
		"CHCD": "CENTRAL HIDROELECTRICA DE CALDAS S.A. E.S.P.",
		"EBSD": "EMPRESA DE ENERGIA DE BOYACA S.A. E.S.P.",
		"EDQD": "EMPRESA DE ENERGIA DEL QUINDIO S.A. E.S.P.",
		"EEPD": "EMPRESA DE ENERGIA DE PEREIRA S.A. E.S.P.",
		"HLAD": "ELECTRIFICADORA DEL HUILA S.A. E.S.P.",
		"EMID": "EMPRESAS MUNICIPALES DE CALI E.I.C.E. E.S.P.",
		"EMSD": "ELECTRIFICADORA DEL META S.A. E.S.P.",
		"ENDD": "ENEL COLOMBIA SA ESP",
		"CASD": "EMPRESA DE ENERGIA DE CASANARE S.A. E.S.P.",
		"EPMD": "EMPRESAS PUBLICAS DE MEDELLIN E.S.P.",
		"ESSD": "ELECTRIFICADORA DE SANTANDER S.A. E.S.P.",
		"RTQD": "RUITOQUE S.A. E.S.P.",
	}}
}

func servicioDePrueba(repo *sdlRepoFake, agentes *agentesFake) tarifas_sdl.TarifasSdlService {
	return tarifas_sdl.NewTarifasSdlService(repo, agentes)
}

// ── Preview ─────────────────────────────────────────────────────────────────

func TestServicioSDL_PreviewCompletaNombresYCalcula(t *testing.T) {
	agentes := agentesDePrueba()
	servicio := servicioDePrueba(&sdlRepoFake{}, agentes)

	res, err := servicio.Preview(context.Background(), cargarArchivosReales(t))
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(res.CriticalErrors) > 0 {
		t.Fatalf("errores críticos: %v", res.CriticalErrors)
	}
	if len(res.Rows) != 21 {
		t.Fatalf("devolvió %d filas, se esperaban 21", len(res.Rows))
	}

	// El alias tiene que haberse aplicado ANTES de consultar: se pide CSSD, no CSID.
	pidioCSSD := false
	for _, c := range agentes.consultado {
		if c == "CSID" {
			t.Error("consultó el catálogo con CSID; hay que pedir CSSD, que es el operador de red vigente")
		}
		if c == "CSSD" {
			pidioCSSD = true
		}
	}
	if !pidioCSSD {
		t.Errorf("no consultó CSSD para resolver AIRE: %v", agentes.consultado)
	}

	porOperador := map[string]tarifas_sdl.PreviewRow{}
	for _, fila := range res.Rows {
		porOperador[fila.OperatorCode] = fila
		if fila.OperatorName == "" {
			t.Errorf("%s quedó sin nombre", fila.OperatorCode)
		}
	}

	// AIRE tiene que traer el nombre limpio y NO el del registro intervenido.
	if aire := porOperador["AIRE"].OperatorName; strings.Contains(aire, "INTERVENIDO") {
		t.Errorf("AIRE trae el nombre del registro intervenido: %q", aire)
	}

	// Los dos pares que comparten razón social: mismo nombre, mercado distinto.
	if porOperador["CELSIA_VALLE"].OperatorName != porOperador["CELSIA_TOLIMA"].OperatorName {
		t.Error("las dos Celsia deberían tener la misma razón social")
	}
	if porOperador["CELSIA_VALLE"].Market == porOperador["CELSIA_TOLIMA"].Market {
		t.Error("las dos Celsia deberían tener mercados distintos, o son indistinguibles")
	}

	// Y las tarifas calculadas coinciden con el motor.
	//
	// El preview las devuelve con los MISMOS nombres que las columnas de la tabla y
	// que el endpoint de lectura, para que el front tenga una sola forma. Antes
	// devolvía `rates.Activa.Nivel1Operador` de un lado y `active_level_1_operator`
	// del otro.
	cens := porOperador["CENS"]
	delMotor := tarifas_sdl.Calcular(mustComponentes(t, cens.SdlInputRow))

	if cens.Rates.ActiveLevel1Operator != delMotor.Activa.Nivel1Operador {
		t.Errorf("active_level_1_operator = %v, el motor da %v",
			cens.Rates.ActiveLevel1Operator, delMotor.Activa.Nivel1Operador)
	}
	if cens.Rates.ReactiveLevel3User != delMotor.Reactiva.Nivel3Usuario {
		t.Errorf("reactive_level_3_user = %v, el motor da %v",
			cens.Rates.ReactiveLevel3User, delMotor.Reactiva.Nivel3Usuario)
	}
}

// Si el catálogo no resuelve un agente, el preview NO se muestra a medias: sin
// nombre no se distinguen los operadores que comparten razón social, así que la
// pantalla sería ambigua.
func TestServicioSDL_PreviewCortaSiFaltaUnNombre(t *testing.T) {
	agentes := agentesDePrueba()
	delete(agentes.nombres, "CHCD") // CHEC queda sin nombre

	servicio := servicioDePrueba(&sdlRepoFake{}, agentes)

	res, err := servicio.Preview(context.Background(), cargarArchivosReales(t))
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(res.CriticalErrors) == 0 {
		t.Fatal("debería dar error crítico si un operador no resuelve nombre")
	}
	if !strings.Contains(res.CriticalErrors[0], "CHEC") {
		t.Errorf("el error no dice qué operador falta: %q", res.CriticalErrors[0])
	}
	if len(res.Rows) != 0 {
		t.Errorf("no debería devolver filas: %d", len(res.Rows))
	}
}

func TestServicioSDL_PreviewPropagaElErrorDelCatalogo(t *testing.T) {
	agentes := &agentesFake{err: errors.New("catálogo caído")}
	servicio := servicioDePrueba(&sdlRepoFake{}, agentes)

	res, err := servicio.Preview(context.Background(), cargarArchivosReales(t))
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(res.CriticalErrors) == 0 || !strings.Contains(res.CriticalErrors[0], "catálogo caído") {
		t.Errorf("no informó la causa: %v", res.CriticalErrors)
	}
}

// ── Confirm ─────────────────────────────────────────────────────────────────

func filasParaConfirmar(t *testing.T) []tarifas_sdl.PreviewRow {
	t.Helper()

	servicio := servicioDePrueba(&sdlRepoFake{}, agentesDePrueba())
	res, err := servicio.Preview(context.Background(), cargarArchivosReales(t))
	if err != nil || len(res.CriticalErrors) > 0 {
		t.Fatalf("no se pudo armar el preview: %v %v", err, res.CriticalErrors)
	}

	return res.Rows
}

func TestServicioSDL_ConfirmGuardaEnLasDosBases(t *testing.T) {
	repo := &sdlRepoFake{}
	servicio := servicioDePrueba(repo, agentesDePrueba())

	loadID, err := servicio.Confirm(context.Background(), "2026-01", filasParaConfirmar(t),
		tarifas_sdl.LoadMeta{CreatedBy: "Erika", CreatedByID: "uid-1"})
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if loadID == "" {
		t.Fatal("no devolvió load_id")
	}

	if len(repo.insumosInsertados) != 21 || len(repo.tarifasInsertadas) != 21 {
		t.Fatalf("insumos=%d tarifas=%d, se esperaban 21 y 21",
			len(repo.insumosInsertados), len(repo.tarifasInsertadas))
	}

	// El mismo load_id en las dos bases es lo que permite auditar.
	for i := range repo.insumosInsertados {
		if repo.insumosInsertados[i].LoadID != loadID || repo.tarifasInsertadas[i].LoadID != loadID {
			t.Fatal("el load_id no coincide entre las dos bases")
		}
		if repo.insumosInsertados[i].Period != "2026-01" {
			t.Errorf("el período no se aplicó: %q", repo.insumosInsertados[i].Period)
		}
	}

	// Los metadatos del cargue viajan solo con los insumos.
	if repo.insumosInsertados[0].CreatedBy != "Erika" || repo.insumosInsertados[0].CreatedByID != "uid-1" {
		t.Errorf("metadatos mal guardados: %+v", repo.insumosInsertados[0])
	}

	// El nombre y el mercado quedan en las DOS tablas: la de tarifas vive en otra
	// base y no puede hacer join contra el catálogo de agentes.
	for _, tarifa := range repo.tarifasInsertadas {
		if tarifa.OperatorName == "" || tarifa.AgentCode == "" {
			t.Errorf("la tarifa de %s quedó sin identidad: %+v", tarifa.OperatorCode, tarifa)
			break
		}
	}
}

// Las tarifas se recalculan en el servidor y NO se toman de lo que mandó el
// navegador. Es lo que garantiza que la tarifa guardada sea consecuencia de los
// componentes guardados — sin eso la auditoría no probaría nada.
func TestServicioSDL_ConfirmIgnoraLasTarifasQueMandaElCliente(t *testing.T) {
	repo := &sdlRepoFake{}
	servicio := servicioDePrueba(repo, agentesDePrueba())

	filas := filasParaConfirmar(t)
	// Un cliente malicioso —o un bug del front— manda una tarifa inventada.
	for i := range filas {
		filas[i].Rates.ActiveLevel1Operator = 999_999
	}

	if _, err := servicio.Confirm(context.Background(), "2026-01", filas, tarifas_sdl.LoadMeta{}); err != nil {
		t.Fatalf("Confirm: %v", err)
	}

	for _, tarifa := range repo.tarifasInsertadas {
		if tarifa.ActiveLevel1Operator == 999_999 {
			t.Fatalf("%s guardó la tarifa que mandó el cliente en vez de recalcularla", tarifa.OperatorCode)
		}
	}
}

func TestServicioSDL_ConfirmRollbackSiFallaLaSegundaBase(t *testing.T) {
	repo := &sdlRepoFake{errAlInsertarTarifas: errors.New("caída")}
	servicio := servicioDePrueba(repo, agentesDePrueba())

	_, err := servicio.Confirm(context.Background(), "2026-01", filasParaConfirmar(t), tarifas_sdl.LoadMeta{})
	if err == nil {
		t.Fatal("se esperaba error")
	}

	// Sin este borrado quedarían componentes sin tarifa: la pantalla mostraría de
	// menos y la auditoría reportaría huérfanos para siempre.
	if repo.borradoDeCarga == "" {
		t.Error("no hizo el rollback de los componentes")
	}
	if repo.insumosInsertados[0].LoadID != repo.borradoDeCarga {
		t.Error("borró una carga distinta de la que había insertado")
	}
}

func TestServicioSDL_ConfirmValidaciones(t *testing.T) {
	casos := []struct {
		nombre  string
		periodo string
		filas   func(*testing.T) []tarifas_sdl.PreviewRow
		mensaje string
	}{
		{
			nombre: "sin período", periodo: "  ", filas: filasParaConfirmar,
			mensaje: "falta el período",
		},
		{
			nombre: "sin filas", periodo: "2026-01",
			filas:   func(*testing.T) []tarifas_sdl.PreviewRow { return nil },
			mensaje: "no tiene filas",
		},
		{
			nombre: "lote incompleto", periodo: "2026-01",
			// Faltar operadores no es un error del cliente: por el modelo
			// append-only, los que falten se quedarían con la tarifa del mes
			// anterior sin que nadie lo note.
			filas: func(t *testing.T) []tarifas_sdl.PreviewRow {
				return filasParaConfirmar(t)[:5]
			},
			mensaje: "no cubre a todos los operadores",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			repo := &sdlRepoFake{}
			servicio := servicioDePrueba(repo, agentesDePrueba())

			_, err := servicio.Confirm(context.Background(), caso.periodo, caso.filas(t), tarifas_sdl.LoadMeta{})
			if err == nil {
				t.Fatal("se esperaba error")
			}
			if !strings.Contains(err.Error(), caso.mensaje) {
				t.Errorf("el error no menciona %q: %v", caso.mensaje, err)
			}
			if len(repo.insumosInsertados) != 0 {
				t.Error("no debería haber guardado nada")
			}
		})
	}
}

// ── Lecturas ────────────────────────────────────────────────────────────────

func TestServicioSDL_LecturasDeleganYPropagan(t *testing.T) {
	repo := &sdlRepoFake{
		tarifasGuardadas: []repositories.SdlRate{{Period: "2026-01", OperatorCode: "CENS"}},
		periodos:         []string{"2026-01"},
		cargues:          []repositories.SdlLoad{{LoadID: "c1", Operators: 21}},
	}
	servicio := servicioDePrueba(repo, agentesDePrueba())
	ctx := context.Background()

	if tarifas, err := servicio.CurrentRates(ctx, repositories.SdlRateFilter{Periods: []string{"2026-01"}}); err != nil {
		t.Errorf("CurrentRates: %v", err)
	} else if len(tarifas) != 1 {
		t.Errorf("CurrentRates devolvió %d", len(tarifas))
	}
	if len(repo.filtroRecibido.Periods) != 1 {
		t.Errorf("el filtro llegó alterado: %+v", repo.filtroRecibido)
	}

	if periodos, err := servicio.Periods(ctx); err != nil || len(periodos) != 1 {
		t.Errorf("Periods: %v %v", periodos, err)
	}
	if cargues, err := servicio.Loads(ctx, []string{"2026-01"}); err != nil || len(cargues) != 1 {
		t.Errorf("Loads: %v %v", cargues, err)
	}

	// Y el error se propaga con su causa.
	repoRoto := &sdlRepoFake{errDeLectura: errors.New("base caída")}
	servicioRoto := servicioDePrueba(repoRoto, agentesDePrueba())
	if _, err := servicioRoto.CurrentRates(ctx, repositories.SdlRateFilter{}); err == nil ||
		!strings.Contains(err.Error(), "base caída") {
		t.Errorf("CurrentRates no propagó la causa: %v", err)
	}
}

// ── Auditoría ───────────────────────────────────────────────────────────────

// insumoGuardado arma una fila como quedaría en la base.
func insumoGuardado(operador, agente, area string, dt1Add, dt2Add, dt3Add float64) models.LiquidationsSdlInput {
	fila := models.LiquidationsSdlInput{
		Period: "2026-01", OperatorCode: operador, AgentCode: agente,
		DT1: 100, DT2: 80, DT3: 60, CDI: 10, CDN4: 5,
		PR1: 0.1, PR2: 0.05, PR3: 0.02,
	}
	if area != "" {
		a := area
		fila.DistributionArea = &a
		fila.DT1Add, fila.DT2Add, fila.DT3Add = &dt1Add, &dt2Add, &dt3Add
	}
	return fila
}

// tarifaDe calcula lo que la base DEBERÍA tener para ese insumo.
func tarifaDe(t *testing.T, insumo models.LiquidationsSdlInput) repositories.SdlRate {
	t.Helper()

	fila := tarifas_sdl.SdlInputRow{
		OperatorCode: insumo.OperatorCode,
		DT1Add:       insumo.DT1Add, DT2Add: insumo.DT2Add, DT3Add: insumo.DT3Add,
		DT1: insumo.DT1, DT2: insumo.DT2, DT3: insumo.DT3,
		CDI: insumo.CDI, CDN4: insumo.CDN4,
		PR1: insumo.PR1, PR2: insumo.PR2, PR3: insumo.PR3,
	}
	if insumo.DistributionArea != nil {
		fila.DistributionArea = *insumo.DistributionArea
	}

	tarifas := tarifas_sdl.Calcular(mustComponentes(t, fila))

	return repositories.SdlRate{
		Period: insumo.Period, OperatorCode: insumo.OperatorCode,
		ActiveLevel1Operator:   tarifas.Activa.Nivel1Operador,
		ActiveLevel1Shared:     tarifas.Activa.Nivel1Compartido,
		ActiveLevel1User:       tarifas.Activa.Nivel1Usuario,
		ActiveLevel2User:       tarifas.Activa.Nivel2Usuario,
		ActiveLevel3User:       tarifas.Activa.Nivel3Usuario,
		ReactiveLevel1Operator: tarifas.Reactiva.Nivel1Operador,
		ReactiveLevel1Shared:   tarifas.Reactiva.Nivel1Compartido,
		ReactiveLevel1User:     tarifas.Reactiva.Nivel1Usuario,
		ReactiveLevel2User:     tarifas.Reactiva.Nivel2Usuario,
		ReactiveLevel3User:     tarifas.Reactiva.Nivel3Usuario,
	}
}

func TestAudit_CuandoTodoCuadraNoReportaNada(t *testing.T) {
	insumo := insumoGuardado("CENS", "CNSD", "CENTRO", 120, 90, 70)
	repo := &sdlRepoFake{
		insumosGuardados: []models.LiquidationsSdlInput{insumo},
		tarifasGuardadas: []repositories.SdlRate{tarifaDe(t, insumo)},
	}

	res, err := servicioDePrueba(repo, agentesDePrueba()).Audit(context.Background(), []string{"2026-01"})
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if res.Checked != 1 {
		t.Errorf("verificó %d filas, se esperaba 1", res.Checked)
	}
	if len(res.Findings) != 0 || len(res.Orphans) != 0 {
		t.Errorf("no debería reportar nada: %+v %v", res.Findings, res.Orphans)
	}
}

// La prueba de que la auditoría sirve: si una tarifa guardada NO es la que sale de
// sus componentes, hay que encontrarla. Sin este test, la auditoría podría estar
// devolviendo "todo bien" siempre.
func TestAudit_DetectaUnaTarifaQueNoSaleDeSusComponentes(t *testing.T) {
	insumo := insumoGuardado("CENS", "CNSD", "CENTRO", 120, 90, 70)
	tarifa := tarifaDe(t, insumo)
	tarifa.ActiveLevel1User += 0.5 // alguien la editó a mano, o se guardó mal

	repo := &sdlRepoFake{
		insumosGuardados: []models.LiquidationsSdlInput{insumo},
		tarifasGuardadas: []repositories.SdlRate{tarifa},
	}

	res, err := servicioDePrueba(repo, agentesDePrueba()).Audit(context.Background(), nil)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("debería reportar exactamente 1 hallazgo, reportó %d: %+v", len(res.Findings), res.Findings)
	}

	hallazgo := res.Findings[0]
	if hallazgo.Column != "active_level_1_user" {
		t.Errorf("señaló la columna %q", hallazgo.Column)
	}
	if hallazgo.OperatorCode != "CENS" || hallazgo.Period != "2026-01" {
		t.Errorf("no identifica la fila: %+v", hallazgo)
	}
	// Los dos valores tienen que viajar, o el hallazgo no sirve para investigar.
	if hallazgo.Stored == 0 || hallazgo.Recomputed == 0 {
		t.Errorf("el hallazgo no trae los dos valores: %+v", hallazgo)
	}
}

func TestAudit_ReportaHuerfanosEnLosDosSentidos(t *testing.T) {
	conTarifa := insumoGuardado("CENS", "CNSD", "CENTRO", 120, 90, 70)
	sinTarifa := insumoGuardado("CHEC", "CHCD", "CENTRO", 120, 90, 70)

	// Una tarifa cuyo componente no está.
	tarifaSuelta := tarifaDe(t, conTarifa)
	tarifaSuelta.OperatorCode = "EPM"

	repo := &sdlRepoFake{
		insumosGuardados: []models.LiquidationsSdlInput{conTarifa, sinTarifa},
		tarifasGuardadas: []repositories.SdlRate{tarifaDe(t, conTarifa), tarifaSuelta},
	}

	res, err := servicioDePrueba(repo, agentesDePrueba()).Audit(context.Background(), nil)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}

	juntos := strings.Join(res.Orphans, " | ")
	if !strings.Contains(juntos, "CHEC") || !strings.Contains(juntos, "componentes sin tarifa") {
		t.Errorf("no reportó el componente sin tarifa: %v", res.Orphans)
	}
	if !strings.Contains(juntos, "EPM") || !strings.Contains(juntos, "tarifa sin componentes") {
		t.Errorf("no reportó la tarifa sin componentes: %v", res.Orphans)
	}
}

func TestAudit_PropagaLosErroresDeLectura(t *testing.T) {
	repo := &sdlRepoFake{errDeLectura: errors.New("base caída")}

	if _, err := servicioDePrueba(repo, agentesDePrueba()).Audit(context.Background(), nil); err == nil ||
		!strings.Contains(err.Error(), "base caída") {
		t.Errorf("no propagó la causa: %v", err)
	}
}

func mustComponentes(t *testing.T, fila tarifas_sdl.SdlInputRow) tarifas_sdl.Componentes {
	t.Helper()

	comp, err := tarifas_sdl.ComponentesDe(fila)
	if err != nil {
		t.Fatalf("ComponentesDe(%s): %v", fila.OperatorCode, err)
	}
	return comp
}
