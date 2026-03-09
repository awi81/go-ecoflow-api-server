package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go-ecoflow-api-server/constants"
	"go-ecoflow-api-server/service"
)

// ChargingHandler handles smart charging requests
type ChargingHandler struct {
	*BaseHandler
	getTibberClient func(r *http.Request) (*service.TibberClient, error)
}

// NewChargingHandler creates a new Charging handler
func NewChargingHandler(baseHandler *BaseHandler, getTibberClient func(r *http.Request) (*service.TibberClient, error)) *ChargingHandler {
	return &ChargingHandler{
		BaseHandler:     baseHandler,
		getTibberClient: getTibberClient,
	}
}

// GetTibberClientOrRespondWithError tries to get the Tibber client from the request
// and responds with an error if it fails
func (h *ChargingHandler) GetTibberClientOrRespondWithError(w http.ResponseWriter, r *http.Request) (*service.TibberClient, bool) {
	client, err := h.getTibberClient(r)
	if err != nil {
		h.RespondWithError(w, http.StatusBadRequest, constants.ErrUnauthorized, err.Error(), nil)
		return nil, false
	}
	return client, true
}

// RegisterRoutes registers the Charging routes
func (h *ChargingHandler) RegisterRoutes(router chi.Router) {
	router.Route("/api/charging", func(r chi.Router) {
		r.Post("/strategy", h.GetChargingStrategy())
		r.Post("/optimal-times", h.GetOptimalChargingTimes())
		r.Post("/savings", h.GetSavings())
	})
}

// ChargingConfigRequest represents the request body for charging strategy
type ChargingConfigRequest struct {
	MinSOC              int     `json:"minSOC"`
	MaxSOC              int     `json:"maxSOC"`
	MaxPriceToCharge    float64 `json:"maxPriceToCharge"`
	MinPriceToDischarge float64 `json:"minPriceToDischarge"`
	ChargingPowerWatts  int     `json:"chargingPowerWatts"`
	DischargingPowerWatts int   `json:"dischargingPowerWatts"`
	PVSurplusWatts      int     `json:"pvSurplusWatts"`
}

// GetChargingStrategy handles retrieving a charging strategy recommendation
// @Summary Get charging strategy
// @Description Returns a recommended charging action based on current electricity prices and configuration
// @Tags Charging
// @Accept json
// @Produce json
// @Param request body ChargingConfigRequest true "Charging configuration"
// @Success 200 {object} SuccessResponse "Strategy retrieved successfully"
// @Failure 400 {object} ErrorResponse "Error retrieving strategy"
// @Router /api/charging/strategy [post]
func (h *ChargingHandler) GetChargingStrategy() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		var request ChargingConfigRequest
		if err := decodeJSON(r, &request); err != nil {
			h.RespondWithError(w, http.StatusBadRequest, constants.ErrInvalidJsonBody, err.Error(), nil)
			return
		}

		// Set defaults if not provided
		if request.MinSOC == 0 {
			request.MinSOC = 20
		}
		if request.MaxSOC == 0 {
			request.MaxSOC = 80
		}
		if request.MaxPriceToCharge == 0 {
			request.MaxPriceToCharge = 0.25 // 25 cent/kWh
		}
		if request.MinPriceToDischarge == 0 {
			request.MinPriceToDischarge = 0.35 // 35 cent/kWh
		}
		if request.ChargingPowerWatts == 0 {
			request.ChargingPowerWatts = 800 // 800W default
		}
		if request.DischargingPowerWatts == 0 {
			request.DischargingPowerWatts = 800
		}

		config := service.ChargingConfig{
			MinSOC:              request.MinSOC,
			MaxSOC:              request.MaxSOC,
			MaxPriceToCharge:    request.MaxPriceToCharge,
			MinPriceToDischarge: request.MinPriceToDischarge,
			ChargingPowerWatts:  request.ChargingPowerWatts,
			DischargingPowerWatts: request.DischargingPowerWatts,
			PVSurplusWatts:      request.PVSurplusWatts,
		}

		strategy, err := client.ChargingStrategy(r.Context(), config)
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}

		h.RespondWithSuccess(w, strategy)
	}
}

// OptimalTimesRequest represents the request for optimal charging times
type OptimalTimesRequest struct {
	HoursNeeded int `json:"hoursNeeded"`
}

// GetOptimalChargingTimes handles retrieving optimal charging time windows
// @Summary Get optimal charging times
// @Description Returns the cheapest hours for charging within the next 24-48 hours
// @Tags Charging
// @Accept json
// @Produce json
// @Param request body OptimalTimesRequest true "Request parameters"
// @Success 200 {object} SuccessResponse "Optimal times retrieved successfully"
// @Failure 400 {object} ErrorResponse "Error retrieving times"
// @Router /api/charging/optimal-times [post]
func (h *ChargingHandler) GetOptimalChargingTimes() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		var request OptimalTimesRequest
		if err := decodeJSON(r, &request); err != nil {
			// Default to 4 hours if not provided
			request.HoursNeeded = 4
		}
		if request.HoursNeeded == 0 {
			request.HoursNeeded = 4
		}
		if request.HoursNeeded > 24 {
			request.HoursNeeded = 24
		}

		times, err := client.CalculateOptimalChargingTime(r.Context(), request.HoursNeeded)
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}

		h.RespondWithSuccess(w, map[string]interface{}{
			"optimalTimes": times,
			"hoursNeeded":  request.HoursNeeded,
		})
	}
}

// SavingsConfigRequest represents the request for savings calculation
type SavingsConfigRequest struct {
	MinSOC              int     `json:"minSOC"`
	MaxSOC              int     `json:"maxSOC"`
	MaxPriceToCharge    float64 `json:"maxPriceToCharge"`
	MinPriceToDischarge float64 `json:"minPriceToDischarge"`
	ChargingPowerWatts  int     `json:"chargingPowerWatts"`
	DischargingPowerWatts int   `json:"dischargingPowerWatts"`
	PVSurplusWatts      int     `json:"pvSurplusWatts"`
}

// GetSavings handles retrieving potential savings from smart charging
// @Summary Get potential savings
// @Description Returns potential savings from smart charging compared to flat-rate charging
// @Tags Charging
// @Accept json
// @Produce json
// @Param request body SavingsConfigRequest true "Configuration for savings calculation"
// @Success 200 {object} SuccessResponse "Savings retrieved successfully"
// @Failure 400 {object} ErrorResponse "Error calculating savings"
// @Router /api/charging/savings [post]
func (h *ChargingHandler) GetSavings() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client, ok := h.GetTibberClientOrRespondWithError(w, r)
		if !ok {
			return
		}

		var request SavingsConfigRequest
		if err := decodeJSON(r, &request); err != nil {
			// Use defaults
			request.MinSOC = 20
			request.MaxSOC = 80
			request.MaxPriceToCharge = 0.25
			request.MinPriceToDischarge = 0.35
			request.ChargingPowerWatts = 800
			request.DischargingPowerWatts = 800
		}

		config := service.ChargingConfig{
			MinSOC:              request.MinSOC,
			MaxSOC:              request.MaxSOC,
			MaxPriceToCharge:    request.MaxPriceToCharge,
			MinPriceToDischarge: request.MinPriceToDischarge,
			ChargingPowerWatts:  request.ChargingPowerWatts,
			DischargingPowerWatts: request.DischargingPowerWatts,
			PVSurplusWatts:      request.PVSurplusWatts,
		}

		savings, err := client.CalculateSavings(r.Context(), config)
		if err != nil {
			h.RespondWithError(w, http.StatusInternalServerError, constants.ErrInternalServer, err.Error(), nil)
			return
		}

		h.RespondWithSuccess(w, savings)
	}
}
