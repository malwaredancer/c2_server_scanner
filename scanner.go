package main

import (
    "bufio"
    "log"
    "fmt"
    "os"
    "strings"
    "net"
    "time"
    "github.com/oschwald/geoip2-golang"
    "bytes"
    "sync"
)

const workers_count = 100
const servers_file_path = "ips"
const coordinates_results_file_path = "coordinates"

func is_server_alive(server_ip string, port string) bool {
    conn_timeout := time.Second * 3
    conn, err := net.DialTimeout("tcp", server_ip + ":" + port, conn_timeout)
    if err != nil {
        return false
    }
    conn.Close()
    return true
}

func get_server_coordinates(server_ip string) string {
    db, err := geoip2.Open("GeoLite2-City.mmdb")
    if err != nil {
            log.Fatal(err)
    }
    defer db.Close()
    ip := net.ParseIP(server_ip)
    record, err := db.City(ip)
    if err != nil {
            log.Fatal(err)
    }
    output_buffer := new(bytes.Buffer)
    fmt.Fprintf(output_buffer, "(%v, %v)", record.Location.Latitude, record.Location.Longitude)
    return output_buffer.String()
}

func split_server_address_port(line string) (string, string) {
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

func worker_server_checker(waitgroup *sync.WaitGroup, server_to_check chan string, coordinates_results chan string) {
    waitgroup.Add(1)
    for {
        line := <-server_to_check
        if strings.Compare(line, "stop") == 0 {
            //fmt.Println("Worker done")
            waitgroup.Done()
            return
        }
        ip, port := split_server_address_port(line)
        is_server_alive_status := is_server_alive(ip, port)
        if is_server_alive_status {
            server_coordinates := get_server_coordinates(ip)
            coordinates_results <- server_coordinates
        }
    }
}

func worker_processing_results(waitgroup *sync.WaitGroup, coordinates_results chan string) {
    waitgroup.Add(1)
    file, err := os.Create(coordinates_results_file_path)
    if err != nil {
        return
    }

    defer file.Close()

    writer := bufio.NewWriter(file)

    for {
        coords := <- coordinates_results
        if strings.Compare(coords, "stop") == 0 {
            //fmt.Println("Worker processing results done")
            writer.Flush()
            waitgroup.Done()
            return
        }
        writer.WriteString(coords + "\n")
	writer.Flush()
    }
}

func iterate_through_server_list(server_list_file_path string) {
    wg := new(sync.WaitGroup)
    servers_to_check := make(chan string, 10)
    coordinates_results := make(chan string, 100)
    file, err := os.Open(server_list_file_path)
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()


    for worker_counter := 0; worker_counter < workers_count; worker_counter++ {
        go worker_server_checker(wg, servers_to_check, coordinates_results)
    }

    go worker_processing_results(wg, coordinates_results)

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        //fmt.Println(scanner.Text())
        line := scanner.Text()
        if len(line) <= 0 {
            continue
        }
        servers_to_check <- line 
    }

    if err := scanner.Err(); err != nil {
        log.Fatal(err)
    }

    for count := 0; count < workers_count; count++ {
           servers_to_check <- "stop"
    }
    //servers_to_check <- "stop"
    coordinates_results <- "stop"
    wg.Wait()
}

func main() {
    iterate_through_server_list(servers_file_path)
}
