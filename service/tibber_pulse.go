package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// TibberPulseConfig holds the configuration for Tibber Pulse
type TibberPulseConfig struct {
	DeviceID string // Tibber Pulse Device ID
	APIKey   string // Tibber Pulse API Key
	Host     string // MQTT Broker Host (default: pulse.tibber.com)
	Port     int    // MQTT Broker Port (default: 8883)
}

// PulseData represents the data received from Tibber Pulse
type PulseData struct {
	Power             float64   `json:"power"`              // Current power in watts
	PowerUnit         string    `json:"powerUnit"`          // Usually "W"
	AccumulatedEnergy float64   `json:"accumulatedEnergy"` // Accumulated energy since midnight
	AccumulatedEnergyUnit string `json:"accumulatedEnergyUnit"` // Usually "kWh"
	Voltage           float64   `json:"voltage"`           // Voltage (if available)
	Current           float64   `json:"current"`           // Current (if available)
	Timestamp         time.Time `json:"timestamp"`
}

// TibberPulseClient handles the MQTT connection to Tibber Pulse
type TibberPulseClient struct {
	config     TibberPulseConfig
	client     mqtt.Client
	data       *PulseData
	dataMutex  sync.RWMutex
	connected  bool
	connectedMu sync.RWMutex
	onData     func(*PulseData) // Callback for new data
}

// NewTibberPulseClient creates a new Tibber Pulse client
func NewTibberPulseClient(config TibberPulseConfig) *TibberPulseClient {
	if config.Host == "" {
		config.Host = "pulse.tibber.com"
	}
	if config.Port == 0 {
		config.Port = 8883
	}

	return &TibberPulseClient{
		config: config,
		data:   &PulseData{},
	}
}

// Connect establishes the MQTT connection to Tibber Pulse
func (t *TibberPulseClient) Connect(ctx context.Context) error {
	brokerAddr := fmt.Sprintf("tcp://%s:%d", t.config.Host, t.config.Port)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerAddr)
	opts.SetClientID(fmt.Sprintf("ecoflow-smart-charging-%d", time.Now().Unix()))
	opts.SetUsername(t.config.DeviceID)
	opts.SetPassword(t.config.APIKey)
	opts.SetTLSConfig(nil) // Use default TLS config
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetOnConnectHandler(t.onConnect)
	opts.SetConnectionLostHandler(t.onConnectionLost)

	// Set up message handler using the callback directly
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		if t.onData != nil {
			var data PulseData
			if err := json.Unmarshal(msg.Payload(), &data); err == nil {
				t.parsePayload(msg.Payload(), &data)
				data.Timestamp = time.Now()
				t.dataMutex.Lock()
				t.data = &data
				t.dataMutex.Unlock()
				t.onData(&data)
			}
		}
	})

	t.client = mqtt.NewClient(opts)

	// Connect with timeout
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	token := t.client.Connect()
	if !token.WaitTimeout(30 * time.Second) {
		return fmt.Errorf("connection timeout")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("connection error: %w", err)
	}

	<-connectCtx.Done()
	return nil
}

// onConnect is called when the client connects
func (t *TibberPulseClient) onConnect(client mqtt.Client) {
	t.connectedMu.Lock()
	t.connected = true
	t.connectedMu.Unlock()

	// Subscribe to pulse data topic
	// The topic format is: home/{homeId}/pulse
	topic := fmt.Sprintf("home/+/pulse")
	token := client.Subscribe(topic, 0, nil)
	if token.Wait() && token.Error() != nil {
		fmt.Printf("Error subscribing to topic: %v\n", token.Error())
	}
}

// onConnectionLost is called when the connection is lost
func (t *TibberPulseClient) onConnectionLost(client mqtt.Client, err error) {
	t.connectedMu.Lock()
	t.connected = false
	t.connectedMu.Unlock()
	fmt.Printf("Tibber Pulse connection lost: %v\n", err)
}

// onMessage handles incoming MQTT messages
func (t *TibberPulseClient) onMessage(client mqtt.Client, msg mqtt.Message) {
	var data PulseData
	if err := json.Unmarshal(msg.Payload(), &data); err != nil {
		fmt.Printf("Error parsing pulse data: %v\n", err)
		return
	}

	// Try to parse from different payload formats
	t.parsePayload(msg.Payload(), &data)

	data.Timestamp = time.Now()

	t.dataMutex.Lock()
	t.data = &data
	t.dataMutex.Unlock()

	// Call the callback if set
	if t.onData != nil {
		t.onData(&data)
	}
}

// parsePayload tries to parse the payload in different formats
func (t *TibberPulseClient) parsePayload(payload []byte, data *PulseData) {
	// Try direct parse
	if err := json.Unmarshal(payload, data); err == nil {
		if data.Power != 0 {
			return
		}
	}

	// Try nested "data" field
	var nested struct {
		Data *PulseData `json:"data"`
	}
	if err := json.Unmarshal(payload, &nested); err == nil && nested.Data != nil {
		data.Power = nested.Data.Power
		data.AccumulatedEnergy = nested.Data.AccumulatedEnergy
		data.Voltage = nested.Data.Voltage
		data.Current = nested.Data.Current
		return
	}

	// Try parsing as map and extracting known fields
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return
	}

	// Try common field names
	if power, ok := raw["power"]; ok {
		switch v := power.(type) {
		case float64:
			data.Power = v
		case string:
			if p, err := strconv.ParseFloat(v, 64); err == nil {
				data.Power = p
			}
		}
	}

	if energy, ok := raw["accumulatedEnergy"]; ok {
		switch v := energy.(type) {
		case float64:
			data.AccumulatedEnergy = v
		case string:
			if e, err := strconv.ParseFloat(v, 64); err == nil {
				data.AccumulatedEnergy = e
			}
		}
	}

	if voltage, ok := raw["voltage"]; ok {
		switch v := voltage.(type) {
		case float64:
			data.Voltage = v
		case string:
			if v2, err := strconv.ParseFloat(v, 64); err == nil {
				data.Voltage = v2
			}
		}
	}

	if current, ok := raw["current"]; ok {
		switch v := current.(type) {
		case float64:
			data.Current = v
		case string:
			if c, err := strconv.ParseFloat(v, 64); err == nil {
				data.Current = c
			}
		}
	}
}

// GetData returns the current pulse data
func (t *TibberPulseClient) GetData() *PulseData {
	t.dataMutex.RLock()
	defer t.dataMutex.RUnlock()
	return t.data
}

// GetPower returns the current power consumption in watts
func (t *TibberPulseClient) GetPower() float64 {
	t.dataMutex.RLock()
	defer t.dataMutex.RUnlock()
	return t.data.Power
}

// GetAccumulatedEnergy returns the accumulated energy since midnight in kWh
func (t *TibberPulseClient) GetAccumulatedEnergy() float64 {
	t.dataMutex.RLock()
	defer t.dataMutex.RUnlock()
	return t.data.AccumulatedEnergy
}

// IsConnected returns whether the client is connected
func (t *TibberPulseClient) IsConnected() bool {
	t.connectedMu.RLock()
	defer t.connectedMu.RUnlock()
	return t.connected
}

// SetDataCallback sets a callback for new data
func (t *TibberPulseClient) SetDataCallback(callback func(*PulseData)) {
	t.onData = callback
}

// Disconnect disconnects from the MQTT broker
func (t *TibberPulseClient) Disconnect() {
	if t.client != nil && t.client.IsConnected() {
		t.client.Disconnect(250)
	}
}
