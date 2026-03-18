package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go-ecoflow-api-server/constants"
	"go-ecoflow-api-server/service"
)

// EcoFlowMQTTHandler handles EcoFlow MQTT requests
type EcoFlowMQTTHandler struct {
	*BaseHandler
	getClient func() *service.EcoFlowMQTTClient
}

// NewEcoFlowMQTTHandler creates a new EcoFlow MQTT handler
func NewEcoFlowMQTTHandler(baseHandler *BaseHandler, getClient func() *service.EcoFlowMQTTClient) *EcoFlowMQTTHandler {
	return &EcoFlowMQTTHandler{
		BaseHandler: baseHandler,
		getClient:   getClient,
	}
}

// RegisterRoutes registers the EcoFlow MQTT routes
func (h *EcoFlowMQTTHandler) RegisterRoutes(router chi.Router) {
	router.Route("/api/ecoflow-mqtt", func(r chi.Router) {
		r.Post("/connect", h.Connect())
		r.Post("/disconnect", h.Disconnect())
		r.Get("/status", h.Status())
		r.Get("/devices/{serial_number}", h.GetDeviceData())
	})
}

// Connect handles connecting to EcoFlow MQTT
// @Summary Connect to EcoFlow MQTT
// @Description Connects to EcoFlow MQTT broker using the provided credentials
// @Tags EcoFlowMQTT
// @Accept json
// @Produce json
// @Param request body EcoFlowMQTTConnectRequest true "MQTT credentials"
// @Success 200 {object} SuccessResponse "Connected successfully"
// @Failure 400 {object} ErrorResponse "Error connecting"
// @Router /api/ecoflow-mqtt/connect [post]
func (h *EcoFlowMQTTHandler) Connect() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		if err := decodeJSON(r, &request); err != nil {
			h.RespondWithError(w, http.StatusBadRequest, constants.ErrInvalidJsonBody, err.Error(), nil)
			return
		}

		if request.Email == "" || request.Password == "" {
			h.RespondWithError(w, http.StatusBadRequest, constants.ErrInvalidJsonBody, "email and password are required", nil)
			return
		}

		client := service.NewEcoFlowMQTTClient(service.EcoFlowMQTTConfig{
			Email:    request.Email,
			Password: request.Password,
		})

		if err := client.Connect(); err != nil {
			h.RespondWithError(w, http.StatusBadRequest, "MQTT_ERROR", "", err.Error())
			return
		}

		// Store client globally (in production, use a proper manager)
		ecoflowMQTTClient = client

		h.RespondWithSuccess(w, map[string]interface{}{
			"connected": true,
			"message":  "Connected to EcoFlow MQTT",
		})
	}
}

// Disconnect handles disconnecting from EcoFlow MQTT
// @Summary Disconnect from EcoFlow MQTT
// @Description Disconnects from EcoFlow MQTT broker
// @Tags EcoFlowMQTT
// @Produce json
// @Success 200 {object} SuccessResponse "Disconnected successfully"
// @Router /api/ecoflow-mqtt/disconnect [post]
func (h *EcoFlowMQTTHandler) Disconnect() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if ecoflowMQTTClient != nil {
			ecoflowMQTTClient.Disconnect()
			ecoflowMQTTClient = nil
		}

		h.RespondWithSuccess(w, map[string]interface{}{
			"connected": false,
			"message":  "Disconnected from EcoFlow MQTT",
		})
	}
}

// Status handles getting MQTT connection status
// @Summary Get MQTT status
// @Description Returns the current MQTT connection status
// @Tags EcoFlowMQTT
// @Produce json
// @Success 200 {object} SuccessResponse "Status retrieved"
// @Router /api/ecoflow-mqtt/status [get]
func (h *EcoFlowMQTTHandler) Status() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		connected := false
		if ecoflowMQTTClient != nil {
			connected = ecoflowMQTTClient.IsConnected()
		}

		h.RespondWithSuccess(w, map[string]interface{}{
			"connected": connected,
		})
	}
}

// GetDeviceData handles getting device data from MQTT
// @Summary Get device data from MQTT
// @Description Returns cached device data from MQTT connection
// @Tags EcoFlowMQTT
// @Produce json
// @Param serial_number path string true "Device Serial Number"
// @Success 200 {object} SuccessResponse "Device data retrieved"
// @Failure 404 {object} ErrorResponse "No data available"
// @Router /api/ecoflow-mqtt/devices/{serial_number} [get]
func (h *EcoFlowMQTTHandler) GetDeviceData() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sn := r.PathValue("serial_number")

		if ecoflowMQTTClient == nil || !ecoflowMQTTClient.IsConnected() {
			h.RespondWithError(w, http.StatusServiceUnavailable, constants.ErrInternalServer, "MQTT not connected", nil)
			return
		}

		data, ok := ecoflowMQTTClient.GetData(sn)
		if !ok {
			h.RespondWithError(w, http.StatusNotFound, constants.ErrInternalServer, "No data for device", map[string]string{
				"serial_number": sn,
			})
			return
		}

		h.RespondWithSuccess(w, data)
	}
}

// Global MQTT client instance
var ecoflowMQTTClient *service.EcoFlowMQTTClient
