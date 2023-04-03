package main

import (
	"bufio"
	"bytes"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
)

const (
	workersCount              = 100
	serversFilePath           = "ips"
	coordinatesResultsFilePath = "coordinates"
)

// isServerAlive checks if a server is alive by attempting to connect to it via TCP
// with a timeout of 3 seconds
func isServerAlive(serverIP string, port string) bool {
	connTimeout := time.Second * 3
	conn, err := net.DialTimeout("tcp", serverIP+":"+port, connTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// getServerCoordinates returns the coordinates of a server's location using
// the MaxMind GeoLite2 City database
func getServerCoordinates(serverIP string) string {
	db, err := geoip2.Open("GeoLite2-City.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ip := net.ParseIP(serverIP)
	record, err := db.City(ip)
	if err != nil {
		log.Fatal(err)
	}
	outputBuffer := new(bytes.Buffer)
	fmt.Fprintf(outputBuffer, "(%v, %v)", record.Location.Latitude, record.Location.Longitude)
	return outputBuffer.String()
}

// splitServerAddressPort splits a string containing a server's address and port
// into separate address and port strings
func splitServerAddressPort(line string) (string, string) {
	splitted := strings.Split(line, ":")
	ip := ""
	port := ""
	if len(splitted) > 1 {
		ip, port = splitted[0], splitted[1]
	} else {
		ip = splitted[0]
		port = "80"
	}
	return ip, port
}

// workerServerChecker checks if a server is alive and gets its coordinates
// and sends the results to a channel
func workerServerChecker(waitGroup *sync.WaitGroup, serversToCheck chan string, coordinatesResults chan string) {
	waitGroup.Add(1)
	for {
		line := <-serversToCheck
		if strings.Compare(line, "stop") == 0 {
			waitGroup.Done()
			return
		}
		ip, port := splitServerAddressPort(line)
		isServerAliveStatus := isServerAlive(ip, port)
		if isServerAliveStatus {
			serverCoordinates := getServerCoordinates(ip)
			coordinatesResults <- serverCoordinates
		}
	}
}

// workerProcessingResults receives coordinates from a channel and writes them
// to a file
func workerProcessingResults(waitGroup *sync.WaitGroup, coordinatesResults chan string) {
	waitGroup.Add(1)
	file, err := os.Create(coordinatesResultsFilePath)
	if err != nil {
		return
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for {
		coords := <-coordinatesResults
		if strings.Compare(coords, "stop") == 0 {
			writer.Flush()
			waitGroup.Done()
			return
		}
		writer.WriteString(coords + "\n")
		writer.Flush()
	}
}

// iterateThroughServerList reads a list of servers from a file and launches
// worker goroutines to check each server's status and get its coordinates
func iterateThroughServerList(serverListFilePath string) {
	wg := new(sync.WaitGroup)
	serversToCheck := make(chan string, 10)
	coordinatesResults := make(chan string, 100)

	// Open the file containing the list of servers
	file, err := os.Open(serverListFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Launch worker goroutines to check each server's status and get its coordinates
	for workerCounter := 0; workerCounter < workersCount; workerCounter++ {
		go workerServerChecker(wg, serversToCheck, coordinatesResults)
	}
	go workerProcessingResults(wg, coordinatesResults)

	// Read the list of servers from the file and send each server to the channel
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) <= 0 {
			continue
		}
		serversToCheck <- line
	}

	// Close the channel after all servers have been sent to it
	close(serversToCheck)

	// Wait for all worker goroutines to finish
	wg.Wait()

	// Close the results channel after all workers have finished processing
	close(coordinatesResults)
}


func main() {
	iterate_through_server_list(servers_file_path)
}
