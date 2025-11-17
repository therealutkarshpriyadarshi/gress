package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

// Simulator generates fake ride requests and driver locations
type Simulator struct {
	endpoint string
	areas    []string
}

// NewSimulator creates a new data simulator
func NewSimulator(endpoint string) *Simulator {
	return &Simulator{
		endpoint: endpoint,
		areas:    []string{"downtown", "airport", "university", "mall"},
	}
}

// GenerateRideRequest creates a simulated ride request
func (s *Simulator) GenerateRideRequest() RideRequest {
	area := s.areas[rand.Intn(len(s.areas))]

	return RideRequest{
		RequestID: fmt.Sprintf("req-%d", rand.Int63()),
		UserID:    fmt.Sprintf("user-%d", rand.Intn(1000)),
		Location: Location{
			Latitude:  37.7749 + (rand.Float64()-0.5)*0.1,
			Longitude: -122.4194 + (rand.Float64()-0.5)*0.1,
			Area:      area,
		},
		Timestamp: time.Now(),
	}
}

// GenerateDriverLocation creates a simulated driver location
func (s *Simulator) GenerateDriverLocation() DriverLocation {
	area := s.areas[rand.Intn(len(s.areas))]

	return DriverLocation{
		DriverID: fmt.Sprintf("driver-%d", rand.Intn(500)),
		Location: Location{
			Latitude:  37.7749 + (rand.Float64()-0.5)*0.1,
			Longitude: -122.4194 + (rand.Float64()-0.5)*0.1,
			Area:      area,
		},
		Available: rand.Float64() > 0.3, // 70% available
		Timestamp: time.Now(),
	}
}

// SendRideRequest sends a ride request to the HTTP endpoint
func (s *Simulator) SendRideRequest(req RideRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(s.endpoint, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// RunSimulation runs the data generation simulation
func (s *Simulator) RunSimulation(duration time.Duration) {
	fmt.Println("Starting ride-sharing simulation...")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(duration)

	for {
		select {
		case <-timeout:
			fmt.Println("Simulation completed")
			return
		case <-ticker.C:
			// Generate 2-5 ride requests per second
			numRequests := 2 + rand.Intn(4)
			for i := 0; i < numRequests; i++ {
				req := s.GenerateRideRequest()
				if err := s.SendRideRequest(req); err != nil {
					fmt.Printf("Error sending request: %v\n", err)
				} else {
					fmt.Printf("Sent ride request: %s in %s\n", req.RequestID, req.Location.Area)
				}
			}
		}
	}
}
