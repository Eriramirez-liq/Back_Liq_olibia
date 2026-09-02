# 03 — Diagramas de Flujo

Área de Liquidaciones · Versión 1.0 — 2026-08-31

Los diagramas complementan las [narrativas](./02-narrativas-procesos.md). Cada punto de control
aparece rotulado con su código `C-xx` de la [matriz de control](./04-matriz-de-control.md).

---

## 1. Mapa de procesos del área

```mermaid
flowchart LR
  subgraph EXT["Agentes externos"]
    OR["Operadores de Red<br/>21 SDL y TC1 · 23 STR"]
    XM["XM<br/>árbitro del proceso"]
  end

  subgraph INT["Procesos internos de BIA"]
    FAC["Facturación / bills<br/>energía facturada y tarifas"]
    OPS["Operaciones<br/>reporte a XM"]
  end

  subgraph LIQ["Área de Liquidaciones"]
    P01["P01 · Gobierno del período<br/>y cargue de fuentes"]
    P02["P02 · Conciliación TC1"]
    P03["P03 · Preliquidación SDL"]
    P04["P04 · COT"]
    P05["P05 · Balances de energía"]
    P06["P06 · Compensaciones"]
    P10["P10 · Cargos STR y OC"]
    P09["P09 · Cierre post-facturación"]
    P07["P07 · Cierre de mes<br/>pérdidas y provisiones"]
    P08["P08 · Reporte TC2"]
  end

  CTB["Contabilidad"]
  NS["NetSuite<br/>orden de compra"]
  USR["Usuario final"]

  OR --> P01
  XM --> P01
  FAC --> P01
  OPS --> XM
  P01 --> P02 & P03 & P04 & P05 & P10
  P03 --> P05
  P03 --> P09
  P02 --> P09
  P05 --> P07
  P03 --> P07
  OR --> P06
  P06 --> P08
  P06 --> USR
  P07 --> CTB
  P08 --> CTB
  P10 --> NS

  classDef manual fill:#fde8e8,stroke:#c0392b,color:#7b241c;
  classDef sistema fill:#e8f4fd,stroke:#2471a3,color:#1a5276;
  class P04,P06,P08 manual;
  class P01,P02,P03,P09,P10 sistema;
```

> **Rojo = proceso sin soporte del sistema.** COT, compensaciones y TC2 se ejecutan íntegramente
> en Excel y correo. Dos de los tres terminan en dinero que sale hacia el usuario final.

---

## 2. Calendario del ciclo mensual

Trabajo del mes calendario **M+1** sobre el consumo del mes **M**.

```mermaid
gantt
  title Ciclo mensual del área — mes M+1 sobre consumos de M
  dateFormat DD
  axisFormat día %d

  section Entradas
  Llegan TC1 de los 21 OR            :a1, 01, 5d
  Descarga Facturación y XM          :milestone, a2, 08, 0d
  Llegan preliquidaciones SDL        :a3, 08, 8d
  Llegan balances de energía         :a4, 24, 6d
  Llega COT                          :a5, 25, 10d

  section Conciliaciones
  Conciliación y respuesta TC1       :b1, 03, 9d
  Conciliación SDL, respuesta en 1 día :crit, b2, 08, 8d
  Validación de balances             :b3, 25, 6d
  Validación COT                     :b4, 26, 9d

  section Reportes
  Cierre de mes a Contabilidad       :milestone, crit, c1, 07, 0d
  Reporte TC2 a Contabilidad         :milestone, crit, c2, 15, 0d
  Cierre post-facturación            :c3, 16, 5d
```

**Los tres cuellos de botella del calendario**

| Fecha | Compromiso | Por qué aprieta |
|---|---|---|
| **Día 7** | Cierre a Contabilidad | Se reporta **antes** de tener la preliquidación SDL del mes (llega desde el día 8): el reporte se arma con la conciliación del período anterior |
| **Día 8-15** | Respuesta al OR en **1 día calendario** | Corre en fines de semana y festivos, y se solapa con TC1 y TC2 |
| **Día 12 / Día 15** | TC1 respondido / TC2 enviado | Ambos dentro de la ventana de mayor carga del mes |

---

## 3. Flujo de datos: de dónde sale cada cifra

```mermaid
flowchart TB
  subgraph F["Fuentes"]
    A1["Facturación BIA<br/>bills · Metabase 73360"]
    A2["Reporte XM<br/>Metabase 76099"]
    A3["Preliquidación SDL<br/>archivo por OR"]
    A4["TC1<br/>plano CREG por OR"]
    A5["G de bolsa<br/>Metabase 1237"]
    A6["Insumos STR y tarifas SDL<br/>XM y OR"]
  end

  subgraph M["Motor del módulo"]
    N1["Normalización:<br/>colapso de fronteras _N,<br/>herencia y dedupe · C-19"]
    N2["Universo = unión de fuentes<br/>C-10"]
    N3["Clasificación A1 a D4<br/>umbral 100 kWh · C-18"]
    N4["Valorización<br/>G + T + D + PR + R · C-33"]
    N5["Congruencia 3 fuentes<br/>C-16"]
    N6["Tarifas SDL por OR,<br/>nivel y propiedad"]
  end

  subgraph S["Salidas"]
    O1["Pérdidas<br/>contingencias L1"]
    O2["Provisiones"]
    O3["Disputas L2<br/>reclamo al OR"]
    O4["Diferencias TC1<br/>NT y propiedad"]
    O5["Indicadores del dashboard"]
    O6["Matriz STR y orden de compra"]
  end

  A1 & A2 & A3 --> N1 --> N2 --> N3 --> N4
  A4 --> N2
  A5 --> N4
  A6 --> N6 --> N4
  N2 --> N5
  N4 --> O1 & O2 & O3
  N2 --> O4
  N4 --> O5
  N5 --> O5
  A6 --> O6
```

---

## 4. P01 — Cargue de una fuente

```mermaid
flowchart TB
  ini(["Analista inicia<br/>Nueva carga"]) --> p1["Selecciona mes de consumo"]
  p1 --> d1{"¿Mes en curso<br/>o futuro?"}
  d1 -->|Sí| x1["RECHAZA · C-02"]:::stop
  d1 -->|No| p2["Selecciona fuente y operador"]
  p2 --> p3["Sube archivo · vista previa · C-09"]
  p3 --> d2{"¿El período del nombre<br/>del archivo coincide?"}
  d2 -->|No| x2["ERROR CRÍTICO<br/>confirmar deshabilitado · C-01"]:::stop
  d2 -->|Sin mes en nombre| w1["Advertencia<br/>continúa"]
  d2 -->|Sí| d3
  w1 --> d3{"¿Existe carga previa<br/>del período, fuente y OR?"}
  d3 -->|No| p6["Confirmar"]
  d3 -->|Sí| d4{"Reemplazar<br/>o agregar"}
  d4 -->|Reemplazar| p4["Justificación obligatoria · C-03"] --> p6
  d4 -->|Agregar| d5{"¿SDL de EEP Pereira<br/>o EPM?"}
  d5 -->|No| x3["RECHAZA"]:::stop
  d5 -->|Sí| p6
  p6 --> p7["Escritura append-only<br/>usuario, fecha, load_id · C-04"]
  p7 --> p8["Estado del período<br/>avance n/21 · C-05"]
  p8 --> fin(["Fuente disponible<br/>para conciliar"])

  classDef stop fill:#fde8e8,stroke:#c0392b,color:#7b241c;
```

---

## 5. P03 — Conciliación SDL, flujo completo

```mermaid
flowchart TB
  d8["Día 8 · descarga Facturación y XM<br/>C-17"] --> rec["Llega preliquidación del OR<br/>días 8 a 15"]
  rec --> car["Cargue · P01"]
  car --> norm["Colapso _N, herencia, dedupe · C-19"]
  norm --> uni["Universo = unión de las 3 fuentes · C-10"]
  uni --> gb["G de bolsa del mes · C-20"]
  gb --> cls["Clasificación por frontera<br/>umbral 100 kWh · C-18"]

  cls --> a1["A1 · sin diferencia"]
  cls --> l1["Línea 1 · BIA vs XM"]
  cls --> l2["Línea 2 · XM vs OR"]
  cls --> inc["INCOMPLETA<br/>falta XM o SDL"]

  l1 --> per["Pérdida<br/>XM por encima de facturación"]
  l1 --> pro["Provisión<br/>facturación por encima de XM"]
  l2 --> dis["Disputa<br/>reclamable al OR"]

  cls --> alm{"¿Caso D1, D2 o D4?"}
  alm -->|Sí| rev["Alerta manual:<br/>revisión obligatoria · C-21"]
  rev --> acc
  per --> acc["Registrar accionable · C-14"]
  pro --> acc
  dis --> acc
  inc --> acc
  acc --> rcl["Reclamo al OR<br/>máximo 1 día calendario · C-22"]
  per --> p07["P07 · reporte del día 7"]
  pro --> p05["P05 · cruce con balance"]
  dis --> seg["Seguimiento de disputa<br/>hasta resuelta o cerrada"]
```

---

## 6. Árbol de decisión de casos SDL

La regla de negocio en una sola imagen: **XM es el árbitro**.

```mermaid
flowchart TB
  ini(["Frontera con E_fac, E_xm, E_sdl<br/>umbral 100 kWh"]) --> q0{"¿Faltan E_xm o E_sdl?"}
  q0 -->|Sí| inc["INCOMPLETA<br/>no se clasifica"]:::gris
  q0 -->|No| q1{"¿BIA igual a XM?"}

  q1 -->|Sí| q2{"¿OR igual a XM?"}
  q2 -->|Sí| a1["A1 · todo cuadra"]:::ok
  q2 -->|"OR cobró de menos"| c1["C1 · DISPUTA<br/>delta por tarifa SDL"]:::disp
  q2 -->|"OR cobró de más"| c2["C2 · DISPUTA<br/>delta por tarifa SDL"]:::disp

  q1 -->|No| q3{"¿BIA por debajo de XM?"}
  q3 -->|"Sí · se reportó de más"| q4{"¿OR alineado con XM?"}
  q4 -->|Sí| b1["B1 · PÉRDIDA<br/>G de bolsa"]:::perd
  q4 -->|"OR entre BIA y XM"| d1["D1 · PÉRDIDA<br/>+ alerta manual"]:::perd
  q3 -->|"No · se reportó de menos"| q5{"¿OR alineado con XM?"}
  q5 -->|Sí| b2["B2 · PROVISIÓN<br/>G de facturación"]:::prov
  q5 -->|"OR igual a BIA"| d3["D3 · PROVISIÓN<br/>distribución neta de tarifa SDL"]:::prov
  q5 -->|"OR entre XM y BIA"| d2["D2 · PROVISIÓN<br/>+ alerta manual"]:::prov
  q3 -->|"Los tres distintos"| d4["D4 · ALERTA MANUAL<br/>+ pérdida o provisión según signo"]:::alert

  classDef ok fill:#e8f8f0,stroke:#1e8449,color:#145a32;
  classDef perd fill:#fdebd0,stroke:#ca6f1e,color:#7e5109;
  classDef prov fill:#eaf2f8,stroke:#2471a3,color:#1a5276;
  classDef disp fill:#f4ecf7,stroke:#7d3c98,color:#4a235a;
  classDef alert fill:#fde8e8,stroke:#c0392b,color:#7b241c;
  classDef gris fill:#f2f3f4,stroke:#7f8c8d,color:#424949;
```

| Resultado | Qué significa para el dinero | Quién lo asume |
|---|---|---|
| **A1** | Nada que hacer | — |
| **Disputa (C1, C2)** | El OR liquidó distinto de XM | **Se reclama al OR** |
| **Pérdida (B1, D1)** | Se reportó a XM más energía de la facturada | **La absorbe BIA** |
| **Provisión (B2, D2, D3)** | Se facturó más de lo reportado; el OR lo cobrará después | **Se provisiona y se libera con el balance** |
| **Alerta manual (D1, D2, D4)** | Las tres fuentes difieren: no hay regla automática | **Decide el analista** |

---

## 7. P05 — Balance de energía y ciclo de la provisión

```mermaid
flowchart LR
  b1["Llega balance del OR<br/>última semana del mes"] --> b2["Verificar energía<br/>contra diferencia de origen · C-25"]
  b2 --> b3["Verificar tarifa<br/>del período de origen · C-26"]
  b3 --> b4{"¿Existe provisión<br/>para esa frontera y período?"}
  b4 -->|Sí| b5["Cruce contra provisión · C-27"]
  b4 -->|No| b6["PÉRDIDA NO ANTICIPADA<br/>investigar antes de aprobar"]:::alerta
  b5 --> b7{"¿Cruce total o parcial?"}
  b7 -->|Total| b8["Provisión liberada"]
  b7 -->|Parcial| b9["Saldo pendiente<br/>seguimiento por antigüedad · C-28"]
  b8 --> b10["Visto bueno al OR<br/>relaciona en factura del mes siguiente"]
  b9 --> b10
  b6 --> b10
  b10 --> b11["Reporte a Contabilidad · P07"]

  classDef alerta fill:#fde8e8,stroke:#c0392b,color:#7b241c;
```

### Ciclo de vida de las posiciones económicas

```mermaid
stateDiagram-v2
  direction LR
  [*] --> Provision_PENDIENTE : conciliación detecta E_fac mayor que E_xm
  Provision_PENDIENTE --> CRUZADO_PARCIAL : llega balance por parte de la energía
  Provision_PENDIENTE --> CRUZADO_TOTAL : llega balance por el total
  CRUZADO_PARCIAL --> CRUZADO_TOTAL : balance posterior
  CRUZADO_TOTAL --> [*] : liberada y reportada

  [*] --> Contingencia_PENDIENTE : conciliación detecta E_fac menor que E_xm
  Contingencia_PENDIENTE --> COBRADO : el OR efectivamente cobra
  COBRADO --> CERRADO : se determina pérdida o ganancia real

  [*] --> Disputa_ABIERTA : el OR liquida distinto de XM
  Disputa_ABIERTA --> EN_GESTION : reclamo enviado al OR
  EN_GESTION --> RESUELTA : el OR ajusta
  EN_GESTION --> CERRADA_SIN_AJUSTE : no procede
```

> **Punto de control (F-07):** el paso a `CERRADA_SIN_AJUSTE` extingue un derecho de cobro contra
> el OR. Hoy lo puede ejecutar el mismo analista que hizo la conciliación, sin aprobación ni
> revisión posterior.

---

## 8. P06 — Compensaciones: el flujo del dinero hacia el usuario

```mermaid
flowchart TB
  i1["Indicador mensual de la frontera<br/>supera el indicador anual del OR"] --> i2["BIA liquida la compensación"]
  i2 --> i3["Se relaciona al OR"]
  i3 --> i4{"¿El OR la valida?"}
  i4 -->|No| i5["Gestión con el OR"]
  i4 -->|Sí| i6["El OR la relaciona<br/>en la factura SDL"]
  i6 --> i7{"¿Reflejada en la factura<br/>SDL recibida? · C-29"}
  i7 -->|No| i8["No se traslada al usuario<br/>queda pendiente"]
  i7 -->|Sí| i9{"¿Usuario activo?"}
  i9 -->|Sí| i10["Reconocimiento<br/>en su factura"]:::ok
  i9 -->|"No · retirado"| i11["PAGO A CUENTA BANCARIA<br/>informada por el usuario"]:::riesgo
  i10 --> i12["Reporte TC2 · solo activos · P08"]
  i8 --> i12
  i11 --> i13["Fuera del TC2<br/>sin reporte de control"]:::riesgo

  classDef ok fill:#e8f8f0,stroke:#1e8449,color:#145a32;
  classDef riesgo fill:#fde8e8,stroke:#c0392b,color:#7b241c;
```

> **Los dos nodos rojos son el punto más expuesto del área.** Un pago en efectivo a un tercero
> que ya no es cliente, calculado por BIA, con datos bancarios recibidos por un canal no
> verificado, ejecutado sin sistema y **excluido del único reporte de control existente (TC2)**.
> Ver [05 — F-04](./05-riesgos-de-fraude.md#f-04--desvío-del-pago-de-compensaciones-a-usuarios-retirados).

---

## 9. P07 — Cierre de mes hacia Contabilidad

```mermaid
flowchart TB
  subgraph PER["Pérdidas del período"]
    e1["Energía reportada de más a XM"] --> e2["× G de bolsa + T + D + PR + R<br/>D y PR por usuario · C-33"] --> e3["Valor de pérdida"]
  end
  subgraph PRO["Movimiento de provisiones"]
    f1["Provisión anterior"] --> f4["Saldo final"]
    f2["− Balances recibidos en el mes"] --> f4
    f3["+ Nueva provisión del mes<br/>× G + T + D + PR + R de facturación"] --> f4
  end
  e3 --> g1{"¿La conciliación se corrió<br/>con la G de bolsa del mes? · C-20"}
  g1 -->|No| g2["Recalcular pérdidas<br/>y rehacer el reporte"]:::alerta
  g2 --> g3
  g1 -->|Sí| g3["Aprobación del Líder · C-35"]
  f4 --> g3
  g3 --> g4["Envío a Contabilidad<br/>día 7"]
  g4 --> g5["Congelar el período · C-36"]
  g5 --> g6["Conciliación de retorno<br/>reportado vs. registrado · C-37"]

  classDef alerta fill:#fde8e8,stroke:#c0392b,color:#7b241c;
```

---

## 10. P10 — Cargos STR: del insumo a la orden de compra

```mermaid
flowchart LR
  s1["Insumos STR del período<br/>validación de nombre · C-01"] --> s2["Matriz por operador<br/>y mes de facturación"]
  s2 --> s3["Crear lote<br/>máximo 25 cargos · C-41"]
  s3 --> s4{"Validaciones<br/>datos, monto distinto de cero,<br/>no procesado antes · C-42"}
  s4 -->|Falla| s5["Rechaza con detalle<br/>del conflicto"]:::stop
  s4 -->|Pasa| s6["Congela el monto<br/>snapshot · C-43"]
  s6 --> s7["Procesar lote<br/>secuencial"]
  s7 --> s8["Orden de compra en NetSuite<br/>proveedor del operador · dpto 131<br/>clave de idempotencia · C-44"]
  s8 --> s9{"¿Respuesta?"}
  s9 -->|OK| s10["PROCESADO<br/>guarda número de OC · C-46"]
  s9 -->|Error| s11["ERROR<br/>reenviable · C-45"]
  s11 --> s7
  s10 --> s12["Conciliación OC emitidas<br/>vs. matriz del período · C-48"]

  classDef stop fill:#fde8e8,stroke:#c0392b,color:#7b241c;
```

### Estados del envío a NetSuite

```mermaid
stateDiagram-v2
  direction LR
  [*] --> PENDIENTE : lote creado
  PENDIENTE --> PROCESANDO : procesar lote
  PROCESANDO --> PROCESADO : NetSuite devuelve número de OC
  PROCESANDO --> ERROR : falla de envío
  ERROR --> PROCESANDO : reenviar, solo con lote en curso
  PENDIENTE --> CANCELADO : cancelar lote
  PROCESADO --> [*]
  CANCELADO --> [*]
```

---

## 11. Vista de segregación de funciones (situación actual)

```mermaid
flowchart LR
  subgraph HOY["Hoy · un mismo perfil ANALISTA"]
    h1["Carga las fuentes"] --> h2["Ejecuta la conciliación"] --> h3["Decide el accionable"] --> h4["Cierra la disputa"] --> h5["Arma el reporte<br/>a Contabilidad"]
  end
  subgraph OBJ["Objetivo"]
    o1["Analista:<br/>carga y concilia"] --> o2["Analista:<br/>propone accionable"] --> o3["Líder:<br/>aprueba accionables<br/>de alto impacto y cierres<br/>sin ajuste"] --> o4["Líder:<br/>aprueba el reporte"] --> o5["Contabilidad:<br/>concilia de retorno"]
  end
  HOY -.->|brecha B-09| OBJ

  classDef riesgo fill:#fde8e8,stroke:#c0392b,color:#7b241c;
  class h1,h2,h3,h4,h5 riesgo;
```

Detalle en [05 — Riesgos de fraude §4](./05-riesgos-de-fraude.md#4-matriz-de-segregación-de-funciones).
