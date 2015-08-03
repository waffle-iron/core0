package stats

import (
	"testing"
	"time"
)

func TestStats_Gauage(t *testing.T) {
	signal := make(chan float64)
	statsd := NewStatsd("prefix", 2*time.Second,
		func(stats *Stats) {

			if len(stats.Series) != 1 {
				t.Error("Invalid number of series returned")
			}

			key := stats.Series[0][0].(string)

			if key != "prefix.test" {
				t.Error("Invalid stats key", key)
			}

			signal <- stats.Series[0][1].(float64)
		})

	statsd.Run()
	statsd.Feed("test:+1|g")
	statsd.Feed("test:+4|g")
	statsd.Feed("test:-2|g")
	statsd.Feed("test:+4.5|g")

	select {
	case v := <-signal:
		if v != 7.5 {
			t.Error("Got wrong gauage value", v)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timedout")
	}
}

func TestStats_Counter(t *testing.T) {
	signal := make(chan float64)
	statsd := NewStatsd("prefix", 2*time.Second,
		func(stats *Stats) {

			if len(stats.Series) != 1 {
				t.Error("Invalid number of series returned")
			}
			key := stats.Series[0][0].(string)

			if key != "prefix.test" {
				t.Error("Invalid stats key", key)
			}

			signal <- stats.Series[0][1].(float64)
		})

	statsd.Run()
	statsd.Feed("test:1|c")
	statsd.Feed("test:2|c")
	statsd.Feed("test:1|c")
	statsd.Feed("test:0.5|c")

	select {
	case v := <-signal:
		if v != 4.5 {
			t.Error("Got wrong counter value", v)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timedout")
	}
}

func TestStats_KV(t *testing.T) {
	signal := make(chan float64)
	statsd := NewStatsd("prefix", 2*time.Second,
		func(stats *Stats) {

			if len(stats.Series) != 1 {
				t.Error("Invalid number of series returned")
			}
			key := stats.Series[0][0].(string)

			if key != "prefix.test" {
				t.Error("Invalid stats key", key)
			}

			signal <- stats.Series[0][1].(float64)
		})

	statsd.Run()
	statsd.Feed("test:1|kv")
	statsd.Feed("test:2|kv")
	statsd.Feed("test:1|kv")
	statsd.Feed("test:0.5|kv")

	select {
	case v := <-signal:
		if v != 0.5 {
			t.Error("Got wrong kv value", v)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timedout")
	}
}

func TestStats_Set(t *testing.T) {
	signal := make(chan float64)
	statsd := NewStatsd("prefix", 2*time.Second,
		func(stats *Stats) {

			if len(stats.Series) != 1 {
				t.Error("Invalid number of series returned")
			}
			key := stats.Series[0][0].(string)

			if key != "prefix.test" {
				t.Error("Invalid stats key", key)
			}

			signal <- stats.Series[0][1].(float64)
		})

	statsd.Run()
	statsd.Feed("test:azmy|s")
	statsd.Feed("test:yahia|s")
	statsd.Feed("test:yahia|s")
	statsd.Feed("test:yahia|s")
	statsd.Feed("test:azmy|s")

	select {
	case v := <-signal:
		if v != 2 {
			t.Error("Got wrong set value", v)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timedout")
	}
}

func TestStats_Timer(t *testing.T) {
	signal := make(chan float64)
	statsd := NewStatsd("prefix", 2*time.Second,
		func(stats *Stats) {

			if len(stats.Series) != 1 {
				t.Error("Invalid number of series returned")
			}
			key := stats.Series[0][0].(string)

			if key != "prefix.test" {
				t.Error("Invalid stats key", key)
			}

			signal <- stats.Series[0][1].(float64)
		})

	statsd.Run()
	statsd.Feed("test:100|ms")
	statsd.Feed("test:150|ms")
	statsd.Feed("test:90|ms")
	statsd.Feed("test:122|ms")
	statsd.Feed("test:70|ms")

	select {
	case v := <-signal:
		if v != 106.4 {
			t.Error("Got wrong timer value", v)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timedout")
	}
}

func TestStats_Mixed(t *testing.T) {
	signal := make(chan *Stats)
	statsd := NewStatsd("prefix", 2*time.Second,
		func(stats *Stats) {
			signal <- stats
		})

	statsd.Run()
	statsd.Feed("gauage:+1|g")
	statsd.Feed("gauage:+3|g")

	statsd.Feed("counter:1|c")
	statsd.Feed("counter:2|c")

	statsd.Feed("timer:90|ms")
	statsd.Feed("timer:122|ms")
	statsd.Feed("timer:70|ms")

	statsd.Feed("set:azmy|s")
	statsd.Feed("set:azmy|s")
	statsd.Feed("set:yahia|s")

	statsd.Feed("kv:1|kv")
	statsd.Feed("kv:2|kv")
	statsd.Feed("kv:3|kv")

	values := map[string]float64{
		"prefix.gauage":  4.,
		"prefix.counter": 3.,
		"prefix.timer":   94.,
		"prefix.set":     2.,
		"prefix.kv":      3.,
	}

	select {
	case stats := <-signal:
		if len(values) != len(stats.Series) {
			t.Error("Invalid number of stats", len(stats.Series))
		}

		for i := 0; i < len(stats.Series); i++ {
			key := stats.Series[i][0].(string)
			value := stats.Series[i][1].(float64)

			expectedValue, ok := values[key]
			if !ok {
				t.Error("Got unexpected stats", key)
			}

			if expectedValue != value {
				t.Error("Got invalid value", value, "for key", key)
			}
		}
	case <-time.After(5 * time.Second):
		t.Error("Timedout")
	}

	//wait for the next flush, values should be reset according to
	//value type

	values = map[string]float64{
		"prefix.gauage":  4.,
		"prefix.counter": 0.,
		"prefix.timer":   0.,
		"prefix.set":     0.,
		"prefix.kv":      0.,
	}

	select {
	case stats := <-signal:
		if len(values) != len(stats.Series) {
			t.Error("Invalid number of stats", len(stats.Series))
		}

		for i := 0; i < len(stats.Series); i++ {
			key := stats.Series[i][0].(string)
			value := stats.Series[i][1].(float64)

			expectedValue, ok := values[key]
			if !ok {
				t.Error("Got unexpected stats", key)
			}

			if expectedValue != value {
				t.Error("Got invalid value", value, "for key", key)
			}
		}
	case <-time.After(5 * time.Second):
		t.Error("Timedout")
	}
}
