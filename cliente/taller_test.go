package main

import (
	"testing"
	"time"
)

type TestConfig struct {
	name      string
	numA      int
	numB      int
	numC      int
	slots     int
	mechanics int
}

func TestSanity(t *testing.T) {
	t.Log("sanity OK")
}

func newCarScaled(id int, issue IssueType, scale time.Duration) *Car {
	var base int
	switch issue {
	case MECH:
		base = 5
	case ELECTRIC:
		base = 3
	default:
		base = 1
	}
	return &Car{
		id:       id,
		issue:    issue,
		duration: time.Duration(base) * scale,
	}
}

func genCarsScaled(numA, numB, numC int, scale time.Duration) map[int]*Car {
	cars := make(map[int]*Car, numA+numB+numC)
	id := 0
	for i := 0; i < numA; i++ {
		cars[id] = newCarScaled(id, MECH, scale)
		id++
	}
	for i := 0; i < numB; i++ {
		cars[id] = newCarScaled(id, ELECTRIC, scale)
		id++
	}
	for i := 0; i < numC; i++ {
		cars[id] = newCarScaled(id, BODY, scale)
		id++
	}
	return cars
}

func runSimulation(cfg TestConfig, scale time.Duration) time.Duration {
	g := newGarage(cfg.slots)
	g.sts.Store(4)

	carspool := genCarsScaled(cfg.numA, cfg.numB, cfg.numC, scale)
	numCars := cfg.numA + cfg.numB + cfg.numC

	docChans := initPhaseChans()
	repChans := initPhaseChans()
	cleanChans := initPhaseChans()
	deliverChans := initPhaseChans()
	var noExits [3]chan *Car

	events := make(chan Event, 500)
	finish := make(chan struct{})
	stopDoc := make(chan struct{})
	stopRep := make(chan struct{})
	stopClean := make(chan struct{})
	stopDeliver := make(chan struct{})

	go func() {
		for range events {
		}
	}()

	startPhase(g, cfg.slots, docChans, repChans, events, DOCPHASE, stopDoc)
	startPhase(g, cfg.mechanics, repChans, cleanChans, events, REPAIRPHASE, stopRep)
	startPhase(g, cfg.slots, cleanChans, deliverChans, events, CLEANPHASE, stopClean)
	startPhase(g, cfg.slots, deliverChans, noExits, events, DELIVERYPHASE, stopDeliver)

	start := time.Now()
	go productor(g, carspool, numCars, docChans, finish)
	<-finish
	elapsed := time.Since(start)

	close(stopDoc)
	close(stopRep)
	close(stopClean)
	close(stopDeliver)
	closeChans(docChans)
	closeChans(repChans)
	closeChans(cleanChans)
	closeChans(deliverChans)
	close(events)

	return elapsed
}

func TestComparativas(t *testing.T) {
	const scale = 50 * time.Millisecond
	const iterations = 3

	configs := []TestConfig{
		{"T1_A10_B10_C10_P6_M3", 10, 10, 10, 6, 3},
		{"T2_A20_B5_C5_P6_M3", 20, 5, 5, 6, 3},
		{"T3_A5_B5_C20_P6_M3", 5, 5, 20, 6, 3},
		{"T4_A10_B10_C10_P4_M4", 10, 10, 10, 4, 4},
		{"T5_A20_B5_C5_P4_M4", 20, 5, 5, 4, 4},
		{"T6_A5_B5_C20_P4_M4", 5, 5, 20, 4, 4},
	}

	for _, cfg := range configs {
		cfg := cfg
		t.Run(cfg.name, func(t *testing.T) {
			var total time.Duration
			var min, max time.Duration

			for i := 0; i < iterations; i++ {
				d := runSimulation(cfg, scale)
				total += d
				if i == 0 || d < min {
					min = d
				}
				if i == 0 || d > max {
					max = d
				}
			}

			mean := total / time.Duration(iterations)
			numCars := cfg.numA + cfg.numB + cfg.numC
			perCar := mean / time.Duration(numCars)

			t.Logf("Coches: %d (A=%d B=%d C=%d) | Plazas=%d Mecánicos=%d",
				numCars, cfg.numA, cfg.numB, cfg.numC, cfg.slots, cfg.mechanics)
			t.Logf("Media=%v | Min=%v | Max=%v | Por coche=%v",
				mean, min, max, perCar)
		})
	}
}
