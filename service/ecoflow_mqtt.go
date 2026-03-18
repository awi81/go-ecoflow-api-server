package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"log/slog"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// EcoFlowMQTTConfig holds the configuration for EcoFlow MQTT
type EcoFlowMQTTConfig struct {
	Email    string // EcoFlow account email (from mobile app)
	Password string // EcoFlow account password (from mobile app)
	ClientID string
	Host     string
	Port     int
}

// EcoFlowMQTTClient handles the MQTT connection to EcoFlow
type EcoFlowMQTTClient struct {
	config    EcoFlowMQTTConfig
	mqtt      mqtt.Client
	data      map[string]interface{}
	dataMutex sync.RWMutex
	onData    func(sn string, data map[string]interface{})
	connected bool
}

// NewEcoFlowMQTTClient creates a new EcoFlow MQTT client
func NewEcoFlowMQTTClient(config EcoFlowMQTTConfig) *EcoFlowMQTTClient {
	return &EcoFlowMQTTClient{
		config: config,
		data:   make(map[string]interface{}),
	}
}

// SetOnData sets the callback for incoming data
func (e *EcoFlowMQTTClient) SetOnData(callback func(sn string, data map[string]interface{})) {
	e.onData = callback
}

// Connect establishes the MQTT connection to EcoFlow using direct paho.mqtt
func (e *EcoFlowMQTTClient) Connect() error {
	slog.Info("Starting MQTT connection", "email", e.config.Email)

	// Default MQTT settings
	host := e.config.Host
	if host == "" {
		host = "mqtt.ecoflow.com"
	}
	port := e.config.Port
	if port == 0 {
		port = 8883
	}

	brokerURL := fmt.Sprintf("tls://%s:%d", host, port)
	slog.Info("Connecting to EcoFlow MQTT broker", "broker", brokerURL)

	// Use configured client ID or generate new one
	clientID := e.config.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("ecoflow-api-%d", time.Now().UnixMilli())
	}

	// Create MQTT client options - use email as username
	slog.Info("MQTT auth attempt", "username", e.config.Email)
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetUsername(e.config.Email).
		SetPassword(e.config.Password).
		SetTLSConfig(nil) // Using tls:// prefix, SSL is automatic

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		slog.Info("MQTT connected, subscribing to topics...")
		// Subscribe to topics after connection
		topics := []string{
			"+/+/+/quota",
			"+/+/+/pd",
			"+/+/+/ems",
			"+/+/+/notify",
			"#",
		}

		for _, topic := range topics {
			token := c.Subscribe(topic, 0, e.messageHandler)
			if token.WaitTimeout(5*time.Second) != true {
				slog.Error("Subscription timeout", "topic", topic)
			} else if token.Error() != nil {
				slog.Error("Subscription error", "topic", topic, "error", token.Error())
			} else {
				slog.Info("Subscribed to topic", "topic", topic)
			}
		}
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		slog.Error("MQTT connection lost", "error", err)
		e.connected = false
	})

	// Create and connect client
	client := mqtt.NewClient(opts)
	token := client.Connect()

	if token.WaitTimeout(30*time.Second) != true {
		slog.Error("MQTT connection timeout")
		return fmt.Errorf("connection timeout")
	}
	if token.Error() != nil {
		slog.Error("MQTT connection error", "error", token.Error())
		return fmt.Errorf("failed to connect: %w", token.Error())
	}

	slog.Info("MQTT connected successfully!")
	e.mqtt = client
	e.connected = true

	return nil
}

// messageHandler handles incoming MQTT messages
func (e *EcoFlowMQTTClient) messageHandler(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	slog.Info("Received MQTT message", "topic", topic, "payload_len", len(payload))

	// Parse JSON payload
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		slog.Error("Failed to parse MQTT message", "topic", topic, "error", err)
		return
	}

	// Extract device SN from topic
	// Topic format: /open/{certificateAccount}/{deviceSN}/{dataType}
	// Example: /open/xxx/BK11ZE1B2H4P0644/quota
	parts := strings.Split(topic, "/")
	// parts[0] = "", parts[1] = "open", parts[2] = certificateAccount, parts[3] = deviceSN, parts[4] = dataType

	var deviceSN string
	if len(parts) >= 4 {
		deviceSN = parts[3]
	}

	// Remove suffix if present (like /quota, /pd, /ems)
	for _, suffix := range []string{"quota", "pd", "ems", "notify"} {
		if strings.HasSuffix(deviceSN, suffix) {
			deviceSN = strings.TrimSuffix(deviceSN, suffix)
			break
		}
	}

	if deviceSN != "" {
		e.dataMutex.Lock()
		e.data[deviceSN] = data
		e.dataMutex.Unlock()

		slog.Info("Stored device data", "device", deviceSN, "topic", topic)

		if e.onData != nil {
			e.onData(deviceSN, data)
		}
	}
}

// GetData returns the cached data for a device
func (e *EcoFlowMQTTClient) GetData(deviceSN string) (map[string]interface{}, bool) {
	e.dataMutex.RLock()
	defer e.dataMutex.RUnlock()

	data, ok := e.data[deviceSN]
	if !ok {
		return nil, false
	}

	return data.(map[string]interface{}), true
}

// GetAllData returns all cached data
func (e *EcoFlowMQTTClient) GetAllData() map[string]interface{} {
	e.dataMutex.RLock()
	defer e.dataMutex.RUnlock()

	result := make(map[string]interface{})
	for k, v := range e.data {
		result[k] = v
	}
	return result
}

// IsConnected returns true if connected to MQTT broker
func (e *EcoFlowMQTTClient) IsConnected() bool {
	return e.connected && e.mqtt != nil && e.mqtt.IsConnected()
}

// Disconnect disconnects from the MQTT broker
func (e *EcoFlowMQTTClient) Disconnect() {
	if e.mqtt != nil && e.mqtt.IsConnected() {
		e.mqtt.Disconnect(250)
		e.connected = false
		slog.Info("Disconnected from EcoFlow MQTT broker")
	}
}
