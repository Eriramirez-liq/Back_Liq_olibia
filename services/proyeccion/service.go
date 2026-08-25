package proyeccion

import (
	"context"
	"fmt"
	"sort"

	"bia-bills/repositories"

	"github.com/biaenergy/bia-commons-go/tracing"
)

// Servicio de Proyección Cargos OR — la parte que ya vive en las bases de BIA.
//
// ⚠️ TRASVASE: archivo del paquete proyeccion. Ver docs/backend/migracion-a-go.md.
//
// ── Qué hace y qué NO ───────────────────────────────────────────────────────
// La proyección completa cuelga de UNA variable: la demanda del mes, que sale de
// Facturación y todavía vive en Supabase. Con ella el motor reparte la energía por
// nivel de tensión, calcula la reactiva y la STR Energy, y valoriza.
//
// Este servicio NO calcula nada de eso. Devuelve las dos piezas que ya están en
// las bases de BIA —los precios por nivel y el valor STR del mes— para que la
// pantalla muestre datos reales en vez de una matriz vacía, y deje en "pendiente"
// lo que depende de la demanda.
//
// Cuando Facturación se migre, la demanda entra por acá y el resto se completa.

// ── Los porcentajes del reparto ─────────────────────────────────────────────
//
// Son la regla de negocio de la proyección: cómo se reparte la demanda del mes en
// energía por nivel y cuánta reactiva y STR genera. Viven acá y no en el front
// porque son negocio, no presentación, y porque la pantalla los muestra como
// etiqueta al lado de cada fila: si el front los tuviera duplicados, un cambio
// acá dejaría la etiqueta mintiendo.
//
// Portados del motor TypeScript sin cambiarlos.
type PorNT struct {
	NT1 float64 `json:"nt1"`
	NT2 float64 `json:"nt2"`
	NT3 float64 `json:"nt3"`
}

// Porcentajes es el reparto que aplica el motor a la demanda del mes.
type Porcentajes struct {
	// Desglose de la activa por nivel. Suman 1.
	ActivaNT PorNT `json:"activa_nt"`
	// La reactiva total es este porcentaje de la energía SDL.
	ReactivaPct float64 `json:"reactiva_pct"`
	// Desglose de la reactiva por nivel, como fracción del total reactivo.
	ReactivaNT PorNT `json:"reactiva_nt"`
	// La STR Energy es la activa total por (1 + este porcentaje).
	StrPct float64 `json:"str_pct"`
}

// PorcentajesVigentes son los del motor, tal cual.
func PorcentajesVigentes() Porcentajes {
	return Porcentajes{
		ActivaNT:    PorNT{NT1: 0.3452, NT2: 0.5296, NT3: 0.1252},
		ReactivaPct: 0.0229,
		ReactivaNT:  PorNT{NT1: 0.4736, NT2: 0.4857, NT3: 0.04},
		StrPct:      0.08,
	}
}

// PreciosMes son los precios por nivel de tensión y el valor STR de un período.
//
// Los punteros distinguen "no hay dato" de "el dato es cero". Un mes puede tener
// tarifas SDL y todavía no tener cargos STR, o al revés: mostrarlo como cero sería
// afirmar algo que no se sabe.
type PreciosMes struct {
	Period string `json:"period"`

	// Promedio de las tarifas de ese nivel entre todos los operadores. nil si el
	// período no tiene tarifas cargadas.
	ActivaNT1 *float64 `json:"activa_nt1"`
	ActivaNT2 *float64 `json:"activa_nt2"`
	ActivaNT3 *float64 `json:"activa_nt3"`

	ReactivaNT1 *float64 `json:"reactiva_nt1"`
	ReactivaNT2 *float64 `json:"reactiva_nt2"`
	ReactivaNT3 *float64 `json:"reactiva_nt3"`

	// Suma de los cargos STR del período, en COP. nil si el período no tiene.
	StrTotalCop *float64 `json:"str_total_cop"`

	// Cuántos operadores respaldan los promedios. Sirve para saber si un mes está
	// completo: con 21 están todos, con menos falta cargar alguno.
	Operators int `json:"operators"`

	// true en los meses que no tienen datos y se proyectaron.
	Projected bool `json:"projected"`
}

type ProyeccionService interface {
	// Prices devuelve los precios y el valor STR de cada período, del más viejo al
	// más nuevo, y agrega `months` meses proyectados al final.
	//
	// Sin períodos, usa todos los que tengan datos.
	Prices(ctx context.Context, periods []string, months int) ([]PreciosMes, error)
}

type proyeccionService struct {
	sdl repositories.LiquidationsSdlRepository
	str repositories.LiquidationsStrRepository
}

func NewProyeccionService(
	sdl repositories.LiquidationsSdlRepository,
	str repositories.LiquidationsStrRepository,
) ProyeccionService {
	return proyeccionService{sdl: sdl, str: str}
}

// acumulador de un nivel de tensión: suma y cantidad, para el promedio.
type acumulado struct {
	sumaActiva   float64
	sumaReactiva float64
	n            int
}

func (a *acumulado) sumar(activa, reactiva float64) {
	a.sumaActiva += activa
	a.sumaReactiva += reactiva
	a.n++
}

func (a acumulado) promedios() (activa, reactiva *float64) {
	if a.n == 0 {
		return nil, nil
	}
	pa := a.sumaActiva / float64(a.n)
	pr := a.sumaReactiva / float64(a.n)

	return &pa, &pr
}

// Tope de meses a proyectar, para que un query no pida mil.
const maxMesesProyeccion = 24

// Meses reales que se promedian para proyectar. Es la ventana del motor original.
const ventanaPromedio = 6

func (service proyeccionService) Prices(
	ctx context.Context,
	periods []string,
	months int,
) ([]PreciosMes, error) {
	ctx, span := tracing.StartSpan(ctx, "services.proyeccion.Prices")
	defer span.End()

	// Sin períodos pedidos, se muestran todos los que tengan algo: los que tienen
	// tarifas SDL más los que tienen cargos STR. Antes la lista de meses salía de
	// Facturación, y sin ella la matriz quedaba vacía aunque hubiera datos.
	if len(periods) == 0 {
		var err error
		periods, err = service.periodosConDatos(ctx)
		if err != nil {
			return nil, err
		}
	}
	if len(periods) == 0 {
		return []PreciosMes{}, nil
	}

	tarifas, err := service.sdl.CurrentRates(ctx, repositories.SdlRateFilter{Periods: periods})
	if err != nil {
		return nil, err
	}

	totalesStr, err := service.str.TotalsByPeriod(ctx, periods)
	if err != nil {
		return nil, err
	}

	// ── El promedio por nivel ────────────────────────────────────────────────
	//
	// Replica exactamente lo que hacía el endpoint TypeScript sobre la tabla de
	// Supabase, que guardaba las tarifas en formato LARGO: una fila por operador,
	// nivel y propiedad de activos, y promediaba todas las filas del nivel.
	//
	// Nuestra tabla es ANCHA —una fila por operador con las diez tarifas— así que
	// el equivalente es: el nivel 1 promedia sus TRES propiedades (OR, compartido
	// y usuario) de cada operador, y los niveles 2 y 3 la única que tienen. Con 21
	// operadores eso da 63 valores para el nivel 1 y 21 para cada uno de los otros,
	// que es lo mismo que contaba la versión anterior.
	//
	// Promediar solo una propiedad daría un número distinto y cambiaría la
	// proyección sin que nadie lo pidiera.
	porPeriodo := map[string]*[3]acumulado{}
	operadores := map[string]map[string]bool{}
	for _, t := range tarifas {
		niveles, existe := porPeriodo[t.Period]
		if !existe {
			niveles = &[3]acumulado{}
			porPeriodo[t.Period] = niveles
			operadores[t.Period] = map[string]bool{}
		}
		operadores[t.Period][t.OperatorCode] = true

		niveles[0].sumar(t.ActiveLevel1Operator, t.ReactiveLevel1Operator)
		niveles[0].sumar(t.ActiveLevel1Shared, t.ReactiveLevel1Shared)
		niveles[0].sumar(t.ActiveLevel1User, t.ReactiveLevel1User)

		niveles[1].sumar(t.ActiveLevel2User, t.ReactiveLevel2User)
		niveles[2].sumar(t.ActiveLevel3User, t.ReactiveLevel3User)
	}

	// Del más VIEJO al más nuevo: la pantalla es una matriz con los meses como
	// columnas y se lee de izquierda a derecha, así que enero va antes que julio y
	// los proyectados quedan al final, después del último real.
	ordenados := append([]string{}, periods...)
	sort.Strings(ordenados)

	salida := make([]PreciosMes, 0, len(ordenados))
	for _, periodo := range ordenados {
		fila := PreciosMes{Period: periodo}

		if niveles, hay := porPeriodo[periodo]; hay {
			fila.ActivaNT1, fila.ReactivaNT1 = niveles[0].promedios()
			fila.ActivaNT2, fila.ReactivaNT2 = niveles[1].promedios()
			fila.ActivaNT3, fila.ReactivaNT3 = niveles[2].promedios()
			fila.Operators = len(operadores[periodo])
		}

		if total, hay := totalesStr[periodo]; hay {
			t := total
			fila.StrTotalCop = &t
		}

		salida = append(salida, fila)
	}

	return append(salida, service.proyectar(salida, months)...), nil
}

// proyectar arma los meses siguientes al último real, con el promedio de precios
// de los últimos meses que sí tienen datos.
//
// Es la misma regla del motor original: los precios de un mes futuro se estiman
// promediando los últimos seis reales. Se promedian solo los meses que TIENEN
// precio, así que un mes sin tarifas no arrastra el promedio hacia abajo.
//
// El valor STR NO se proyecta: es plata que ya se liquidó o no existe todavía.
// Inventar un total daría un número que parece real y no lo es.
func (service proyeccionService) proyectar(reales []PreciosMes, months int) []PreciosMes {
	if months <= 0 || len(reales) == 0 {
		return nil
	}
	if months > maxMesesProyeccion {
		months = maxMesesProyeccion
	}

	// Se promedian los últimos con precios, no los últimos sin más.
	conPrecios := []PreciosMes{}
	for _, m := range reales {
		if m.ActivaNT1 != nil {
			conPrecios = append(conPrecios, m)
		}
	}
	if len(conPrecios) == 0 {
		return nil
	}
	if len(conPrecios) > ventanaPromedio {
		conPrecios = conPrecios[len(conPrecios)-ventanaPromedio:]
	}

	promedio := func(de func(PreciosMes) *float64) *float64 {
		suma, n := 0.0, 0
		for _, m := range conPrecios {
			if v := de(m); v != nil {
				suma += *v
				n++
			}
		}
		if n == 0 {
			return nil
		}
		p := suma / float64(n)

		return &p
	}

	activa1 := promedio(func(m PreciosMes) *float64 { return m.ActivaNT1 })
	activa2 := promedio(func(m PreciosMes) *float64 { return m.ActivaNT2 })
	activa3 := promedio(func(m PreciosMes) *float64 { return m.ActivaNT3 })
	reactiva1 := promedio(func(m PreciosMes) *float64 { return m.ReactivaNT1 })
	reactiva2 := promedio(func(m PreciosMes) *float64 { return m.ReactivaNT2 })
	reactiva3 := promedio(func(m PreciosMes) *float64 { return m.ReactivaNT3 })

	ultimo := reales[len(reales)-1].Period
	proyectados := make([]PreciosMes, 0, months)
	for i := 1; i <= months; i++ {
		ultimo = mesSiguiente(ultimo)
		proyectados = append(proyectados, PreciosMes{
			Period:      ultimo,
			Projected:   true,
			ActivaNT1:   activa1,
			ActivaNT2:   activa2,
			ActivaNT3:   activa3,
			ReactivaNT1: reactiva1,
			ReactivaNT2: reactiva2,
			ReactivaNT3: reactiva3,
		})
	}

	return proyectados
}

// mesSiguiente pasa "2026-12" a "2027-01".
func mesSiguiente(periodo string) string {
	anio, mes, err := partirPeriodo(periodo)
	if err != nil {
		return periodo
	}
	if mes == 12 {
		return fmt.Sprintf("%04d-01", anio+1)
	}

	return fmt.Sprintf("%04d-%02d", anio, mes+1)
}

func partirPeriodo(periodo string) (anio, mes int, err error) {
	_, err = fmt.Sscanf(periodo, "%d-%d", &anio, &mes)
	if err != nil || mes < 1 || mes > 12 {
		return 0, 0, fmt.Errorf("período inválido: %q", periodo)
	}

	return anio, mes, nil
}

// periodosConDatos es la unión de los períodos con tarifas SDL y con cargos STR.
//
// Un mes entra si tiene ALGUNA de las dos cosas: puede haber tarifas cargadas y
// todavía no los cargos, o al revés, y en los dos casos hay algo que mostrar.
func (service proyeccionService) periodosConDatos(ctx context.Context) ([]string, error) {
	conTarifas, err := service.sdl.PeriodsWithRates(ctx)
	if err != nil {
		return nil, err
	}

	conCargos, err := service.str.PeriodsWithCharges(ctx)
	if err != nil {
		return nil, err
	}

	vistos := map[string]bool{}
	union := []string{}
	for _, p := range append(conTarifas, conCargos...) {
		if p != "" && !vistos[p] {
			vistos[p] = true
			union = append(union, p)
		}
	}

	return union, nil
}
