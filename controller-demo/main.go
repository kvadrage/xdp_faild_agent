package main

import (
	"log"
	"net/http"
	"sync"
	"time"
	"fmt"

	"faild-agent/faild"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MyCollector struct {
	faildStats *prometheus.Desc
}

func (c *MyCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.faildStats
}

func (c *MyCollector) Collect(ch chan<- prometheus.Metric) {
	allStats := globalStats.Dump()
	for agent, stats := range(allStats) {
		for metric, value := range(stats.Stats) {
			ch <- prometheus.MustNewConstMetric(c.faildStats, prometheus.CounterValue, float64(value), agent, metric)
		}
	}
}

func newCollector() *MyCollector {
    return &MyCollector{
        faildStats: prometheus.NewDesc("faild_stats",
            "Shows Fails stats",
            []string{"agent", "metric"}, nil,
        ),
    }
}

type GlobalStats struct {
    sync.Mutex
    m  map[string]*faild.Stats
}

func (s *GlobalStats) Get(agent string) (*faild.Stats, bool) {
	s.Lock()
	defer s.Unlock()

	stats, exists := s.m[agent]
	return stats, exists
}

func (s *GlobalStats) Dump() map[string]*faild.Stats {
	s.Lock()
	defer s.Unlock()

	dump := make(map[string]*faild.Stats)
	for k,v := range(s.m) {
		dump[k] = v
	}
	return dump
}

func (s *GlobalStats) Set(agent string, stats *faild.Stats) {
	s.Lock()
	defer s.Unlock()

	s.m[agent] = stats
}

type Agent struct {
	Id  string
	Addr string
	VIP string
	Task chan string
	Stop chan bool
}

var (
	globalStats GlobalStats
	agents []*Agent
)

func startFaildHandler(w http.ResponseWriter, r *http.Request){
	fmt.Println("Starting Faild")
	for _, agent := range(agents) {
		agent.Task <- "start_faild"
	}
	fmt.Fprintf(w, "Started Faild")
}

func stopFaildHandler(w http.ResponseWriter, r *http.Request){
	fmt.Println("Stopping Faild")
	for _, agent := range(agents) {
		agent.Task <- "stop_faild"
	}
	fmt.Fprintf(w, "Stopping Faild")
}

func handleAPIRequests() {
	http.HandleFunc("/start_faild", startFaildHandler)
	http.HandleFunc("/stop_faild", stopFaildHandler)
    log.Fatal(http.ListenAndServe(":10000", nil))
}

func main() {
	globalStats.m = make(map[string]*faild.Stats)
	// register with the prometheus collector
	prometheus.MustRegister(
		newCollector(),
	)

	go handleAPIRequests()
	go startAgents()

	handler := http.NewServeMux()
	handler.Handle("/metrics", promhttp.Handler())

	log.Println("[INFO] starting HTTP server on port :9009")
	log.Fatal(http.ListenAndServe(":9009", handler))
}

func startAgents() {

	agent1 := Agent{
		Id: "host_a",
		Addr: "192.0.2.18:9000",
		VIP: "198.51.100.1/32",
		Task: make(chan string),
		Stop: make(chan bool),
	}
	agent2 := Agent{
		Id: "host_b",
		Addr: "192.0.2.19:9000",
		VIP: "198.51.100.1/32",
		Task: make(chan string),
		Stop: make(chan bool),
	}
	agent3 := Agent{
		Id: "host_c",
		Addr: "192.0.2.20:9000",
		VIP: "198.51.100.1/32",
		Task: make(chan string),
		Stop: make(chan bool),
	}
	agents = append(agents, &agent1, &agent2, &agent3)
	//agents := [...]*Agent{&agent1, &agent2, &agent3}

	log.Printf("[INFO] starting %d agents\n", len(agents))
	wait := sync.WaitGroup{}
	// notify the sync group we need to wait for 10 goroutines
	wait.Add(len(agents))

	for _, agent := range(agents) {
		go startAgent(agent)
		agent.Task <- "init"
		time.Sleep(1 * time.Second)
		agent.Task <- "start"
	}

	wait.Wait()
}


// creates a worker that pulls jobs from the job channel
func startAgent(a *Agent) {
	var err error
	var conn *grpc.ClientConn
	var c faild.FaildServiceClient
	for {
		select {
		// read from the job channel
		case task := <-a.Task:
			switch (task) {
			case "init":
				conn, err = grpc.Dial(a.Addr, grpc.WithInsecure())
				if err != nil {
					log.Fatalf("did not connect: %s", err)
				}
				c = faild.NewFaildServiceClient(conn)
				initReq := faild.InitRequest{
					VIP: a.VIP,
				}
				initResp, err := c.Init(context.Background(), &initReq)
				if err != nil {
					log.Fatalf("Agent: %s, Error when calling Init: %s", a.Id, err)
				}
				log.Printf("Agent: %s, Init Response from server: %v", a.Id, initResp)
			case "start":
				go func(){
					select {
						case <-a.Stop:
							log.Printf("Agent: %s, stop", a.Id)
							break
						default:
							for {
								statsResp, err := c.GetStatistics(context.Background(), &faild.Empty{})
								if err != nil {
									log.Printf("Agent: %s, Error when calling GetStatistics: %s", a.Id, err)
								}
								//log.Printf("Agent: %s, GetStatistics Response from server: %v", a.Id, statsResp)
								time.Sleep(3 * time.Second)
								globalStats.Set(a.Id, statsResp)
							}
					}
				}()
			case "start_faild":
				go func(){
					_, err := c.Start(context.Background(), &faild.Empty{})
					if err != nil {
						log.Printf("Agent: %s, Error Starting Faild: %s", a.Id, err)
					}
				}()
			case "stop_faild":
				go func(){
					_, err := c.Stop(context.Background(), &faild.Empty{})
					if err != nil {
						log.Printf("Agent: %s, Error Stopping Faild: %s", a.Id, err)
					}
				}()
			}
		}
	}
}