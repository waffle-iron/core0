//run the test with
//go test -v -parallel 10

package stats

import (
    "testing"
    "os"
    "time"
    "log"

    "sync"
)

var statsd *Statsd
func TestMain(m *testing.M) {
    var wg sync.WaitGroup

    wg.Add(3)

    values := make(map[string]float64)

    statsd = NewStatsd(10 * time.Second, func (key string, value float64){
        values[key] = value
        wg.Done()
    })

    statsd.Run()
    exit := m.Run()

    //all tested passes, now wait for avg to be flushed.
    log.Println("Faiting for flushing in 10 second")

    wg.Wait()

    if values["a"] != 2.0 {
        log.Fatal("Invalid average value for 'a'", values["a"])
    }

    if values["b"] != 3.0 {
        log.Fatal("Invalid average value for 'b'", values["b"])
    }

    if values["c"] != 3.2 {
        log.Fatal("Invalid average value for 'c'", values["c"])
    }

    os.Exit(exit)
}

func TestFeed_a(t *testing.T) {
    t.Parallel()

    values := []float64{1.0, 2.0, 4.0, 1.0} // avg 2.0
    for _, v := range values {
        statsd.Avg("a", v)
    }
}

func TestFeed_b(t *testing.T) {
    t.Parallel()

    values := []float64{2.0, 4.0, 1.0, 5.0} // avg 3.0
    for _, v := range values {
        statsd.Avg("b", v)
    }
}

func TestFeed_c(t *testing.T) {
    t.Parallel()

    values := []float64{1.0, 2.0, 4.0, 6.0, 3.0} // avg 3.2
    for _, v := range values {
        statsd.Avg("c", v)
    }
}

