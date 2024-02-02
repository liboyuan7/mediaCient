package main

import (
	"flag"
	"fmt"
	"mediaClient/client"
	"mediaClient/port"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	serverAddr  = flag.String("server_addr", "127.0.0.1:5678", "The server address in the format of host:port")
	concurrency = flag.Int("concurrency", 4, "The number of concurrent requests audio and video")
	numRequests = flag.Int("num_requests", 12, "The total number of requests audio and video to be made")
	audioGraph  = flag.String("audio_graph", "[rtp_src] -> [rtp_sink]", "The audio session graph used for media sever node")
	videoGraph  = flag.String("video_graph", "[rtp_src] -> [rtp_sink]", "The video session graph used for media sever node")
)

func main() {
	flag.Parse()

	ip, gPort, err := net.SplitHostPort(*serverAddr)
	client.GrpcIp = ip
	client.GrpcPort, _ = strconv.Atoi(gPort)

	fmt.Printf("start grpc %v port %v\n", ip, gPort)

	if err != nil {
		fmt.Println("Error parsing server address:", err)
		return
	}

	client.InitAudioDataCache("./audio.pcm")
	client.InitVideoDataCache("./raw.h264")

	// Set up a connection to the server.
	/*conn, err := grpc.Dial(*serverAddr, grpc.WithInsecure())
	if err != nil {
		fmt.Printf("Failed to connect: %v", err)
	}
	defer conn.Close()*/

	portRequest := port.NewPortPool()
	portRequest.Init(20000, 30000)

	// Perform the benchmark.
	start := time.Now()
	runBenchmark(*concurrency, *numRequests, portRequest)
	elapsed := time.Since(start)

	fmt.Printf("Total time taken: %s\n", elapsed)
	fmt.Printf("Requests per second: %.2f\n", float64(*numRequests)/elapsed.Seconds())
}

func runBenchmark(concurrency, numRequests int, p *port.MyPortPool) {
	var wg sync.WaitGroup
	requestsPerWorker := numRequests / concurrency
	var index int32
	var flag int32
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				currentIndex := atomic.AddInt32(&index, 1)
				id := "new_session_" + strconv.Itoa(int(currentIndex))

				// 原子操作，确保并发安全的读取和修改 flag
				isAudio := atomic.LoadInt32(&flag) == 1
				atomic.StoreInt32(&flag, 1-atomic.LoadInt32(&flag))
				if isAudio {
					client.StartSessionCall(p, id, true, *audioGraph)
				} else {
					client.StartSessionCall(p, id, false, *videoGraph)
				}
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Wait for all workers to finish.
	wg.Wait()
}
