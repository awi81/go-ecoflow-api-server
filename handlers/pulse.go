package handlers

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"go-ecoflow-api-server/constants"
	"go-ecoflow-api-server/service"
)

// PulseHandler handles Tibber Pulse data requests
type PulseHandler struct {
	*BaseHandler
	pulseClient   *service.TibberPulseClient
	pulseClientMu sync.RWMutex
}

// NewPulseHandler creates a new Pulse handler
func NewPulseHandler(baseHandler *BaseHandler) *PulseHandler {
	return &PulseHandler{
		BaseHandler: baseHandler,
	}
}

// SetPulseClient sets the Tibber Pulse client
func (h *PulseHandler) SetPulseClient(client *service.TibberPulseClient) {
	h.pulseClientMu.Lock()
	defer h.pulseClientMu.Unlock()
	h.pulseClient = client
}

// RegisterRoutes registers the Pulse routes
func (h *PulseHandler) RegisterRoutes(router chi.Router) {
	router.Route("/api/pulse", func(r chi.Router) {
		r.Get("/data", h.GetPulseData())
		r.Get("/status", h.GetPulseStatus())
		r.Post("/connect", h.ConnectPulse())
		r.Post("/disconnect", h.DisconnectPulse())
	})
}

// PulseDataResponse represents the response for pulse data
type PulseDataResponse struct {
	Power             float64   `json:"power"`
	AccumulatedEnergy float64   `json:"accumulatedEnergy"`
	Voltage           float64   `json:"voltage"`
	Current           float64   `json:"current"`
	Timestamp         string    `json:"timestamp"`
}

// GetPulseData handles retrieving current pulse data
// @Summary Get Tibber Pulse data
// @Description Returns current power consumption data from Tibber Pulse
// @Tags Tibber Pulse
// @Produce json
// @Success 200 {object} SuccessResponse "Pulse data retrieved successfully"
// @Failure 500 {object} ErrorResponse "Error retrieving pulse data"
// @Router /api/pulse/data [get]
func (h *PulseHandler) GetPulseData() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		h.pulseClientMu.RLock()
		client := h.pulseClient
		h.pulseClientMu.RUnlock()

		if client == nil {
			h.RespondWithError(w, http.StatusServiceUnavailable, constants.ErrInternalServer, "Tibber Pulse nicht konfiguriert", nil)
			return
		}

		data := client.GetData()
		if data == nil {
			h.RespondWithError(w, http.StatusNoContent, constants.ErrInternalServer, "Noch keine Daten verfügbar", nil)
			return
		}

		response := PulseDataResponse{
			Power:             data.Power,
			AccumulatedEnergy: data.AccumulatedEnergy,
			Voltage:           data.Voltage,
			Current:           data.Current,
			Timestamp:         data.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		}

		h.RespondWithSuccess(w, response)
	}
}

// GetPulseStatus handles retrieving pulse connection status
// @Summary Get Tibber Pulse status
// @Description Returns the connection status of Tibber Pulse
// @Tags Tibber Pulse
// @Produce json
// @Success 200 {object} SuccessResponse "Status retrieved successfully"
// @Router /api/pulse/status [get]
func (h *PulseHandler) GetPulseStatus() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		h.pulseClientMu.RLock()
		client := h.pulseClient
		h.pulseClientMu.RUnlock()

		status := map[string]interface{}{
			"connected": false,
			"configured": client != nil,
		}

		if client != nil {
			status["connected"] = client.IsConnected()
			data := client.GetData()
			if data != nil {
				status["lastUpdate"] = data.Timestamp.Format("2006-01-02T15:04:05Z07:00")
			}
		}

		h.RespondWithSuccess(w, status)
	}
}

// ConnectPulseRequest represents the request to connect to Tibber Pulse
type ConnectPulseRequest struct {
	DeviceID string `json:"deviceId"`
	APIKey   string `json:"apiKey"`
}

// ConnectPulse handles connecting to Tibber Pulse
// @Summary Connect to Tibber Pulse
// @Description Connects to Tibber Pulse using MQTT
// @Tags Tibber Pulse
// @Accept json
// @Produce json
// @Param request body ConnectPulseRequest true "Tibber Pulse credentials"
// @Success 200 {object} SuccessResponse "Connected successfully"
// @Failure 400 {object} ErrorResponse "Error connecting"
// @Router /api/pulse/connect [post]
func (h *PulseHandler) ConnectPulse() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var request ConnectPulseRequest
		if err := decodeJSON(r, &request); err != nil {
			h.RespondWithError(w, http.StatusBadRequest, constants.ErrInvalidJsonBody, err.Error(), nil)
			return
		}

		if request.DeviceID == "" || request.APIKey == "" {
			h.RespondWithError(w, http.StatusBadRequest, constants.ErrInvalidJsonBody, "deviceId und apiKey erforderlich", nil)
			return
		}

		// Create new client
		config := service.TibberPulseConfig{
			DeviceID: request.DeviceID,
			APIKey:   request.APIKey,
		}
		client := service.NewTibberPulseClient(config)

		// Connect
		if err := client.Connect(r.Context()); err != nil {
			h.RespondWithError(w, http.StatusBadRequest, constants.ErrInternalServer, "Verbindung fehlgeschlagen: "+err.Error(), nil)
			return
		}

		// Set client
		h.pulseClientMu.Lock()
		h.pulseClient = client
		h.pulseClientMu.Unlock()

		h.RespondWithSuccess(w, map[string]string{
			"status": "connected",
		})
	}
}

// DisconnectPulse handles disconnecting from Tibber Pulse
// @Summary Disconnect from Tibber Pulse
// @Description Disconnects from Tibber Pulse
// @Tags Tibber Pulse
// @Produce json
// @Success 200 {object} SuccessResponse "Disconnected successfully"
// @Router /api/pulse/disconnect [post]
func (h *PulseHandler) DisconnectPulse() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		h.pulseClientMu.Lock()
		defer h.pulseClientMu.Unlock()

		if h.pulseClient != nil {
			h.pulseClient.Disconnect()
			h.pulseClient = nil
		}

		h.RespondWithSuccess(w, map[string]string{
			"status": "disconnected",
		})
	}
}

// decodeJSON is a helper to decode JSON
func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
