package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/denniswinter/nginx-log-exporter/tail"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/satyrius/gonx"
)

// Metrics is a struct containing pointers
type Metrics struct {
	countTotal          *prometheus.CounterVec
	bytesTotal          *prometheus.CounterVec
	upstreamSeconds     *prometheus.SummaryVec
	upstreamSecondsHist *prometheus.HistogramVec
	upstreamBytes       *prometheus.CounterVec
	responseSeconds     *prometheus.SummaryVec
	responseSecondsHist *prometheus.HistogramVec
	responseBytes       *prometheus.CounterVec
	parseErrorsTotal    prometheus.Counter
}

// Config is a struct
type Config struct {
	LogConfig    LogConfig
	ListenConfig ListenConfig
	Labels       map[string]string `short:"l" long:"labels" description:"Labels which to add to metrics"`
}

// ListenConfig is a struct
type ListenConfig struct {
	ListenAddress string `long:"web.listen-address" default:"0.0.0.0:4040" description:"Address to listen on for web interface and telemetry."`
	TelemetryPath string `long:"web.telemetry-path" default:"/metrics" description:"Path under which to expose metrics"`
}

// LogConfig is a struct
type LogConfig struct {
	FileName string `short:"f" long:"filename" default:"/var/log/nginx/access.log" description:"Path to logfile to parse"`
	Format   string `long:"format" default:"$remote_addr - $remote_user [$time_local] \"$request\" $status $body_bytes_sent \"$http_referer\" \"$http_user_agent\" \"$http_x_forwarded_for\" $request_time" description:"NGINX access_log format"`
}

// Init Initializes a metrics struct
func (m *Metrics) Init() {

	labels := make([]string, 2)
	labels[0] = "status"
	labels[1] = "method"

	m.countTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nginx",
		Name:      "http_response_count_total",
		Help:      "Amount of processes HTTP requests",
	}, labels)

	m.bytesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nginx",
		Name:      "http_response_bytes_total",
		Help:      "Total amount of transferred bytes",
	}, labels)

	m.upstreamSeconds = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "nginx",
		Name:      "http_upstream_time_seconds",
		Help:      "Time needed by upstream servers to handle requests",
	}, labels)

	m.upstreamSecondsHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "nginx",
		Name:      "http_upstream_time_seconds_hist",
		Help:      "Time needed by upstream servers to handle requests",
	}, labels)

	m.upstreamBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nginx",
		Name:      "http_upstream_bytes",
		Help:      "Amount of upstream bytes send",
	}, labels)

	m.responseSeconds = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "nginx",
		Name:      "http_response_time_seconds",
		Help:      "Time needed by nginx to handle requests",
	}, labels)

	m.responseSecondsHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "nginx",
		Name:      "http_response_time_seconds_hist",
		Help:      "Time needed by nginx to handle requests",
	}, labels)

	m.responseBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nginx",
		Name:      "http_response_bytes",
		Help:      "Amount of response bytes send",
	}, labels)

	m.parseErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "nginx",
		Name:      "parse_errors_total",
		Help:      "Total numbers of log file lines that could not be parsed",
	})

	prometheus.MustRegister(m.countTotal)
	prometheus.MustRegister(m.bytesTotal)
	prometheus.MustRegister(m.upstreamSeconds)
	prometheus.MustRegister(m.upstreamSecondsHist)
	prometheus.MustRegister(m.upstreamBytes)
	prometheus.MustRegister(m.responseSeconds)
	prometheus.MustRegister(m.responseSecondsHist)
	prometheus.MustRegister(m.responseBytes)
	prometheus.MustRegister(m.parseErrorsTotal)
}

func main() {
	var cfg Config
	_, err := flags.ParseArgs(&cfg, os.Args)

	if err != nil {
		panic(err)
	}

	t, err := tail.NewFollower(cfg.LogConfig.FileName)
	if err != nil {
		panic(err)
	}

	t.OnError(func(err error) {
		panic(err)
	})

	metrics := Metrics{}
	metrics.Init()

	parser := gonx.NewParser(cfg.LogConfig.Format)

	go processLogFile(cfg, t, parser, &metrics)

	log.Printf("Running HTTP server on address %s\n", cfg.ListenConfig.ListenAddress)

	http.Handle(cfg.ListenConfig.TelemetryPath, prometheus.Handler())
	http.ListenAndServe(cfg.ListenConfig.ListenAddress, nil)
}

func processLogFile(cfg Config, t tail.Follower, parser *gonx.Parser, metrics *Metrics) {
	for line := range t.Lines() {
		entry, err := parser.ParseString(line.Text)
		if err != nil {
			log.Fatalf("Error while parsing line '%s': '%s'", line.Text, err)
			metrics.parseErrorsTotal.Inc()
			continue
		}

		labelValues := make([]string, 2)

		if status, err := entry.Field("status"); err == nil {
			labelValues[0] = status
		}

		if request, err := entry.Field("request"); err == nil {
			chunks := strings.Fields(request)
			labelValues[1] = chunks[0]
		}

		log.Printf("Parsed line '%s'", line.Text)

		metrics.countTotal.WithLabelValues(labelValues...).Inc()

		if bytes, err := entry.FloatField("body_bytes_sent"); err == nil {
			metrics.bytesTotal.WithLabelValues(labelValues...).Add(bytes)
		}

		if upstreamTime, err := entry.FloatField("upstream_response_time"); err == nil {
			metrics.upstreamSeconds.WithLabelValues(labelValues...).Observe(upstreamTime)
			metrics.upstreamSecondsHist.WithLabelValues(labelValues...).Observe(upstreamTime)
		}

		if responseTime, err := entry.FloatField("request_time"); err == nil {
			metrics.responseSeconds.WithLabelValues(labelValues...).Observe(responseTime)
			metrics.responseSecondsHist.WithLabelValues(labelValues...).Observe(responseTime)
		}
	}
}
