# Práctica 4 - Sistemas Distribuidos - RAUL MOLINA
## Concurrencia en Go - El Taller del Pueblo

---

## 1. Descripción del problema

Se modela un taller mecánico con **N coches** que deben pasar por 4 fases secuenciales:

1. **Documentación** — preparación del coche
2. **Reparación** — atendido por un mecánico
3. **Limpieza** — preparación para entrega
4. **Entrega** — revisión final y salida

Los coches tienen tres categorías con distinta prioridad y tiempo por fase:

| Categoría | Incidencia | Prioridad | Tiempo/fase |
|-----------|-----------|-----------|-------------|
| A | Mecánica | Alta | 5s |
| B | Eléctrica | Media | 3s |
| C | Carrocería | Baja | 1s |

El taller reacciona a los estados enviados por la mutua a través de un servidor TCP:

| Estado | Comportamiento |
|--------|---------------|
| 0, 9 | Taller cerrado |
| 1, 2, 3 | Solo accede la categoría correspondiente |
| 4, 5, 6 | Prioridad para la categoría correspondiente |
| 7, 8 | Se mantiene el estado anterior |

---

## 2. Arquitectura del sistema

El sistema está compuesto por tres programas independientes:

- **servidor.go** — broker TCP en localhost:8000, rebroadcast de mensajes
- **mutua.go** — envía números aleatorios al servidor simulando cambios de estado
- **taller.go** — cliente que recibe los estados y simula el taller

Dentro del taller hay un pipeline de 4 fases conectadas por canales. Cada fase tiene un grupo de workers (goroutines) que procesan los coches en paralelo respetando prioridades , se podria haber separado pero no pense que iba a tener tanto tamaño cuando empece y esta hecho tal y como avanzaba todo con sentido,creo que es mejor todo en taller.go

---

## 3. Estructura de archivos
- **servidor.go** — Broker TCP 
 - **mutua.go** — Simulador de la mutua 
- **taller.go** — Lógica del taller (archivo principal)
- **types.go** — Tipos y estructuras de datos
- **taller_test.go** — Tests 
---

## 4. Cómo ejecutar

Abrir tres terminales en orden:

```bash
# Terminal 1
go run servidor.go

# Terminal 2
go run taller.go types.go

# Terminal 3
go run mutua.go
```

Para ejecutar los tests:

```bash
go test 
```

---

## 5. Decisiones técnicas

**Prioridad sin heap** — Se usan 3 canales por fase (uno por categoría). La función `getCar` implementa `select` anidados: intenta primero el canal de mayor prioridad sin bloquearse, y si no hay nada se bloquea esperando cualquiera de los tres.

**Semáforo de plazas** — `freeSlots` es un canal buffered con N tokens. Coger plaza equivale a recibir un token, liberar plaza a devolverlo. Es el patrón productor/consumidor del temario.

**Estado atómico** — El estado de la mutua se almacena en `atomic.Int32` para evitar mutex en lecturas concurrentes desde múltiples goroutines.

**Logger centralizado** — Solo una goroutine escribe por stdout, recibiendo eventos por canal. Evita condiciones de carrera en la salida.

**Pipeline de fases** — Cada fase es un conjunto de workers independientes conectados por canales tipados. El coche fluye de fase en fase sin bloquear el pipeline completo.

---

## 6. Diagramas

- `diagrama_arquitectura` — Arquitectura general del sistema con los tres componentes y flujo de datos
- `diagrama_secuencia` — Ciclo de vida completo de un coche a través de las 4 fases
- `diagrama_clases` — Tipos y relaciones entre Car, Event y Garage

---

## 7. Resultados de los tests
Los resultados de los tests se encuentran en la carpeta test
el test ejecuta el 44% del codigo tiene sentido ya que para el test realmente solo usamos taller.go y types.go 
Los tests ejecutan 6 comparativas c y 3 iteraciones cada una:


| Test | Coches (A/B/C) | Plazas | Mecánicos | Media |
|------|---------------|--------|-----------|-------|
| T1 | 10/10/10 | 6 | 3 | ~3.3s |
| T2 | 20/5/5 | 6 | 3 | ~4.2s |
| T3 | 5/5/20 | 6 | 3 | ~2.2s |
| T4 | 10/10/10 | 4 | 4 | ~4.6s |
| T5 | 20/5/5 | 4 | 4 | ~6.2s |
| T6 | 5/5/20 | 4 | 4 | ~3.0s |

**Observaciones:**
- T2 y T5 son los más lentos: más coches de categoría A (5s/fase) aumentan el tiempo total
- T3 y T6 son los más rápidos: mayoría de coches de categoría C (1s/fase)
- Reducir plazas de 6 a 4 aumenta el tiempo en todos los tests por mayor contención en la entrada
