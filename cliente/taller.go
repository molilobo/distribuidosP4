/*
ESTE ES EL ÚNICO ARCHIVO QUE SE PUEDE MODIFICAR

RECOMENDACIÓN: Solo modicar a partir de la parte
				donde se encuentran la explicación
				de las otras variables.

*/

package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	NumPlazas    = 6
	NumMecanicos = 3
	NumCochesA   = 10
	NumCochesB   = 10
	NumCochesC   = 10
)

var (
	buf    bytes.Buffer
	logger = log.New(&buf, "logger: ", log.Lshortfile)
	msg    string
)

func newGarage(numSlots int) *Garage {
	g := &Garage{
		cars:      make(map[int]*Car),
		freeSlots: make(chan struct{}, numSlots),
	}
	for i := 0; i < numSlots; i++ {
		g.freeSlots <- struct{}{}
	}
	return g
}

func genCars(numA, numB, numC int) map[int]*Car {
	cars := make(map[int]*Car, numA+numB+numC)
	id := 0
	for i := 0; i < numA; i++ {
		cars[id] = newCar(id, MECH)
		id++
	}
	for i := 0; i < numB; i++ {
		cars[id] = newCar(id, ELECTRIC)
		id++
	}
	for i := 0; i < numC; i++ {
		cars[id] = newCar(id, BODY)
		id++
	}
	return cars
}

func newCar(id int, issue IssueType) *Car {
	lag := float64(rand.Intn(21)) / 10.0
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
		duration: time.Duration((float64(base) + lag) * float64(time.Second)),
	}
}
func initPhaseChans() [3]chan *Car {
	var chans [3]chan *Car
	for i := range chans {
		chans[i] = make(chan *Car)
	}
	return chans
}
func closeChans(chans [3]chan *Car) {
	for i := range chans {
		close(chans[i])
	}
}

func sendCar(chans [3]chan *Car, c *Car) {
	switch c.issue {
	case MECH:
		chans[0] <- c
	case ELECTRIC:
		chans[1] <- c
	case BODY:
		chans[2] <- c
	}
}

func getCar(g *Garage, chans [3]chan *Car, stop <-chan struct{}) *Car {
	mode := g.sts.Load()
	high, medium, low := int32(0), int32(1), int32(2)
	if mode >= 4 && mode <= 6 {
		high = mode - 4
		if mode != 4 {
			medium = 0
		}
		if mode == 6 {
			low = 1
		}
	}
	select {
	case c := <-chans[high]:
		return c
	default:
		select {
		case c := <-chans[medium]:
			return c
		default:
			select {
			case c := <-chans[high]:
				return c
			case c := <-chans[medium]:
				return c
			case c := <-chans[low]:
				return c
			case <-stop:
				return nil
			}
		}
	}
}
func productor(g *Garage, carspool map[int]*Car, numCars int, docChans [3]chan *Car, finish chan struct{}) {
	for len(carspool) > 0 {
		mode := g.sts.Load()
		if mode == 0 || mode == 9 {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		// esperar plaza libre
		<-g.freeSlots
		c := getCarFromQ(carspool, g)
		if c == nil {
			g.freeSlots <- struct{}{}
			break
		}
		c.start = time.Now()
		g.signInCar(c)
		g.wg.Add(1)
		sendCar(docChans, c)
	}
	g.wg.Wait()
	finish <- struct{}{}
}

func worker(g *Garage, entrys [3]chan *Car, exits [3]chan *Car, events chan<- Event, phase int, stop <-chan struct{}) {
	for {
		c := getCar(g, entrys, stop)
		if c == nil {
			return
		}
		g.updatePhase(c.id, phase)
		genEvent(events, c, "entra")
		time.Sleep(c.duration)
		genEvent(events, c, "sale")

		if phase == DELIVERYPHASE {
			g.delCar(c.id)
			g.freeSlots <- struct{}{} // libera plaza
			g.wg.Done()
		} else {
			sendCar(exits, c)
		}
	}
}

func startPhase(g *Garage, nWorkers int, entrys [3]chan *Car, exits [3]chan *Car, events chan<- Event, phase int, stop <-chan struct{}) {
	for i := 0; i < nWorkers; i++ {
		go worker(g, entrys, exits, events, phase, stop)
	}
}
func logManager(events <-chan Event) {
	fmt.Printf("%-10s %-10s %-12s %-6s %-8s\n", "Tiempo[s]", "Coche[id]", "Incidencia", "Fase", "Estado")
	fmt.Println("--------------------------------------------------")
	for e := range events {
		fmt.Printf("%-10.2f %-10d %-12s %-6d %-8s\n",
			e.elapsed.Seconds(),
			e.car,
			e.issue,
			e.phase,
			e.status,
		)
	}
}

func genEvent(events chan<- Event, c *Car, sts string) {
	events <- Event{
		elapsed: time.Since(c.start),
		car:     c.id,
		phase:   c.curphase,
		status:  sts,
		issue:   c.issue,
	}
}

func (g *Garage) signInCar(c *Car) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cars[c.id] = c
}

func (g *Garage) updatePhase(id int, phase int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if c, ok := g.cars[id]; ok {
		c.curphase = phase
	}
}

func (g *Garage) delCar(id int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.cars, id)
}
func getCarFromM(cars map[int]*Car, issue IssueType) *Car {
	for id, c := range cars {
		if c.issue == issue {
			delete(cars, id)
			return c
		}
	}
	return nil
}

func getCarFromQ(cars map[int]*Car, g *Garage) *Car {
	if len(cars) == 0 {
		return nil
	}
	for {
		mode := g.sts.Load()
		switch mode {
		case 1:
			if c := getCarFromM(cars, MECH); c != nil {
				return c
			}
			time.Sleep(300 * time.Millisecond)
		case 2:
			if c := getCarFromM(cars, ELECTRIC); c != nil {
				return c
			}
			time.Sleep(300 * time.Millisecond)
		case 3:
			if c := getCarFromM(cars, BODY); c != nil {
				return c
			}
			time.Sleep(300 * time.Millisecond)
		case 4:
			if c := getCarFromM(cars, MECH); c != nil {
				return c
			}
			if c := getCarFromM(cars, ELECTRIC); c != nil {
				return c
			}
			return getCarFromM(cars, BODY)
		case 5:
			if c := getCarFromM(cars, ELECTRIC); c != nil {
				return c
			}
			if c := getCarFromM(cars, MECH); c != nil {
				return c
			}
			return getCarFromM(cars, BODY)
		case 6:
			if c := getCarFromM(cars, BODY); c != nil {
				return c
			}
			if c := getCarFromM(cars, MECH); c != nil {
				return c
			}
			return getCarFromM(cars, ELECTRIC)
		default:
			time.Sleep(300 * time.Millisecond)
		}
		if len(cars) == 0 {
			return nil
		}
	}
}
func main() {
	conn, err := net.Dial("tcp", "localhost:8000")
	if err != nil {
		logger.Fatal(err)
	}
	defer conn.Close()
	buf := make([]byte, 512)

	g := newGarage(NumPlazas)
	carspool := genCars(NumCochesA, NumCochesB, NumCochesC)

	// canales entre fases
	docChans := initPhaseChans()
	repChans := initPhaseChans()
	cleanChans := initPhaseChans()
	deliverChans := initPhaseChans()

	events := make(chan Event)
	finish := make(chan struct{})
	stopDoc := make(chan struct{})
	stopRep := make(chan struct{})
	stopClean := make(chan struct{})
	stopDeliver := make(chan struct{})

	go logManager(events)
	startPhase(g, NumPlazas, docChans, repChans, events, DOCPHASE, stopDoc)
	startPhase(g, NumMecanicos, repChans, cleanChans, events, REPAIRPHASE, stopRep)
	startPhase(g, NumPlazas, cleanChans, deliverChans, events, CLEANPHASE, stopClean)
	startPhase(g, NumPlazas, deliverChans, [3]chan *Car{}, events, DELIVERYPHASE, stopDeliver)

	go productor(g, carspool, NumCochesA+NumCochesB+NumCochesC, docChans, finish)
	for {
		n, err := conn.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			continue
		}
		if n > 0 {
			msg = strings.TrimSpace(string(buf[:n]))
			sts, err := strconv.Atoi(msg)
			if err != nil {
				continue
			}
			if sts != 7 && sts != 8 {
				g.sts.Store(int32(sts))
			}
			select {
			case <-finish:
				close(stopDoc)
				close(stopRep)
				close(stopClean)
				close(stopDeliver)
				closeChans(docChans)
				closeChans(repChans)
				closeChans(cleanChans)
				closeChans(deliverChans)
				close(events)
				fmt.Println("Simulación completada.")
				return
			default:
			}
			fmt.Println("len: " + strconv.Itoa(n) + " msg: " + msg)
		}
	}
}
